"""内置模型类与短名注册表。

模型结构由预存模型的 JSON 定义文件通过「短名」选择，
权重来自同目录下的 .pt 文件。此处仅提供训练端内置的模型类实现。

约定（均使用全局平均池化，输入空间尺寸无关，便于从磁盘加载权重）：
- ImageCNN       : 输入 [B, C, H, W]（默认 C=3），输出 [B, num_classes]
- TextClassifier : 输入 [B, L]（字节级 long 索引），输出 [B, num_classes]
- MixClassifier  : 输入 image [B, C, H, W] + word [B, L]，输出 [B, num_classes]
"""

from typing import Type

import torch
import torch.nn as nn

MODEL_REGISTRY: "dict[str, Type[nn.Module]]" = {}


def register(name: str):
    def decorator(cls: Type[nn.Module]) -> Type[nn.Module]:
        MODEL_REGISTRY[name] = cls
        return cls

    return decorator


@register("ImageCNN")
class ImageCNN(nn.Module):
    def __init__(self, in_channels: int = 3, num_classes: int = 10, hidden: int = 128):
        super().__init__()
        self.features = nn.Sequential(
            nn.Conv2d(in_channels, 32, kernel_size=3, padding=1),
            nn.BatchNorm2d(32),
            nn.ReLU(inplace=True),
            nn.MaxPool2d(2),
            nn.Conv2d(32, 64, kernel_size=3, padding=1),
            nn.BatchNorm2d(64),
            nn.ReLU(inplace=True),
            nn.MaxPool2d(2),
            nn.Conv2d(64, 128, kernel_size=3, padding=1),
            nn.BatchNorm2d(128),
            nn.ReLU(inplace=True),
            nn.AdaptiveAvgPool2d(1),
        )
        self.head = nn.Sequential(
            nn.Flatten(),
            nn.Linear(128, hidden),
            nn.ReLU(inplace=True),
            nn.Linear(hidden, num_classes),
        )

    def forward(self, x: torch.Tensor) -> torch.Tensor:
        return self.head(self.features(x))


@register("TextClassifier")
class TextClassifier(nn.Module):
    def __init__(self, vocab_size: int = 256, embed_dim: int = 64, num_classes: int = 10, hidden: int = 128):
        super().__init__()
        self.embedding = nn.Embedding(vocab_size, embed_dim, padding_idx=0)
        self.head = nn.Sequential(
            nn.Linear(embed_dim, hidden),
            nn.ReLU(inplace=True),
            nn.Linear(hidden, num_classes),
        )

    def forward(self, x: torch.Tensor) -> torch.Tensor:
        # x: [B, L] long，取均值池化得到 [B, embed_dim]
        emb = self.embedding(x)
        pooled = emb.mean(dim=1)
        return self.head(pooled)


@register("MixClassifier")
class MixClassifier(nn.Module):
    def __init__(
        self,
        in_channels: int = 3,
        vocab_size: int = 256,
        embed_dim: int = 64,
        num_classes: int = 10,
        hidden: int = 128,
    ):
        super().__init__()
        self.image_branch = nn.Sequential(
            nn.Conv2d(in_channels, 32, kernel_size=3, padding=1),
            nn.BatchNorm2d(32),
            nn.ReLU(inplace=True),
            nn.MaxPool2d(2),
            nn.Conv2d(32, 64, kernel_size=3, padding=1),
            nn.BatchNorm2d(64),
            nn.ReLU(inplace=True),
            nn.AdaptiveAvgPool2d(1),
            nn.Flatten(),
        )
        self.word_embedding = nn.Embedding(vocab_size, embed_dim, padding_idx=0)
        self.head = nn.Sequential(
            nn.Linear(64 + embed_dim, hidden),
            nn.ReLU(inplace=True),
            nn.Linear(hidden, num_classes),
        )

    def forward(self, image: torch.Tensor, word: torch.Tensor) -> torch.Tensor:
        img_feat = self.image_branch(image)
        word_feat = self.word_embedding(word).mean(dim=1)
        fused = torch.cat([img_feat, word_feat], dim=1)
        return self.head(fused)
