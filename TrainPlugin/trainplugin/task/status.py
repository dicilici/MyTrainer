"""训练任务状态常量，与 receive.proto 中 TrainMsg.Status 的取值对齐。"""

QUEUED = "排队中"
PREPARING = "准备中"
RUNNING = "运行中"
FINISHED = "已完成"
FAILED = "已失败"
CANCELLED = "已取消"

ALL_STATUSES = (QUEUED, PREPARING, RUNNING, FINISHED, FAILED, CANCELLED)
