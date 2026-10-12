//go:build windows

package ui

import (
	"os/exec"
	"syscall"
)

// shellCommand runs cmdLine through cmd.exe. os/exec only hands stdio to the
// child, so the door socket is listed explicitly for socket doors (viewer.exe
// reads it from door32.sys and needs the same handle number).
func shellCommand(cmdLine string, socks ...uintptr) *exec.Cmd {
	cmd := exec.Command("cmd.exe")
	// Raw command line: os/exec would backslash-escape quotes, which cmd.exe
	// does not understand.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CmdLine: `cmd.exe /C ` + cmdLine}
	for _, s := range socks {
		if s != 0 && s != ^uintptr(0) {
			cmd.SysProcAttr.AdditionalInheritedHandles = append(cmd.SysProcAttr.AdditionalInheritedHandles, syscall.Handle(s))
		}
	}
	return cmd
}
