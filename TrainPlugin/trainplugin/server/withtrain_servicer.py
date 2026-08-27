"""WithTrain 服务实现（totrain.proto）。"""

import logging

import grpc

from ..data.decode import DecodeError, decode_to_samples
from ..proto_gen import totrain_pb2, totrain_pb2_grpc
from ..task import status as task_status
from ..task.manager import TaskManager
from ..train.task_log import TaskLogger

logger = logging.getLogger(__name__)


def _check_cancelled(context):
    if not context.is_active():
        context.abort(grpc.StatusCode.CANCELLED, "client cancelled")


class WithTrainServicer(totrain_pb2_grpc.WithTrainServicer):
    def __init__(self, manager: TaskManager, task_log: TaskLogger) -> None:
        self._manager = manager
        self._task_log = task_log

    def Check(self, request, context):
        _check_cancelled(context)
        return totrain_pb2.CheckResponse(IsOK=True, Msg="ok")

    def SendTrain(self, request, context):
        _check_cancelled(context)
        task = self._manager.get_task(request.Id)
        if task is None:
            return totrain_pb2.ToTrainResponse(IsOK=False, Msg="no matching training task")
        if task.status not in (task_status.QUEUED, task_status.PREPARING):
            return totrain_pb2.ToTrainResponse(IsOK=False, Msg="task is not collecting data")

        try:
            samples = decode_to_samples(request)
        except DecodeError as exc:
            logger.warning("task %s: decode error: %s", task.id, exc)
            task.set_error(str(exc))
            self._task_log.write(task, task.status, str(exc))
            return totrain_pb2.ToTrainResponse(IsOK=False, Msg=str(exc))

        if not self._manager.receive_data(request.Id, samples):
            return totrain_pb2.ToTrainResponse(IsOK=False, Msg="no matching training task")

        return totrain_pb2.ToTrainResponse(IsOK=True)

    def Finish(self, request, context):
        _check_cancelled(context)
        task = self._manager.get_task(request.Id)
        if task is None:
            return totrain_pb2.ToTrainResponse(IsOK=False, Msg="no matching training task")

        task.data_ready_event.set()
        return totrain_pb2.ToTrainResponse(IsOK=True)

    def ReportError(self, request, context):
        _check_cancelled(context)
        ok = self._manager.handle_data_error(request.Id, request.Error)
        if not ok:
            return totrain_pb2.ToTrainResponse(IsOK=False, Msg="no matching training task")
        return totrain_pb2.ToTrainResponse(IsOK=True)
