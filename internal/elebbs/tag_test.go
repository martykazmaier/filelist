package elebbs

import (
	"strings"
	"testing"
)

func TestIsTaggedOnlySelectedFile(t *testing.T) {
	area := FilesArea{AreaNum: 3}
	a := FileEntry{ShortName: "ONE.ZIP", DisplayName: "one-long.zip", AreaNum: 3, Hdr: FilesHdr{Name: "ONE.ZIP", RecordNum: 1}}
	b := FileEntry{ShortName: "TWO.ZIP", DisplayName: "two-long.zip", AreaNum: 3, Hdr: FilesHdr{Name: "TWO.ZIP", RecordNum: 2}}
	c := FileEntry{ShortName: "TRE.ZIP", DisplayName: "tre-long.zip", AreaNum: 3, Hdr: FilesHdr{Name: "TRE.ZIP", RecordNum: 3}}

	tags, on := ToggleTag(nil, a, area)
	if !on || len(tags) != 1 {
		t.Fatalf("tag one: on=%v n=%d", on, len(tags))
	}
	if !IsTagged(tags, a, 3) {
		t.Fatal("selected file should be tagged")
	}
	if IsTagged(tags, b, 3) || IsTagged(tags, c, 3) {
		t.Fatal("other files must not show as tagged")
	}

	tags, on = ToggleTag(tags, c, area)
	if !on || len(tags) != 2 {
		t.Fatalf("tag third: on=%v n=%d", on, len(tags))
	}
	if !IsTagged(tags, a, 3) || !IsTagged(tags, c, 3) {
		t.Fatal("both tagged files should show a check")
	}
	if IsTagged(tags, b, 3) {
		t.Fatal("untagged file must not show a check")
	}
}

func TestIsTaggedSameShortNameDifferentRec(t *testing.T) {
	area := FilesArea{AreaNum: 3}
	a := FileEntry{ShortName: "SAME.ZIP", DisplayName: "one.long", AreaNum: 3, Hdr: FilesHdr{Name: "SAME.ZIP", RecordNum: 1}}
	b := FileEntry{ShortName: "SAME.ZIP", DisplayName: "two.long", AreaNum: 3, Hdr: FilesHdr{Name: "SAME.ZIP", RecordNum: 2}}

	tags, on := ToggleTag(nil, a, area)
	if !on {
		t.Fatal("tag a")
	}
	if !IsTagged(tags, a, 3) {
		t.Fatal("a should be tagged")
	}
	if IsTagged(tags, b, 3) {
		t.Fatal("same 8.3 name in another record must not show a check")
	}
}

func TestIsTaggedRecordZeroDifferentNames(t *testing.T) {
	area := FilesArea{AreaNum: 3}
	a := FileEntry{ShortName: "ONE.ZIP", AreaNum: 3, Hdr: FilesHdr{Name: "ONE.ZIP", RecordNum: 0}}
	b := FileEntry{ShortName: "TWO.ZIP", AreaNum: 3, Hdr: FilesHdr{Name: "TWO.ZIP", RecordNum: 0}}

	tags, _ := ToggleTag(nil, a, area)
	if !IsTagged(tags, a, 3) {
		t.Fatal("selected file should be tagged")
	}
	if IsTagged(tags, b, 3) {
		t.Fatal("record 0 must not treat every file as tagged")
	}
}

func TestIsTaggedEmptyNameDoesNotMatchAll(t *testing.T) {
	a := FileEntry{ShortName: "ONE.ZIP", AreaNum: 3, Hdr: FilesHdr{RecordNum: 1}}
	b := FileEntry{ShortName: "TWO.ZIP", AreaNum: 3, Hdr: FilesHdr{RecordNum: 2}}
	tags := []TagRecord{{AreaNum: 3, RecordNum: 0, Name: ""}}
	if IsTagged(tags, a, 3) || IsTagged(tags, b, 3) {
		t.Fatal("empty tag name must not paint checks on every file")
	}
}

func TestToggleTagEleBBSStyle(t *testing.T) {
	area := FilesArea{AreaNum: 3}
	e := FileEntry{
		ShortName:   "README.TXT",
		DisplayName: "Read Me Please.TXT",
		AreaNum:     3,
		Hdr:         FilesHdr{Name: "README.TXT", RecordNum: 1},
	}
	tags, on := ToggleTag(nil, e, area)
	if !on || len(tags) != 1 {
		t.Fatalf("tag: on=%v n=%d", on, len(tags))
	}
	if tags[0].Name != "README.TXT" {
		t.Fatalf("tag Name %q, want FDB name README.TXT", tags[0].Name)
	}
	if tags[0].Lfn != "Read Me Please.TXT" {
		t.Fatalf("tag Lfn %q, want long name", tags[0].Lfn)
	}
	got := parseTag(encodeTag(tags[0]))
	if got.Name != "README.TXT" {
		t.Fatalf("stored name %q, want README.TXT (EleBBS String[12])", got.Name)
	}
	if got.RecordNum != 1 || got.AreaNum != 3 {
		t.Fatalf("stored rec/area %d/%d", got.RecordNum, got.AreaNum)
	}
	if !IsTagged(tags, e, 3) {
		t.Fatal("tagged LFN must still display as tagged")
	}

	long := FileEntry{
		ShortName:   "FILE.MKV",
		DisplayName: "Armor Hunter Mellowlink - 01 - Wilderness.mkv",
		AreaNum:     3,
		Hdr:         FilesHdr{Name: "FILE.MKV", RecordNum: 2},
	}
	putPascal(long.Hdr.NameRaw[:], "FILE.MKV")
	tags, on = ToggleTag(nil, long, area)
	if !on {
		t.Fatal("tag long")
	}
	got = parseTag(encodeTag(tags[0]))
	if got.Name != "FILE.MKV" {
		t.Fatalf("stored name %q, want FILE.MKV not LFN prefix", got.Name)
	}
	if tags[0].Lfn != long.DisplayName {
		t.Fatalf("in-memory Lfn %q", tags[0].Lfn)
	}
	if strings.Contains(got.Name, "Armor") || strings.Contains(got.Name, "~") {
		t.Fatalf("stored %q, want EleBBS FDB name", got.Name)
	}
	if !IsTagged(tags, long, 3) {
		t.Fatal("tagged long name must still display as tagged")
	}
}
