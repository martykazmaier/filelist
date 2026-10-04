//go:build !windows

package ui

import "os/exec"

func shellCommand(cmdLine string) *exec.Cmd {
	return exec.Command("/bin/sh", "-c", cmdLine)
}
