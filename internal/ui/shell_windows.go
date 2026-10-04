//go:build windows

package ui

import (
	"os/exec"
	"syscall"
)

func shellCommand(cmdLine string) *exec.Cmd {
	cmd := exec.Command("cmd.exe", "/C", cmdLine)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}
