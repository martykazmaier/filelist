package comm

import (
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	solSocket      = 0xFFFF
	soRcvTimeo     = 0x1006
	soSndTimeo     = 0x1005
	soError        = 0x1007
	ipprotoTCP     = 6
	tcpNoDelay     = 1
	fionbio        = 0x8004667E
	fionread       = 0x4004667F
	invalidSocket  = ^uintptr(0)
	socketError    = ^uintptr(0)
	wsaIoPending   = 997
	wsaWouldBlock  = 10035
	wsaTimedOut    = 10060
	wsaConnReset   = 10054
	wsaConnAbort   = 10053
	wsaNetReset    = 10052
	wsaNotConn     = 10057
	wsaShutdown    = 10058
	wsaConnRefused = 10061
	wsaNetDown     = 10050
	wsaNetUnreach  = 10051
	// 24h — never use SO_RCVTIMEO 0; some stacks treat 0 as an immediate timeout.
	blockTimeoutMS = 86400000
)

var (
	ws2_32          = windows.NewLazySystemDLL("ws2_32.dll")
	procIoctlsocket = ws2_32.NewProc("ioctlsocket")
	procRecv        = ws2_32.NewProc("recv")
	procSend        = ws2_32.NewProc("send")
)

func FromDoorHandle(handle uintptr) (Stream, error) {
	if handle == 0 || handle == invalidSocket {
		return nil, fmt.Errorf("invalid socket handle")
	}
	var wsa windows.WSAData
	if err := windows.WSAStartup(uint32(0x0202), &wsa); err != nil {
		return nil, fmt.Errorf("WSAStartup: %w", err)
	}
	h := windows.Handle(handle)
	_ = windows.SetsockoptInt(h, ipprotoTCP, tcpNoDelay, 1)
	var nb uint32
	_, _, _ = procIoctlsocket.Call(uintptr(h), fionbio, uintptr(unsafe.Pointer(&nb)))
	// EleBBS often leaves a short receive timeout on the inherited socket.
	_ = windows.SetsockoptInt(h, solSocket, soRcvTimeo, blockTimeoutMS)
	_ = windows.SetsockoptInt(h, solSocket, soSndTimeo, blockTimeoutMS)
	return &doorSock{h: h}, nil
}

type doorSock struct {
	h         windows.Handle
	mu        sync.Mutex
	rdeadline time.Time
	wdeadline time.Time
}

func (s *doorSock) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	s.mu.Lock()
	deadline := s.rdeadline
	s.mu.Unlock()

	for {
		if timedOut(deadline) {
			return 0, os.ErrDeadlineExceeded
		}
		if err := socketDead(s.h); err != nil {
			return 0, err
		}
		var avail uint32
		r1, _, _ := procIoctlsocket.Call(uintptr(s.h), fionread, uintptr(unsafe.Pointer(&avail)))
		if r1 == 0 && avail > 0 {
			n, err := recv(s.h, p, 0)
			if n > 0 {
				return n, nil
			}
			if isWait(err) || isTimeout(err) {
				sleepUntil(deadline, 10*time.Millisecond)
				continue
			}
			if isReset(err) {
				return 0, io.EOF
			}
			if err != nil {
				sleepUntil(deadline, 15*time.Millisecond)
				continue
			}
			// n==0 with FIONREAD>0 is often a false close on overlapped sockets.
			sleepUntil(deadline, 15*time.Millisecond)
			continue
		}
		sleepUntil(deadline, 20*time.Millisecond)
	}
}

func (s *doorSock) Write(p []byte) (int, error) {
	sent := 0
	s.mu.Lock()
	deadline := s.wdeadline
	s.mu.Unlock()
	for sent < len(p) {
		if timedOut(deadline) {
			return sent, os.ErrDeadlineExceeded
		}
		n, err := send(s.h, p[sent:], 0)
		sent += n
		if n > 0 && err == nil {
			continue
		}
		if isWait(err) || isTimeout(err) {
			sleepUntil(deadline, 15*time.Millisecond)
			continue
		}
		if isReset(err) {
			return sent, err
		}
		if err != nil {
			return sent, err
		}
		sleepUntil(deadline, 15*time.Millisecond)
	}
	return sent, nil
}

func (s *doorSock) Close() error {
	return nil
}

func (s *doorSock) SetReadDeadline(t time.Time) error {
	s.mu.Lock()
	s.rdeadline = t
	s.mu.Unlock()
	ms := blockTimeoutMS
	if !t.IsZero() {
		d := time.Until(t)
		if d < 0 {
			d = 0
		}
		ms = int(d / time.Millisecond)
		if ms <= 0 {
			ms = 1
		}
	}
	return windows.SetsockoptInt(s.h, solSocket, soRcvTimeo, ms)
}

func (s *doorSock) SetWriteDeadline(t time.Time) error {
	s.mu.Lock()
	s.wdeadline = t
	s.mu.Unlock()
	return nil
}

func recv(h windows.Handle, p []byte, flags int) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	r0, _, e := procRecv.Call(uintptr(h), uintptr(unsafe.Pointer(&p[0])), uintptr(len(p)), uintptr(flags))
	n := int32(r0)
	if n == -1 {
		if e == syscall.Errno(0) {
			return 0, syscall.EINVAL
		}
		return 0, e
	}
	return int(n), nil
}

func send(h windows.Handle, p []byte, flags int) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	r0, _, e := procSend.Call(uintptr(h), uintptr(unsafe.Pointer(&p[0])), uintptr(len(p)), uintptr(flags))
	n := int32(r0)
	if n == -1 {
		if e == syscall.Errno(0) {
			return 0, syscall.EINVAL
		}
		return 0, e
	}
	return int(n), nil
}

