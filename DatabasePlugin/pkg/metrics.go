package pkg

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

const metricsInterval = 200 * time.Millisecond

func readCPUTimes() (total, idle uint64, err error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return 0, 0, sc.Err()
	}
	parts := strings.Fields(sc.Text())
	if len(parts) < 8 {
		return 0, 0, os.ErrInvalid
	}
	var vals [7]uint64
	for i := 0; i < 7; i++ {
		v, err := strconv.ParseUint(parts[i+1], 10, 64)
		if err != nil {
			return 0, 0, err
		}
		vals[i] = v
	}
	for _, v := range vals {
		total += v
	}
	idle = vals[3] + vals[4]
	return total, idle, nil
}

func readMemPercent() (float32, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer f.Close()
	info := make(map[string]uint64)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		kv := strings.SplitN(sc.Text(), ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		fields := strings.Fields(kv[1])
		if len(fields) == 0 {
			continue
		}
		n, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			continue
		}
		info[key] = n
	}
	total := info["MemTotal"]
	avail := info["MemAvailable"]
	if total == 0 {
		return 0, os.ErrInvalid
	}
	return float32(1-float64(avail)/float64(total)) * 100, nil
}

func rootDevice() string {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) >= 2 && fields[1] == "/" {
			idx := strings.LastIndex(fields[0], "/")
			return fields[0][idx+1:]
		}
	}
	return ""
}

func readDiskIOMs() (float64, error) {
	dev := rootDevice()
	if dev == "" {
		return 0, os.ErrNotExist
	}
	f, err := os.Open("/proc/diskstats")
	if err != nil {
		return 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.Fields(sc.Text())
		if len(parts) >= 13 && parts[2] == dev {
			return strconv.ParseFloat(parts[12], 64)
		}
	}
	return 0, os.ErrNotExist
}

// CollectMetrics 采集 CPU、内存、磁盘、磁盘IO 使用占比（百分比）。
func CollectMetrics() (cpu, memory, disk, diskIO float32) {
	t0, i0, err0 := readCPUTimes()
	io0, _ := readDiskIOMs()
	time.Sleep(metricsInterval)
	t1, i1, _ := readCPUTimes()
	io1, _ := readDiskIOMs()

	if err0 == nil && t1 > t0 {
		cpu = float32(1-float64(i1-i0)/float64(t1-t0)) * 100
	}
	memory, _ = readMemPercent()
	disk, _ = readDiskPercent()
	diskIO = float32((io1 - io0) / (metricsInterval.Seconds() * 1000) * 100)
	return
}
