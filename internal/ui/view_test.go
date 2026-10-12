package ui

import (
	"strings"
	"testing"
)

func TestExpandViewQuotesLFN(t *testing.T) {
	path := `c:\ele\files\anime\mellow\Armor Hunter Mellowlink - 01 (BDRip 1440x1080p).mkv`
	tmpl := `c:\ele\viewer.exe -Dc:\ele\node*N -B57600 -T15 @`
	want := `c:\ele\viewer.exe -Dc:\ele\node1 -B57600 -T15 "` + path + `"`
	if got := expandView(tmpl, path, 1, 596); got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
	if got := expandView(`view.exe "@"`, path, 1, 0); got != `view.exe "`+path+`"` {
		t.Fatalf("already-quoted @ double quoted: %s", got)
	}
	if got := expandView(`view.exe @`, `C:\ELE\SYS.ZIP`, 1, 0); got != `view.exe C:\ELE\SYS.ZIP` {
		t.Fatalf("plain path should stay unquoted: %s", got)
	}
	if got := expandView(`v.exe -H*W`, "x", 1, 812); got != `v.exe -H812` {
		t.Fatalf("*W: %s", got)
	}
}

func TestIsViewer(t *testing.T) {
	for cmd, want := range map[string]bool{
		`c:\ele\viewer.exe -Dc:\ele\node1 @`:  true,
		`"C:\Program Files\Ele\Viewer.EXE" @`: true,
		`c:\ele\other.exe @`:                  false,
		`viewer`:                              true,
	} {
		if got := isViewer(cmd); got != want {
			t.Errorf("isViewer(%q) = %v, want %v", cmd, got, want)
		}
	}
}

func TestWrapPlain(t *testing.T) {
	long := "viewer: socket handle 596 does not exist in this process, and it could not be copied from a parent process"
	got := wrapPlain([]string{long}, 40)
	if len(got) < 3 {
		t.Fatalf("expected wrapping, got %q", got)
	}
	for _, l := range got {
		if len(l) > 40 {
			t.Fatalf("line too long (%d): %q", len(l), l)
		}
	}
	if strings.Join(got, " ") != long {
		t.Fatalf("text lost in wrap: %q", got)
	}

	ansi := "\x1b[1;31m" + strings.Repeat("x", 60) + "\x1b[0m"
	if got := wrapPlain([]string{ansi}, 40); len(got) != 1 || got[0] != ansi {
		t.Fatalf("ANSI line must stay intact, got %q", got)
	}
}
