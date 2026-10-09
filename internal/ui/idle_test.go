package ui

import (
	"bytes"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"filelist/internal/comm"
)

func TestNextKeyIdleHangup(t *testing.T) {
	client, door := net.Pipe()
	defer client.Close()
	defer door.Close()

	var out bytes.Buffer
	hung := false
	a := &App{
		Out:    &out,
		Keys:   comm.NewKeyboard(door),
		Idle:   50 * time.Millisecond,
		Hangup: func() { hung = true },
	}
	_, err := a.nextKey()
	if !errors.Is(err, errIdle) {
		t.Fatalf("err = %v, want errIdle", err)
	}
	if !hung {
		t.Fatal("Hangup not called")
	}
	if !strings.Contains(out.String(), "Inactivity Timeout") {
		t.Fatalf("no timeout message: %q", out.String())
	}
	if _, err := a.nextKey(); !errors.Is(err, errIdle) {
		t.Fatalf("second nextKey err = %v, want errIdle", err)
	}
}

func TestNextKeyResetsOnKey(t *testing.T) {
	client, door := net.Pipe()
	defer client.Close()
	defer door.Close()

	a := &App{Out: &bytes.Buffer{}, Keys: comm.NewKeyboard(door), Idle: time.Second}
	go func() {
		time.Sleep(100 * time.Millisecond)
		_, _ = client.Write([]byte("x"))
	}()
	ev, err := a.nextKey()
	if err != nil || !comm.KeyCharEquals(ev, 'X') {
		t.Fatalf("ev=%+v err=%v", ev, err)
	}
}
