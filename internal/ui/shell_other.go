//go:build !windows

package ui

import "os/exec"

// shellCommand runs cmdLine through /bin/sh. The door descriptor from
// door32.sys is not close-on-exec, so the child inherits it as-is.
func shellCommand(cmdLine string, socks ...uintptr) *exec.Cmd {
	return exec.Command("/bin/sh", "-c", cmdLine)
}
