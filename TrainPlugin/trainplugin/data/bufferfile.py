"""排队任务数据落盘缓冲（方案 D：numpy .npy + 长度前缀）。

每条样本 ``(kind, {key: ndarray}, label)`` 落盘为一条记录：

    [4字节 记录长度][记录体]

记录体：

    [4字节 label 长度][label UTF-8]
    [4字节 kind 长度][kind UTF-8]
    [4字节 张量数量 N]
    重复 N 次：
        [4字节 key 长度][key UTF-8]
        [4字节 npy 长度][npy 块]

npy 块由 ``np.save`` 写入 ``BytesIO`` 得到（自带 dtype/shape）。
长度前缀采用大端 uint32，避免二进制内容与分隔符冲突。
"""

import io
import struct

import numpy as np

_U32 = ">I"


def _pack(n: int) -> bytes:
    return struct.pack(_U32, n)


def _read_u32(data: bytes, offset: int):
    return struct.unpack_from(_U32, data, offset)[0], offset + 4


def encode_sample(sample) -> bytes:
    """(kind, tensors, label) -> 记录体字节。"""
    kind, tensors, label = sample
    buf = io.BytesIO()

    label_b = label.encode("utf-8")
    buf.write(_pack(len(label_b)))
    buf.write(label_b)

    kind_b = kind.encode("utf-8")
    buf.write(_pack(len(kind_b)))
    buf.write(kind_b)

    buf.write(_pack(len(tensors)))
    for key, arr in tensors.items():
        key_b = key.encode("utf-8")
        buf.write(_pack(len(key_b)))
        buf.write(key_b)

        npy_buf = io.BytesIO()
        np.save(npy_buf, arr, allow_pickle=False)
        npy_b = npy_buf.getvalue()
        buf.write(_pack(len(npy_b)))
        buf.write(npy_b)

    return buf.getvalue()


def append_samples(path: str, samples) -> None:
    """把一批样本追加写入文件，每条记录带长度前缀。"""
    with open(path, "ab") as f:
        for sample in samples:
            body = encode_sample(sample)
            f.write(_pack(len(body)))
            f.write(body)


def _decode_record(body: bytes):
    offset = 0
    label_len, offset = _read_u32(body, offset)
    label = body[offset:offset + label_len].decode("utf-8")
    offset += label_len

    kind_len, offset = _read_u32(body, offset)
    kind = body[offset:offset + kind_len].decode("utf-8")
    offset += kind_len

    num, offset = _read_u32(body, offset)
    tensors = {}
    for _ in range(num):
        key_len, offset = _read_u32(body, offset)
        key = body[offset:offset + key_len].decode("utf-8")
        offset += key_len

        npy_len, offset = _read_u32(body, offset)
        npy_b = body[offset:offset + npy_len]
        offset += npy_len
        tensors[key] = np.load(io.BytesIO(npy_b), allow_pickle=False)

    return kind, tensors, label


def read_samples(path: str) -> list:
    """逐条流式读取文件，还原为样本列表 [(kind, {key: ndarray}, label), ...]。

    不整读整个文件：每条记录先读 4 字节长度头，再读该条 body 并立即解码，
    避免「整文件字节 + 全部数组」同时驻留内存。
    """
    samples = []
    with open(path, "rb") as f:
        while True:
            header = f.read(4)
            if len(header) == 0:
                break
            if len(header) < 4:
                raise ValueError("truncated record header")
            body_len = struct.unpack(_U32, header)[0]
            body = f.read(body_len)
            if len(body) < body_len:
                raise ValueError("truncated record body")
            samples.append(_decode_record(body))
    return samples
