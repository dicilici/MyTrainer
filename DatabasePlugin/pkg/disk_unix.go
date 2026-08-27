//go:build !windows

package pkg

import (
	"os"
	"syscall"
)

func readDiskPercent() (float32, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs("/", &st); err != nil {
		return 0, err
	}
	if st.Blocks == 0 {
		return 0, os.ErrInvalid
	}
	return float32(1-float64(st.Bavail)/float64(st.Blocks)) * 100, nil
}
