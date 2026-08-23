"""损失函数名称 -> 损失计算规格。

支持（大小写不敏感）：
- CrossEntropyLoss / cross_entropy / ce
- BCEWithLogitsLoss / bce_with_logits / bce
- MSELoss / mse / mean_squared_error
"""

import torch
import torch.nn as nn

_CROSS_ENTROPY = {"crossentropyloss", "cross_entropy", "crossentropy", "ce"}
_BCE = {"bcewithlogitsloss", "bce_with_logits", "bce", "binary_cross_entropy"}
_MSE = {"mseloss", "mse", "mean_squared_error"}


class CrossEntropySpec:
    def __init__(self, num_classes: int) -> None:
        self.criterion = nn.CrossEntropyLoss()

    def __call__(self, output: torch.Tensor, labels: torch.Tensor) -> torch.Tensor:
        return self.criterion(output, labels)


class BCESpec:
    def __init__(self, num_classes: int) -> None:
        if num_classes not in (1, 2):
            raise ValueError("BCEWithLogitsLoss requires num_classes 1 or 2")
        self.criterion = nn.BCEWithLogitsLoss()

    def __call__(self, output: torch.Tensor, labels: torch.Tensor) -> torch.Tensor:
        out = output
        if out.dim() == 2 and out.shape[1] == 1:
            out = out.squeeze(1)
        if out.dim() != 1:
            raise ValueError(f"BCEWithLogitsLoss expects [B] or [B,1] output, got {tuple(output.shape)}")
        return self.criterion(out, labels.float())


class MSESpec:
    def __init__(self, num_classes: int) -> None:
        self.num_classes = num_classes
        self.criterion = nn.MSELoss()

    def __call__(self, output: torch.Tensor, labels: torch.Tensor) -> torch.Tensor:
        target = torch.nn.functional.one_hot(labels, num_classes=self.num_classes).float()
        return self.criterion(output, target)


def resolve_loss(name: str, num_classes: int):
    key = (name or "").strip().lower()
    if key in _CROSS_ENTROPY:
        return CrossEntropySpec(num_classes)
    if key in _BCE:
        return BCESpec(num_classes)
    if key in _MSE:
        return MSESpec(num_classes)
    raise ValueError(f"unsupported loss function: {name!r}")
