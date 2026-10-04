package elebbs

import "testing"

func TestReadTxtNameCString(t *testing.T) {
	b := append([]byte("Armor Hunter Mellowlink - 01.mkv"), 0)
	got := readTxtName(b)
	if got != "Armor Hunter Mellowlink - 01.mkv" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveDisplayNamePrefersLongerLFN(t *testing.T) {
	h := FilesHdr{Name: "Armor Hunter", Size: 123, LfnPtr: 1}
	txt := append([]byte{0}, append([]byte("Armor Hunter Mellowlink - 01.mkv"), 0)...)
	got := resolveDisplayName("", h, txt)
	if got != "Armor Hunter Mellowlink - 01.mkv" {
		t.Fatalf("got %q", got)
	}
}
