"""预存模型的加载与写回。

模型文件夹约定（ModelSave 绝对路径）：
- 恰好一个 *.pt 权重文件
- 恰好一个 *.json 结构定义文件

JSON 结构示例：
    {
      "name": "ImageCNN",
      "args": {"in_channels": 3, "num_classes": 10},
      "input_type": "image",
      "num_classes": 10
    }
"""

import json
from pathlib import Path
from typing import Any, Dict, Optional, Tuple

import torch
import torch.nn as nn

from .registry import MODEL_REGISTRY


class ModelLoadError(Exception):
    pass


class ModelLoader:
    def __init__(self, registry: Optional[Dict[str, type]] = None) -> None:
        self._registry = registry or MODEL_REGISTRY

    def _discover(self, folder: str) -> Tuple[Path, Path]:
        root = Path(folder)
        if not root.is_dir():
            raise ModelLoadError(f"model folder does not exist: {folder}")

        pts = list(root.glob("*.pt"))
        jsons = list(root.glob("*.json"))
        if len(pts) != 1:
            raise ModelLoadError(f"expected exactly one .pt file in {folder}, found {len(pts)}")
        if len(jsons) != 1:
            raise ModelLoadError(f"expected exactly one .json file in {folder}, found {len(jsons)}")
        return pts[0], jsons[0]

    def load(self, folder: str, input_type: str, num_classes: int) -> Tuple[nn.Module, Dict[str, Any]]:
        pt_path, json_path = self._discover(folder)
        try:
            spec = json.loads(json_path.read_text(encoding="utf-8"))
        except json.JSONDecodeError as exc:
            raise ModelLoadError(f"invalid model json {json_path}: {exc}") from exc

        if not isinstance(spec, dict):
            raise ModelLoadError(f"model json {json_path} must be an object")

        name = spec.get("name")
        if not isinstance(name, str) or name not in self._registry:
            raise ModelLoadError(f"unknown model name {name!r}")

        args = spec.get("args") or {}
        if not isinstance(args, dict):
            raise ModelLoadError("model json 'args' must be an object")

        try:
            model = self._registry[name](**args)
        except TypeError as exc:
            raise ModelLoadError(f"failed to instantiate model {name!r}: {exc}") from exc

        state = torch.load(pt_path, map_location="cpu")
        if isinstance(state, dict) and "state_dict" in state:
            state = state["state_dict"]
        model.load_state_dict(state)
        model.eval()

        self._validate(spec, input_type, num_classes)
        return model, spec

    @staticmethod
    def _validate(spec: Dict[str, Any], input_type: str, num_classes: int) -> None:
        spec_type = spec.get("input_type")
        if spec_type is not None and spec_type != input_type:
            raise ModelLoadError(
                f"model input_type {spec_type!r} mismatch dataset input {input_type!r}"
            )
        spec_classes = spec.get("num_classes")
        if spec_classes is not None and int(spec_classes) != int(num_classes):
            raise ModelLoadError(
                f"model num_classes {spec_classes} mismatch dataset categories {num_classes}"
            )

    def save(self, folder: str, model: nn.Module) -> None:
        pt_path, _ = self._discover(folder)
        torch.save(model.state_dict(), pt_path)
