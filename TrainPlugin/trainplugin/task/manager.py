"""单任务串行的任务管理器。

- 维护一个 FIFO 队列与一个 worker 线程，同一时刻只训练一个任务。
- 状态机：排队中 -> 准备中 -> 运行中 -> 已完成 / 已失败。
- 每次状态迁移通过 reporter 上报 RemoteLog（TrainRemote）。
"""

import contextlib
import logging
import os
import queue
import re
import threading
import time
from typing import Any, Callable, Optional

from ..data.bufferfile import append_samples, read_samples
from . import status
from .task import TaskCancelled, TrainingTask

logger = logging.getLogger(__name__)

_TIMEOUT_UNITS = {"d": 86400, "h": 3600, "m": 60, "s": 1}


def parse_timeout(s: str) -> Optional[float]:
    """解析训练限时字符串（d/h/m/s 自由组合，如 1h30m、90m）为秒数；空或非法返回 None。"""
    if not s:
        return None
    total = 0.0
    for num, unit in re.findall(r"(\d+)([dhms])", s):
        total += int(num) * _TIMEOUT_UNITS[unit]
    return total or None


class RWLock:
    """基于 Condition 的读者-写者锁。"""

    def __init__(self) -> None:
        self._cond = threading.Condition()
        self._readers = 0
        self._writer = False
        self._waiting_writers = 0

    @contextlib.contextmanager
    def read_lock(self):
        with self._cond:
            while self._writer or self._waiting_writers > 0:
                self._cond.wait()
            self._readers += 1
        try:
            yield
        finally:
            with self._cond:
                self._readers -= 1
                if self._readers == 0:
                    self._cond.notify_all()

    @contextlib.contextmanager
    def write_lock(self):
        with self._cond:
            self._waiting_writers += 1
            try:
                while self._writer or self._readers > 0:
                    self._cond.wait()
                self._writer = True
            finally:
                self._waiting_writers -= 1
        try:
            yield
        finally:
            with self._cond:
                self._writer = False
                self._cond.notify_all()


