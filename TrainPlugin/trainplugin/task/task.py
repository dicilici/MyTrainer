"""训练任务的数据结构。

一个 TrainingTask 对应一次训练后端发起的训练请求（SendToTrain）。
任务持有配置、数据缓冲、进度与状态；缓冲与进度读写需加锁保护。
"""

import threading
from dataclasses import dataclass, field
from typing import List, Optional, Tuple

from . import status as task_status


class TaskCancelled(Exception):
    """训练任务被取消时抛出。"""


@dataclass
class DatasetConfig:
    input: str = ""
    validation: int = 0
    categories_number: int = 0


@dataclass
class TrainConfig:
    epochs: int = 0
    learning_rate: float = 0.0
    loss_function: str = ""
    early_stop: bool = False
    early_stop_patient: int = 0
    model_save: str = ""
    log_save: str = ""
    timeout: str = ""


@dataclass
class TrainingTask:
    id: str
    remote_log_url: str
    dataset: DatasetConfig
    train_config: TrainConfig

    status: str = task_status.QUEUED
    error: Optional[str] = None

    buffer: List[Tuple] = field(default_factory=list)
    label_vocab: dict = field(default_factory=dict)

    epoch: int = 0
    loss_list: List[float] = field(default_factory=list)
    done: bool = False

    cancel_event: threading.Event = field(default_factory=threading.Event)
    data_ready_event: threading.Event = field(default_factory=threading.Event)
    abort_event: threading.Event = field(default_factory=threading.Event)
    timeout_event: threading.Event = field(default_factory=threading.Event)
    timeout_timer: Optional[threading.Timer] = None
    lock: threading.RLock = field(default_factory=threading.RLock)

    def set_status(self, new_status: str) -> None:
        with self.lock:
            self.status = new_status

    def set_error(self, message: str) -> None:
        with self.lock:
            self.error = message
            self.status = task_status.FAILED

    def abort(self, message: str) -> None:
        with self.lock:
            self.error = message
            self.status = task_status.FAILED
            self.buffer = []
            self.loss_list = []
        self.abort_event.set()

    def append_sample(self, sample: Tuple) -> None:
        with self.lock:
            self.buffer.append(sample)

    def append_loss(self, value: float) -> None:
        with self.lock:
            self.loss_list.append(value)
            self.epoch += 1

    def mark_done(self) -> None:
        with self.lock:
            self.done = True
            self.status = task_status.FINISHED

    def snapshot(self) -> Tuple[int, List[float], bool, str, str]:
        with self.lock:
            return self.epoch, list(self.loss_list), self.done, self.status, self.error or ""
