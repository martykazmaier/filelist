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
	doorFile = f
	return WrapRaw(c), nil
}

// doorFile holds the original door32.sys descriptor so its finalizer never
// closes it; socket doors started from the viewer expect that fd number.
var doorFile *os.File

func SocketDiag(h uintptr) string { return fmt.Sprintf("fd %d", h) }

// DupForChild returns sock itself: the door fd is inherited unchanged.
func DupForChild(sock uintptr) (uintptr, func()) { return sock, func() {} }

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

// Hangup drops the caller with an abortive close so the BBS sees carrier loss
// instead of resuming the session.
func Hangup(handle uintptr) {
	if handle == 0 || handle == ^uintptr(0) {
		return
	}
	fd := int(handle)
	_ = unix.SetsockoptLinger(fd, unix.SOL_SOCKET, unix.SO_LINGER, &unix.Linger{Onoff: 1, Linger: 0})
	_ = unix.Shutdown(fd, unix.SHUT_RDWR)
	if doorFile != nil {
		_ = doorFile.Close()
		doorFile = nil
	}
}