class TaskManager:
    def __init__(
        self,
        prep_fn: Callable[[TrainingTask], Any],
        train_fn: Callable[[TrainingTask, Any], None],
        reporter: Callable[[TrainingTask, str, str], None],
        data_wait_timeout: Optional[float] = None,
    ) -> None:
        self._prep_fn = prep_fn
        self._train_fn = train_fn
        self._reporter = reporter
        self._data_wait_timeout = data_wait_timeout

        self._queue: "queue.Queue[TrainingTask]" = queue.Queue()
        self._ready: "queue.Queue" = queue.Queue()
        self._current: Optional[TrainingTask] = None
        self._running: Optional[TrainingTask] = None
        self._tasks: dict = {}
        self._file_locks: dict = {}
        self._data_dir = os.environ.get("DATA_PATH", "./train_data_buffer")
        os.makedirs(self._data_dir, exist_ok=True)
        self._lock = threading.Lock()

        self._stop = threading.Event()
        self._prep_worker = threading.Thread(target=self._prep_loop, name="task-prep", daemon=True)
        self._prep_worker.start()
        self._worker = threading.Thread(target=self._run, name="task-worker", daemon=True)
        self._worker.start()

    def submit(self, task: TrainingTask) -> None:
        with self._lock:
            self._current = task
            self._tasks[task.id] = task
        self._queue.put(task)
        self._reporter(task, status.QUEUED, "task queued")

    def current(self) -> Optional[TrainingTask]:
        with self._lock:
            return self._current

    def get_task(self, task_id: str) -> Optional[TrainingTask]:
        with self._lock:
            return self._tasks.get(task_id)

    def receive_data(self, task_id: str, samples: list) -> bool:
        """将收到的样本路由到当前任务 buffer 或落盘文件，返回任务是否存在。"""
        with self._lock:
            if task_id not in self._tasks:
                return False
            running = self._running
            if running is not None and running.id == task_id:
                target = running
                to_memory = True
            else:
                target = None
                to_memory = False
                lock = self._get_file_lock(task_id)

        if to_memory:
            with target.lock:
                target.buffer.extend(samples)
        else:
            with lock.write_lock():
                append_samples(self._file_path(task_id), samples)
        return True

    def _file_path(self, task_id: str) -> str:
        return os.path.join(self._data_dir, f"{task_id}.txt")

    def _get_file_lock(self, task_id: str) -> "RWLock":
        lock = self._file_locks.get(task_id)
        if lock is None:
            lock = RWLock()
            self._file_locks[task_id] = lock
        return lock

    def _load_file(self, task_id: str) -> list:
        path = self._file_path(task_id)
        if not os.path.exists(path):
            return []
        lock = self._get_file_lock(task_id)
        with lock.read_lock():
            if not os.path.exists(path):
                return []
            samples = read_samples(path)
            os.remove(path)
            return samples

    def _remove_file(self, task_id: str) -> None:
        try:
            os.remove(self._file_path(task_id))
        except FileNotFoundError:
            pass

    def handle_data_error(self, task_id: str, error: str) -> bool:
        with self._lock:
            task = self._tasks.get(task_id)
        if task is None:
            return False
        self._reporter(task, status.FAILED, error)
        queued = (task.status == status.QUEUED)
        task.abort(error)
        if queued:
            self._remove_file(task_id)
        else:
            task.clear_data()
            self._remove_file(task_id)
        return True

    def cancel(self) -> Optional[TrainingTask]:
        task = self.current()
        if task is None:
            return None
        task.cancel_event.set()
        return task

    def close(self) -> None:
        self._stop.set()
        task = self.current()
        if task is not None:
            task.cancel_event.set()
            task.data_ready_event.set()

    def _prep_loop(self) -> None:
        while True:
            try:
                task = self._queue.get(timeout=1.0)
            except queue.Empty:
                if self._stop.is_set():
                    return
                continue

            try:
                prepared = self._prepare(task)
            except Exception as exc:  # noqa: BLE001 - 兜底，避免准备线程退出
                logger.exception("unexpected error while preparing task %s", task.id)
                task.set_error(str(exc))
                self._reporter(task, status.FAILED, f"unexpected error: {exc}")
                self._cleanup(task)
                continue

            if prepared is None:
                self._cleanup(task)
                continue

            self._ready.put((task, prepared))

    def _run(self) -> None:
        while True:
            try:
                task, prepared = self._ready.get(timeout=1.0)
            except queue.Empty:
                if self._stop.is_set():
                    return
                continue

            with self._lock:
                self._running = task
            try:
                self._train(task, prepared)
            except Exception as exc:  # noqa: BLE001 - 兜底，避免 worker 退出
                logger.exception("unexpected error while training task %s", task.id)
                task.set_error(str(exc))
                self._reporter(task, status.FAILED, f"unexpected error: {exc}")
            finally:
                self._cleanup(task)

    def _cleanup(self, task: TrainingTask) -> None:
        self._cancel_timeout(task)
        task.clear_data()
        with self._lock:
            if self._running is task:
                self._running = None
            self._tasks.pop(task.id, None)
            self._file_locks.pop(task.id, None)
        self._remove_file(task.id)

    def _start_timeout(self, task: TrainingTask) -> None:
        seconds = parse_timeout(task.train_config.timeout)
        if seconds:
            task.timeout_timer = threading.Timer(seconds, self._on_timeout, args=(task,))
            task.timeout_timer.start()

    def _cancel_timeout(self, task: TrainingTask) -> None:
        if task.timeout_timer is not None:
            task.timeout_timer.cancel()
            task.timeout_timer = None

    def _on_timeout(self, task: TrainingTask) -> None:
        task.timeout_event.set()
        logger.warning("task %s timed out", task.id)
        self._reporter(task, status.CANCELLED, f"{task.id}:timeout")
        task.set_cancelled("timeout")

    def _wait_data_ready(self, task: TrainingTask) -> bool:
        """等待数据接收完毕，返回 True 表示数据就绪，False 表示被取消或出错。"""
        self._start_timeout(task)
        deadline = None if self._data_wait_timeout is None else time.time() + self._data_wait_timeout
        while not task.data_ready_event.is_set():
            if task.cancel_event.is_set():
                return False
            if task.abort_event.is_set():
                return False
            if task.timeout_event.is_set():
                return False
            if deadline is not None and time.time() >= deadline:
                logger.warning("task %s: data wait timeout, start with %d samples", task.id, len(task.buffer))
                break
            task.data_ready_event.wait(timeout=0.1)
        self._cancel_timeout(task)
        return True

    def _prepare(self, task: TrainingTask):
        if task.cancel_event.is_set():
            self._reporter(task, status.CANCELLED, "cancelled before start")
            task.set_cancelled("cancelled before start")
            return None

        self._reporter(task, status.PREPARING, "collecting data")
        task.set_status(status.PREPARING)
        if not self._wait_data_ready(task):
            if task.timeout_event.is_set():
                return None
            if task.abort_event.is_set():
                return None
            self._reporter(task, status.CANCELLED, "cancelled while collecting data")
            task.set_cancelled("cancelled while collecting data")
            return None

        if task.cancel_event.is_set():
            self._reporter(task, status.CANCELLED, "cancelled before training")
            task.set_cancelled("cancelled before training")
            return None

        pending = self._load_file(task.id)
        if pending:
            with task.lock:
                task.buffer.extend(pending)

        return self._prep_fn(task)

    def _train(self, task: TrainingTask, prepared) -> None:
        self._reporter(task, status.RUNNING, "training started")
        task.set_status(status.RUNNING)
        try:
            self._train_fn(task, prepared)
        except TaskCancelled:
            if task.timeout_event.is_set():
                return
            self._reporter(task, status.CANCELLED, "cancelled during training")
            task.set_cancelled("cancelled during training")
            return
        except Exception as exc:
            self._reporter(task, status.FAILED, f"training failed: {exc}")
            task.set_error(str(exc))
            return

        if task.cancel_event.is_set():
            self._reporter(task, status.CANCELLED, "cancelled during training")
            task.set_cancelled("cancelled during training")
            return

        task.mark_done()
        self._reporter(task, status.FINISHED, "training finished")
