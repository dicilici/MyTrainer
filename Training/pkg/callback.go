package pkg

import (
	"golang.org/x/sys/windows"
	"os"
	"os/exec"
	"syscall"
)

func CallBack(path string) {
	cmd := exec.Command(path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NO_WINDOW,
	}
	cmd.Start()
}
