"""RemoteLog gRPC 客户端，负责向训练后端上报 TrainRemote 状态。"""

import logging
import threading
from typing import Dict, Optional

import grpc

from ..proto_gen import receive_pb2, receive_pb2_grpc

logger = logging.getLogger(__name__)

_REPORT_TIMEOUT = 5.0


class RemoteLogClient:
    def __init__(self) -> None:
        self._channels: Dict[str, grpc.Channel] = {}
        self._stubs: Dict[str, receive_pb2_grpc.RemoteLogStub] = {}
        self._lock = threading.Lock()

    def _get_stub(self, url: str) -> receive_pb2_grpc.RemoteLogStub:
        with self._lock:
            stub = self._stubs.get(url)
            if stub is None:
                channel = grpc.insecure_channel(url)
                self._channels[url] = channel
                stub = receive_pb2_grpc.RemoteLogStub(channel)
                self._stubs[url] = stub
            return stub

    def report(self, url: str, status: str, log_msg: str, task_id: str) -> None:
        if not url:
            logger.warning("task %s: empty remote log url, skip report", task_id)
            return
        try:
            stub = self._get_stub(url)
            msg = receive_pb2.TrainMsg(Status=status, LogMsg=log_msg, TaskId=task_id)
            stub.TrainRemote(msg, timeout=_REPORT_TIMEOUT)
        except grpc.RpcError as exc:
            logger.warning("task %s: failed to report '%s': %s", task_id, status, exc)

    def close(self) -> None:
        with self._lock:
            for channel in self._channels.values():
                channel.close()
            self._channels.clear()
            self._stubs.clear()
