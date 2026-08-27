"""共享工具函数。"""

import re
from typing import Optional

_TIME_UNITS = {"d": 86400, "h": 3600, "m": 60, "s": 1}

_DURATION_RE = re.compile(r"(\d+(?:\.\d+)?)([dhms])")


def parse_duration(s: str) -> Optional[float]:
    """解析时长字符串（d/h/m/s 自由组合，可含小数，如 1h30m、1.5h、90m）为秒数；空或非法返回 None。"""
    if not s:
        return None
    total = 0.0
    for num, unit in _DURATION_RE.findall(s):
        total += float(num) * _TIME_UNITS[unit]
    return total or None
