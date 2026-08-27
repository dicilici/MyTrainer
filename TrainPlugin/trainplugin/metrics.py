"""无依赖的系统指标采集（Linux /proc + 标准库）。

返回四项使用占比（百分比，0~100 或 0~100+）：
CPU、内存、磁盘、磁盘IO（iostat 式 %util）。
"""

import shutil
import time

_DEFAULT_INTERVAL = 0.2


def _cpu_times():
    with open("/proc/stat") as f:
        parts = f.readline().split()
    vals = [int(x) for x in parts[1:8]]  # user nice system idle iowait irq softirq steal
    total = sum(vals)
    idle = vals[3] + vals[4]  # idle + iowait
    return total, idle


def _mem_percent():
    info = {}
    with open("/proc/meminfo") as f:
        for line in f:
            key, _, value = line.partition(":")
            info[key.strip()] = int(value.split()[0])
    return (1 - info["MemAvailable"] / info["MemTotal"]) * 100


def _disk_percent():
    usage = shutil.disk_usage("/")
    return (1 - usage.free / usage.total) * 100


def _root_device():
    with open("/proc/mounts") as f:
        for line in f:
            parts = line.split()
            if len(parts) >= 2 and parts[1] == "/":
                return parts[0].rsplit("/", 1)[-1]
    return ""


def _disk_io_ms():
    dev = _root_device()
    if not dev:
        return 0.0
    with open("/proc/diskstats") as f:
        for line in f:
            parts = line.split()
            if len(parts) >= 13 and parts[2] == dev:
                return int(parts[12])  # time_doing_io（毫秒）
    return 0.0


def collect(interval=_DEFAULT_INTERVAL):
    """采集一次指标，返回 (cpu, memory, disk, disk_io) 四个百分比。"""
    try:
        total0, idle0 = _cpu_times()
        io0 = _disk_io_ms()
        time.sleep(interval)
        total1, idle1 = _cpu_times()
        io1 = _disk_io_ms()

        cpu = (1 - (idle1 - idle0) / (total1 - total0)) * 100 if total1 > total0 else 0.0
        mem = _mem_percent()
        disk = _disk_percent()
        diskio = (io1 - io0) / (interval * 1000) * 100
        return cpu, mem, disk, diskio
    except Exception:
        return 0.0, 0.0, 0.0, 0.0
