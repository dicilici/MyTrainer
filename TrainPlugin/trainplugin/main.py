"""TrainPlugin 入口。

启动一个 gRPC server，同时注册 Training（sender.proto）与 WithTrain（totrain.proto）
两个服务，并拉起单 worker 线程串行处理训练任务。
"""

import logging
import os
import time
from concurrent import futures

import grpc

from .client.remote_log import RemoteLogClient
from .config import TrainPluginConfig
from .proto_gen import sender_pb2_grpc, totrain_pb2_grpc
from .server.training_servicer import TrainingServicer
from .server.withtrain_servicer import WithTrainServicer
from .task.manager import TaskManager
from .train import engine
from .train.task_log import TaskLogger
from .util import parse_duration

logger = logging.getLogger(__name__)


def load_config() -> TrainPluginConfig:
    listen = os.environ.get("TRAIN_LISTEN", "0.0.0.0:50052")
    withtrain_listen = os.environ.get("TRAIN_WITHTRAIN_LISTEN", "0.0.0.0:50054")
    max_workers = int(os.environ.get("TRAIN_MAX_WORKERS", "32"))
    timeout_raw = os.environ.get("TRAIN_DATA_WAIT_TIMEOUT")
    data_wait_timeout = parse_duration(timeout_raw or "")
    return TrainPluginConfig(
        listen=listen,
        withtrain_listen=withtrain_listen,
        max_workers=max_workers,
        data_wait_timeout=data_wait_timeout,
    )


def resolve_log_dir() -> str:
    log_dir = os.environ.get("TRAIN_LOG")
    if not log_dir:
        raise SystemExit("TRAIN_LOG environment variable is not set")
    return log_dir


def serve(cfg: TrainPluginConfig) -> None:
    remote_log = RemoteLogClient()
    task_log = TaskLogger(resolve_log_dir())

    def reporter(task, status, msg):
        remote_log.report(task.remote_log_url, status, msg, task.id)
        task_log.write(task, status, msg)

    def prep_fn(task):
        return engine.prepare(task)

    def train_fn(task, prepared):
        engine.train(task, prepared, task_log)

    manager = TaskManager(
        prep_fn=prep_fn,
        train_fn=train_fn,
        reporter=reporter,
        data_wait_timeout=cfg.data_wait_timeout,
    )

    training_server = grpc.server(futures.ThreadPoolExecutor(max_workers=cfg.max_workers))
    sender_pb2_grpc.add_TrainingServicer_to_server(TrainingServicer(manager), training_server)
    training_server.add_insecure_port(cfg.listen)
    training_server.start()
    logger.info("TrainPlugin Training service listening on %s", cfg.listen)

    withtrain_server = grpc.server(futures.ThreadPoolExecutor(max_workers=cfg.max_workers))
    totrain_pb2_grpc.add_WithTrainServicer_to_server(WithTrainServicer(manager, task_log), withtrain_server)
    withtrain_server.add_insecure_port(cfg.withtrain_listen)
    withtrain_server.start()
    logger.info("TrainPlugin WithTrain service listening on %s", cfg.withtrain_listen)

    try:
        while True:
            time.sleep(3600)
    except KeyboardInterrupt:
        logger.info("shutting down")
    finally:
        training_server.stop(grace=5)
        withtrain_server.stop(grace=5)
        manager.close()
        remote_log.close()


def main() -> None:
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(levelname)s %(name)s: %(message)s",
    )
    serve(load_config())


if __name__ == "__main__":
    main()
