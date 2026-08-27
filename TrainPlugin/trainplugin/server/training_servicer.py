"""Training 服务实现（sender.proto）。"""

import logging
from typing import Optional

import grpc

from .. import metrics
from ..proto_gen import sender_pb2, sender_pb2_grpc
from ..task.manager import TaskManager
from ..task.task import DatasetConfig, TrainConfig, TrainingTask

logger = logging.getLogger(__name__)

_VALID_INPUTS = {"txt", "image", "mix"}


def _check_cancelled(context):
    if not context.is_active():
        context.abort(grpc.StatusCode.CANCELLED, "client cancelled")


class TrainingServicer(sender_pb2_grpc.TrainingServicer):
    def __init__(self, manager: TaskManager) -> None:
        self._manager = manager

    def SendToTrain(self, request, context):
        _check_cancelled(context)
        error = self._validate(request)
        if error:
            return sender_pb2.Response(IsOK=False, Errors=error)

        task = TrainingTask(
            id=request.id,
            remote_log_url=request.RemoteLogUrl,
            dataset=DatasetConfig(
                input=request.Dataset.Input,
                validation=int(request.Dataset.Validation),
                categories_number=int(request.Dataset.CategoriesNumber),
            ),
            train_config=TrainConfig(
                epochs=int(request.TrainConfig.Epochs),
                learning_rate=float(request.TrainConfig.LearningRate),
                loss_function=request.TrainConfig.LossFunction,
                early_stop=bool(request.TrainConfig.EarlyStop),
                early_stop_patient=int(request.TrainConfig.EarlyStopPatient),
                model_save=request.TrainConfig.ModelSave,
                log_save=request.TrainConfig.LogSave,
                timeout=request.TrainConfig.TimeOut,
            ),
        )
        self._manager.submit(task)
        logger.info("task %s submitted", task.id)
        return sender_pb2.Response(IsOK=True)

    def QueryTraining(self, request, context):
        _check_cancelled(context)
        task = self._manager.current()
        if task is None:
            return sender_pb2.QueryResponse(
                Done=False, Epoch=0, loss=[], idOK=False, Errors="no active training task"
            )

        epoch, loss, done, _, error = task.snapshot()
        if error:
            return sender_pb2.QueryResponse(
                Done=False, Epoch=epoch, loss=loss, idOK=False, Errors=error
            )
        return sender_pb2.QueryResponse(Done=done, Epoch=epoch, loss=loss, idOK=True)

    def CancelTraining(self, request, context):
        _check_cancelled(context)
        task = self._manager.cancel()
        if task is None:
            return sender_pb2.CancelResponse(isOK=False, Errors="no active training task")
        return sender_pb2.CancelResponse(isOK=True)

    def CheckNode(self, request, context):
        _check_cancelled(context)
        cpu, memory, disk, disk_io = metrics.collect()
        return sender_pb2.CheckNodeReply(Cpu=cpu, Memory=memory, Disk=disk, DiskIO=disk_io)

    @staticmethod
    def _validate(request) -> Optional[str]:
        if not request.id:
            return "missing task id"
        if not request.RemoteLogUrl:
            return "missing RemoteLogUrl"
        if request.Dataset.Input not in _VALID_INPUTS:
            return f"invalid Input {request.Dataset.Input!r}, expect txt/image/mix"
        if request.TrainConfig.ModelSave == "":
            return "missing ModelSave"
        return None
