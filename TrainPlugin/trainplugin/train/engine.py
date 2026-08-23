"""训练引擎：数据装载、模型加载、训练循环、早停、保存。"""

import logging
import time
from typing import Optional

import torch

from ..data.dataset import build_loaders
from ..model.loader import ModelLoader
from ..task.task import TaskCancelled, TrainingTask
from .early_stop import EarlyStopping
from .loss import resolve_loss
from .task_log import TaskLogger

logger = logging.getLogger(__name__)

_DEVICE = torch.device("cuda" if torch.cuda.is_available() else "cpu")


def _forward(model, kind: str, inputs):
    if kind == "image":
        return model(inputs["image"])
    if kind == "txt":
        return model(inputs["txt"])
    if kind == "mix":
        return model(inputs["image"], inputs["word"])
    raise ValueError(f"unknown sample kind {kind!r}")


def _move(inputs, labels, device):
    inputs = {k: v.to(device) for k, v in inputs.items()}
    labels = labels.to(device)
    return inputs, labels


def _check_cancel(task: TrainingTask) -> None:
    if task.cancel_event.is_set() or task.timeout_event.is_set():
        raise TaskCancelled()


def _run_epoch(model, loader, loss_spec, task: TrainingTask, optimizer=None) -> float:
    train = optimizer is not None
    model.train(train)
    total_loss = 0.0
    count = 0
    with torch.set_grad_enabled(train):
        for kind, inputs, labels in loader:
            _check_cancel(task)
            inputs, labels = _move(inputs, labels, _DEVICE)
            output = _forward(model, kind, inputs)
            loss = loss_spec(output, labels)
            if train:
                optimizer.zero_grad()
                loss.backward()
                optimizer.step()
            total_loss += loss.item() * labels.size(0)
            count += labels.size(0)
    if count == 0:
        return 0.0
    return total_loss / count


def prepare(task: TrainingTask):
    """准备段：读任务缓冲 + 建 DataLoader，返回 (train_loader, val_loader, n_samples)。"""
    samples = list(task.buffer)
    train_loader, val_loader, vocab = build_loaders(
        samples,
        validation=task.dataset.validation,
        categories_number=task.dataset.categories_number,
    )
    with task.lock:
        task.label_vocab = dict(vocab)
    return train_loader, val_loader, len(samples)


def train(task: TrainingTask, prepared, task_log: TaskLogger) -> None:
    """训练段：加载模型 + 训练循环 + 保存。prepared 由 prepare() 产出。"""
    cfg = task.train_config
    train_loader, val_loader, n_samples = prepared

    loader = ModelLoader()
    model, _ = loader.load(
        cfg.model_save,
        input_type=task.dataset.input,
        num_classes=task.dataset.categories_number,
    )
    model.to(_DEVICE)

    loss_spec = resolve_loss(cfg.loss_function, task.dataset.categories_number)
    optimizer = torch.optim.Adam(model.parameters(), lr=cfg.learning_rate)
    early_stop = EarlyStopping(cfg.early_stop_patient) if cfg.early_stop else None

    task_log.write(task, task.status, f"task {task.id}: start training, epochs={cfg.epochs}, samples={n_samples}")

    for epoch in range(1, cfg.epochs + 1):
        _check_cancel(task)
        epoch_start = time.time()
        train_loss = _run_epoch(model, train_loader, loss_spec, task, optimizer=optimizer)

        val_loss: Optional[float] = None
        if val_loader is not None:
            val_loss = _run_epoch(model, val_loader, loss_spec, task, optimizer=None)

        duration = time.time() - epoch_start
        metric = val_loss if val_loss is not None else train_loss
        task.append_loss(train_loss)

        msg = f"epoch {epoch}/{cfg.epochs}: train_loss={train_loss:.6f}"
        if val_loss is not None:
            msg += f", val_loss={val_loss:.6f}"
        msg += f", learning_rate={cfg.learning_rate}, duration={duration:.3f}s"
        task_log.write(task, task.status, msg)

        if early_stop is not None:
            early_stop.step(metric)
            if early_stop.stop:
                task_log.write(task, task.status, f"early stopping triggered at epoch {epoch}")
                break

    loader.save(cfg.model_save, model)
