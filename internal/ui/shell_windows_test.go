//go:build windows

package ui

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"testing"

	"golang.org/x/sys/windows"

	"filelist/internal/comm"
)

// The door socket arrives the way EleBBS DupForDoor makes it: a
// non-overlapped WSASocket from WSADuplicateSocket, inherited into the door.
// The door then starts viewer.exe through cmd.exe. These tests run that same
// two-hop chain with the test binary standing in for the door and viewer.

const (
	envRole = "FILELIST_TEST_ROLE"
	envSock = "FILELIST_TEST_SOCK"
	envMode = "FILELIST_TEST_MODE"
)

func TestMain(m *testing.M) {
	switch os.Getenv(envRole) {
	case "door":
		os.Exit(runDoorRole())
	case "viewer":
		os.Exit(runViewerRole())
	}
	os.Exit(m.Run())
}

func envHandle(name string) uintptr {
	n, _ := strconv.ParseUint(os.Getenv(name), 10, 64)
	return uintptr(n)
}

func sockUsable(h uintptr) error {
	var d windows.WSAData
	_ = windows.WSAStartup(0x0202, &d)
	if _, err := windows.GetsockoptInt(windows.Handle(h), 0xFFFF, 0x1008); err != nil {
		return fmt.Errorf("SO_TYPE: %w", err)
	}
	if _, err := windows.Getpeername(windows.Handle(h)); err != nil {
		return fmt.Errorf("getpeername: %w", err)
	}
	return nil
}

// runDoorRole is filelist32.exe: it holds the inherited door socket and
// launches the "viewer" through shellCommand.
func runDoorRole() int {
	sock := envHandle(envSock)
	if err := sockUsable(sock); err != nil {
		fmt.Printf("door: inherited socket %d unusable: %v\n", sock, err)
		return 4
	}
	child, release := sock, func() {}
	if os.Getenv(envMode) == "dup" {
		child, release = comm.DupForChild(sock)
	}
	defer release()
	cmd := shellCommand(fmt.Sprintf(`"%s"`, os.Args[0]), child, sock)
	cmd.Env = append(os.Environ(), envRole+"=viewer", envSock+"="+strconv.FormatUint(uint64(child), 10))
	comm.PrepareHandoff(sock)
	out, err := cmd.CombinedOutput()
	comm.RestoreHandoff(sock)
	fmt.Printf("%s", out)
	if err != nil {
		return 5
	}
	return 0
}

func runViewerRole() int {
	h := envHandle(envSock)
	if err := sockUsable(h); err != nil {
		fmt.Printf("viewer: socket %d unusable: %v\n", h, err)
		return 3
	}
	fmt.Printf("viewer: socket %d ok\n", h)
	return 0
}

// eleDoorSocket builds a socket the way EleBBS DupForDoor does.
func eleDoorSocket(t *testing.T) uintptr {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		if c, err := ln.Accept(); err == nil {
			defer c.Close()
			select {}
		}
	}()
	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	f, err := c.(*net.TCPConn).File()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	var info windows.WSAProtocolInfo
	if err := windows.WSADuplicateSocket(windows.Handle(f.Fd()), windows.GetCurrentProcessId(), &info); err != nil {
		t.Fatal(err)
	}
	nt, err := windows.WSASocket(info.AddressFamily, info.SocketType, info.Protocol, &info, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { windows.Closesocket(nt) })
	_ = windows.SetHandleInformation(nt, windows.HANDLE_FLAG_INHERIT, windows.HANDLE_FLAG_INHERIT)
	return uintptr(nt)
}

func runChain(t *testing.T, mode string) ([]byte, error) {
	sock := eleDoorSocket(t)
	cmd := shellCommand(fmt.Sprintf(`"%s"`, os.Args[0]), sock)
	cmd.Env = append(os.Environ(), envRole+"=door", envMode+"="+mode, envSock+"="+strconv.FormatUint(uint64(sock), 10))
	return cmd.CombinedOutput()
}

func TestViewerGetsSocketThroughDoor(t *testing.T) {
	out, err := runChain(t, "dup")
	t.Logf("%s", out)
	if err != nil {
		t.Fatalf("chain failed: %v", err)
	}
}
