"""样本缓冲 -> torch Dataset / DataLoader。

- 依据首次出现顺序建立 label_vocab，并校验唯一标签数不超过 CategoriesNumber。
- 按 Validation 百分比随机划分训练/验证集。
- 文本/词序列按 batch 动态 padding。
"""

import random
from typing import Dict, List, Optional, Tuple

import numpy as np
import torch
from torch.utils.data import DataLoader, Dataset

DEFAULT_BATCH_SIZE = 32


def _to_image_tensor(arr: np.ndarray) -> torch.Tensor:
    t = torch.from_numpy(np.ascontiguousarray(arr))
    return t.permute(2, 0, 1).float()


def _to_long(arr: np.ndarray) -> torch.Tensor:
    t = torch.from_numpy(np.ascontiguousarray(arr))
    return t.round().clamp(0, 255).long()


class SampleDataset(Dataset):
    def __init__(self, samples: List[Tuple[str, Dict[str, np.ndarray], int]]) -> None:
        self.samples = samples

    def __len__(self) -> int:
        return len(self.samples)

    def __getitem__(self, index: int):
        return self.samples[index]


def collate_fn(batch):
    kind = batch[0][0]
    labels = torch.tensor([s[2] for s in batch], dtype=torch.long)

    if kind == "image":
        imgs = torch.stack([_to_image_tensor(s[1]["image"]) for s in batch])
        return kind, {"image": imgs}, labels

    if kind == "txt":
        seqs = [_to_long(s[1]["txt"]) for s in batch]
        padded = torch.nn.utils.rnn.pad_sequence(seqs, batch_first=True)
        return kind, {"txt": padded}, labels

    if kind == "mix":
        imgs = torch.stack([_to_image_tensor(s[1]["image"]) for s in batch])
        words = [_to_long(s[1]["word"]) for s in batch]
        words_padded = torch.nn.utils.rnn.pad_sequence(words, batch_first=True)
        return kind, {"image": imgs, "word": words_padded}, labels

    raise ValueError(f"unknown sample kind {kind!r}")


def build_loaders(
    samples: List[Tuple[str, Dict[str, np.ndarray], str]],
    validation: int,
    categories_number: int,
    batch_size: int = DEFAULT_BATCH_SIZE,
):
    """返回 (train_loader, val_loader_or_None, label_vocab)。"""
    if not samples:
        raise ValueError("no training samples")

    vocab: Dict[str, int] = {}
    for _, _, label in samples:
        if label not in vocab:
            vocab[label] = len(vocab)

    if len(vocab) > categories_number:
        raise ValueError(f"unique labels {len(vocab)} exceed categories {categories_number}")

    encoded = [(kind, tensors, vocab[label]) for kind, tensors, label in samples]

    n = len(encoded)
    indices = list(range(n))
    random.shuffle(indices)

    val_n = int(round(n * validation / 100.0)) if 0 < validation < 100 else 0
    val_n = min(val_n, n - 1) if n > 1 else 0

    train_indices = indices[val_n:]
    val_indices = indices[:val_n]

    train_dataset = SampleDataset([encoded[i] for i in train_indices])
    train_loader = DataLoader(
        train_dataset, batch_size=batch_size, shuffle=True, collate_fn=collate_fn
    )

    val_loader: Optional[DataLoader] = None
    if val_indices:
        val_dataset = SampleDataset([encoded[i] for i in val_indices])
        val_loader = DataLoader(
            val_dataset, batch_size=batch_size, shuffle=False, collate_fn=collate_fn
        )

    return train_loader, val_loader, vocab
