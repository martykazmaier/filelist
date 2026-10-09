package elebbs

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadUserTimeOut(t *testing.T) {
	dir := t.TempDir()
	if got := ReadUserTimeOut(dir); got != 0 {
		t.Fatalf("missing config.ra: got %v, want 0", got)
	}
	cfg := make([]byte, 6331)
	binary.LittleEndian.PutUint16(cfg[configUserTimeOutOff:], 900)
	binary.LittleEndian.PutUint16(cfg[configUserTimeOutOff+2:], 30)
	if err := os.WriteFile(filepath.Join(dir, "CONFIG.RA"), cfg, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ReadUserTimeOut(dir); got != 15*time.Minute {
		t.Fatalf("got %v, want 15m", got)
	}
}
