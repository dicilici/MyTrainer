"""TrainPlugin 启动级配置。

任务级参数（数据集、训练配置、模型路径、RemoteLog URL 等）均来自 gRPC 请求，
此处仅包含 TrainPlugin 进程本身的启动参数。
"""

from dataclasses import dataclass
from typing import Optional


@dataclass
class TrainPluginConfig:
    listen: str = "0.0.0.0:50052"
    withtrain_listen: str = "0.0.0.0:50054"
    max_workers: int = 32
    data_wait_timeout: Optional[float] = None
