"""重新生成 Python gRPC 桩，并修复生成的相对导入。

用法：python gen_proto.py
"""

import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).parent
PROTO_DIR = ROOT / "proto"
OUT_DIR = ROOT / "trainplugin" / "proto_gen"

PROTOS = ["sender.proto", "totrain.proto", "receive.proto"]


def main() -> None:
    OUT_DIR.mkdir(parents=True, exist_ok=True)
    cmd = [
        sys.executable,
        "-m",
        "grpc_tools.protoc",
        f"-I{PROTO_DIR}",
        f"--python_out={OUT_DIR}",
        f"--grpc_python_out={OUT_DIR}",
    ] + [str(PROTO_DIR / p) for p in PROTOS]
    subprocess.run(cmd, check=True)

    for name in ["sender_pb2_grpc.py", "totrain_pb2_grpc.py", "receive_pb2_grpc.py"]:
        path = OUT_DIR / name
        base = name.replace("_pb2_grpc.py", "")
        text = path.read_text(encoding="utf-8")
        text = text.replace(
            f"import {base}_pb2 as {base}__pb2",
            f"from . import {base}_pb2 as {base}__pb2",
        )
        path.write_text(text, encoding="utf-8")

    print("proto stubs regenerated and patched")


if __name__ == "__main__":
    main()
