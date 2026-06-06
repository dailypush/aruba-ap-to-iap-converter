//go:build windows

package platform

import (
	"os"
	"os/exec"
	"syscall"
)

func startDetached(cmd *exec.Cmd, logPath string) error {
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	cmd.Stdout = f
	cmd.Stderr = f
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
	err = cmd.Start()
	_ = f.Close()
	return err
}
