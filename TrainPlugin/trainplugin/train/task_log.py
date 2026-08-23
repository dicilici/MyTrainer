"""任务级本地日志写入器。

每条日志统一格式为 ``time:... message:... status:...``，每次写入后立即关闭文件句柄。
日志文件统一存放在 ``TRAIN_LOG`` 目录下，文件名来自任务的 LogSave（仅允许文件名），
为空或非法时回退为 ``<任务id>.txt``。
"""

import logging
import os
from datetime import datetime
from typing import TYPE_CHECKING

if TYPE_CHECKING:  # pragma: no cover - 仅用于类型提示
    from ..task.task import TrainingTask

logger = logging.getLogger(__name__)


class TaskLogger:
    def __init__(self, base_dir: str) -> None:
        self._base_dir = base_dir

    @staticmethod
    def _valid_filename(name: str) -> bool:
        if not name:
            return False
        if name in (".", ".."):
            return False
        if "/" in name or "\\" in name:
            return False
        if "\x00" in name:
            return False
        return True

    def _filename(self, task: "TrainingTask") -> str:
        name = (task.train_config.log_save or "").strip()
        if self._valid_filename(name):
            return name
        return f"{task.id}.txt"

    def path_for(self, task: "TrainingTask") -> str:
        os.makedirs(self._base_dir, exist_ok=True)
        return os.path.join(self._base_dir, self._filename(task))

    def write(self, task: "TrainingTask", status: str, message: str) -> None:
        path = self.path_for(task)
        now = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        line = f"time:{now} message:{message} status:{status}\n"
        try:
            with open(path, "a", encoding="utf-8") as f:
                f.write(line)
        except OSError:
            logger.warning("failed to write training log to %s", path)
