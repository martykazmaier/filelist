//go:build linux

package comm

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"

	"golang.org/x/sys/unix"
)

// FromDoorHandle wraps the socket descriptor named in door32.sys line 2.
// net.FileConn duplicates it, so the BBS keeps its own copy open.
func FromDoorHandle(handle uintptr) (Stream, error) {
	if handle == 0 || handle == ^uintptr(0) {
		return nil, fmt.Errorf("invalid socket handle")
	}
	f := os.NewFile(handle, "door32")
	if f == nil {
		return nil, fmt.Errorf("invalid socket descriptor %d", handle)
	}
	c, err := net.FileConn(f)
	if err != nil {
		return nil, fmt.Errorf("socket %d: %w", handle, err)
	}
	return WrapRaw(c), nil
}

func PrepareHandoff(uintptr) {}

func RestoreHandoff(uintptr) {}

func RunInheritedConsole(cmdLine, dir string, sock uintptr) error {
	cmd := exec.Command("/bin/sh", "-c", cmdLine)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("view: %w", err)
	}
	return nil
}

func HideConsole() {}

var savedTermios *unix.Termios

func EnableVT() {
	fd := int(os.Stdin.Fd())
	t, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return
	}
	orig := *t
	savedTermios = &orig
	t.Lflag &^= unix.ECHO | unix.ICANON | unix.ISIG | unix.IEXTEN
	t.Iflag &^= unix.ICRNL | unix.IXON
	t.Cc[unix.VMIN] = 1
	t.Cc[unix.VTIME] = 0
	_ = unix.IoctlSetTermios(fd, unix.TCSETS, t)
}

func RestoreConsole() {
	if savedTermios == nil {
		return
	}
	_ = unix.IoctlSetTermios(int(os.Stdin.Fd()), unix.TCSETS, savedTermios)
}

type stdioStream struct {
	in  *os.File
	out *os.File
}

func LocalStdio() Stream {
	EnableVT()
	return &stdioStream{in: os.Stdin, out: os.Stdout}
}

func (s *stdioStream) Read(p []byte) (int, error)         { return s.in.Read(p) }
func (s *stdioStream) Write(p []byte) (int, error)        { return s.out.Write(p) }
func (s *stdioStream) Close() error                       { RestoreConsole(); return nil }
func (s *stdioStream) SetReadDeadline(t time.Time) error  { return nil }
func (s *stdioStream) SetWriteDeadline(t time.Time) error { return nil }

// Hangup drops the caller by shutting down the inherited socket in both
// directions; the BBS then sees the connection close.
func Hangup(handle uintptr) {
	if handle == 0 || handle == ^uintptr(0) {
		return
	}
	_ = unix.Shutdown(int(handle), unix.SHUT_RDWR)
}