func socketDead(h windows.Handle) error {
	v, err := windows.GetsockoptInt(h, solSocket, soError)
	if err != nil {
		return nil
	}
	if v == 0 {
		return nil
	}
	errno := syscall.Errno(v)
	if isReset(errno) {
		return io.EOF
	}
	return nil
}

func timedOut(deadline time.Time) bool {
	return !deadline.IsZero() && !time.Now().Before(deadline)
}

func sleepUntil(deadline time.Time, d time.Duration) {
	if !deadline.IsZero() {
		left := time.Until(deadline)
		if left <= 0 {
			return
		}
		if left < d {
			d = left
		}
	}
	time.Sleep(d)
}

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errno, ok := err.(syscall.Errno); ok {
		return errno == wsaTimedOut || errno == windows.WSAETIMEDOUT
	}
	return false
}

func isWait(err error) bool {
	if err == nil {
		return false
	}
	errno, ok := err.(syscall.Errno)
	if !ok {
		return false
	}
	switch errno {
	case wsaWouldBlock, wsaIoPending, windows.WSAEINTR:
		return true
	}
	return false
}

func isReset(err error) bool {
	if err == nil {
		return false
	}
	errno, ok := err.(syscall.Errno)
	if !ok {
		return false
	}
	switch errno {
	case wsaConnReset, wsaConnAbort, wsaNetReset, wsaNotConn, wsaShutdown, wsaConnRefused, wsaNetDown, wsaNetUnreach:
		return true
	}
	return false
}

func PrepareHandoff(sock uintptr) {
	if sock == 0 {
		return
	}
	h := windows.Handle(sock)
	_ = windows.SetHandleInformation(h, windows.HANDLE_FLAG_INHERIT, windows.HANDLE_FLAG_INHERIT)
	var nb uint32
	_, _, _ = procIoctlsocket.Call(uintptr(h), fionbio, uintptr(unsafe.Pointer(&nb)))
	_ = windows.SetsockoptInt(h, solSocket, soRcvTimeo, 0)
	_ = windows.SetsockoptInt(h, solSocket, soSndTimeo, 0)
}

func RestoreHandoff(sock uintptr) {
	if sock == 0 {
		return
	}
	h := windows.Handle(sock)
	var nb uint32
	_, _, _ = procIoctlsocket.Call(uintptr(h), fionbio, uintptr(unsafe.Pointer(&nb)))
	_ = windows.SetsockoptInt(h, solSocket, soRcvTimeo, blockTimeoutMS)
	_ = windows.SetsockoptInt(h, solSocket, soSndTimeo, blockTimeoutMS)
}

func RunInheritedConsole(cmdLine, dir string, sock uintptr) error {
	PrepareHandoff(sock)
	defer RestoreHandoff(sock)

	comspec := os.Getenv("ComSpec")
	if comspec == "" {
		root := os.Getenv("SystemRoot")
		if root == "" {
			root = `C:\Windows`
		}
		comspec = root + `\System32\cmd.exe`
	}
	full := comspec + " /C " + cmdLine

	var si windows.StartupInfo
	si.Cb = uint32(unsafe.Sizeof(si))
	si.Flags = windows.STARTF_USESHOWWINDOW
	si.ShowWindow = windows.SW_SHOWNORMAL

	var pi windows.ProcessInformation
	var dirp *uint16
	if dir != "" {
		dirp = windows.StringToUTF16Ptr(dir)
	}
	err := windows.CreateProcess(
		nil,
		windows.StringToUTF16Ptr(full),
		nil,
		nil,
		true,
		windows.CREATE_NEW_CONSOLE,
		nil,
		dirp,
		&si,
		&pi,
	)
	if err != nil {
		return fmt.Errorf("view: %w", err)
	}
	defer windows.CloseHandle(pi.Thread)
	defer windows.CloseHandle(pi.Process)
	s, err := windows.WaitForSingleObject(pi.Process, windows.INFINITE)
	if err != nil {
		return err
	}
	if s != windows.WAIT_OBJECT_0 {
		return fmt.Errorf("view: wait failed")
	}
	return nil
}

func HideConsole() {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	user32 := windows.NewLazySystemDLL("user32.dll")
	getConsoleWindow := kernel32.NewProc("GetConsoleWindow")
	showWindow := user32.NewProc("ShowWindow")
	hwnd, _, _ := getConsoleWindow.Call()
	if hwnd != 0 {
		showWindow.Call(hwnd, 0)
	}
}

func EnableVT() {
	h, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	if err != nil {
		return
	}
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return
	}
	mode |= windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING | windows.ENABLE_PROCESSED_OUTPUT
	_ = windows.SetConsoleMode(h, mode)

	in, err := windows.GetStdHandle(windows.STD_INPUT_HANDLE)
	if err != nil {
		return
	}
	if err := windows.GetConsoleMode(in, &mode); err != nil {
		return
	}
	mode &^= windows.ENABLE_ECHO_INPUT | windows.ENABLE_LINE_INPUT | windows.ENABLE_PROCESSED_INPUT
	mode |= windows.ENABLE_VIRTUAL_TERMINAL_INPUT
	_ = windows.SetConsoleMode(in, mode)
}

func RestoreConsole() {
	in, err := windows.GetStdHandle(windows.STD_INPUT_HANDLE)
	if err != nil {
		return
	}
	_ = windows.SetConsoleMode(in, windows.ENABLE_ECHO_INPUT|windows.ENABLE_LINE_INPUT|windows.ENABLE_PROCESSED_INPUT|windows.ENABLE_EXTENDED_FLAGS)
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
	if handle == 0 || handle == invalidSocket {
		return
	}
	_ = windows.Shutdown(windows.Handle(handle), windows.SHUT_RDWR)
}
