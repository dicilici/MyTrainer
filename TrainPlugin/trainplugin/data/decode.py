"""张量解码与输入归一化。

按 DatabasePlugin handler.go 的编码约定解析：
- TensorContent 为 float32 小端字节流，形状由 Tensor.Dim 给出。
- 图像 : [H, W, 3]（RGB，0~1）
- 文本 : [L]（字节码 0~255 的 float32）
- 混合 : image [N, H, W, 3] + word [N, maxLen]
"""

from typing import Dict, List, Tuple

import numpy as np

IMAGE_KEYS = {"jpg", "jpeg", "png", "image"}
MIX_KEYS = {"json", "mix"}


class DecodeError(Exception):
    pass


def decode_tensor(tensor) -> np.ndarray:
    dim = list(tensor.Dim)
    content = bytes(tensor.TensorContent)
    arr = np.frombuffer(content, dtype="<f4")

    if not dim:
        return arr

    expected = int(np.prod(dim))
    if arr.size != expected:
        raise DecodeError(f"tensor content size {arr.size} != prod(dim) {expected}")
    return arr.reshape(dim)


def _single(raw: Dict[str, np.ndarray]) -> np.ndarray:
    return next(iter(raw.values()))


def normalize_inputs(to_train) -> Tuple[str, Dict[str, np.ndarray]]:
    """将 ToTrain.inputs 归一化为 (kind, {canonical_key: ndarray})。

    kind 为 txt / image / mix 之一。
    """
    dtype = (to_train.DType or "").lower()
    raw = {k: decode_tensor(v) for k, v in to_train.inputs.items()}
    if not raw:
        raise DecodeError("empty inputs")

    if dtype == "txt":
        return "txt", {"txt": raw["txt"] if "txt" in raw else _single(raw)}

    if dtype in MIX_KEYS:
        if "image" not in raw or "word" not in raw:
            raise DecodeError("mix input requires 'image' and 'word' keys")
        return "mix", {"image": raw["image"], "word": raw["word"]}

    if "image" in raw:
        return "image", {"image": raw["image"]}

    if dtype in IMAGE_KEYS:
        return "image", {"image": _single(raw)}

    raise DecodeError(f"unsupported DType {to_train.DType!r}")


def decode_to_samples(to_train) -> List[Tuple[str, Dict[str, np.ndarray], str]]:
    """把一条 ToTrain 消息解码为若干条样本 (kind, tensors, label)。

    混合数据一条 ToTrain 含 N 个 (image, word) 对，展开为 N 条样本。
    """
    kind, tensors = normalize_inputs(to_train)
    label = to_train.Type

    if kind == "mix":
        image = tensors["image"]
        word = tensors["word"]
        if image.ndim != 4 or word.ndim != 2:
            raise DecodeError(f"mix image/word shape unexpected: {image.shape} / {word.shape}")
        n = image.shape[0]
        if word.shape[0] != n:
            raise DecodeError(f"mix image/word batch mismatch: {image.shape[0]} vs {word.shape[0]}")
        return [("mix", {"image": image[i], "word": word[i]}, label) for i in range(n)]

    return [(kind, tensors, label)]
