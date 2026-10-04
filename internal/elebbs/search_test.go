package elebbs

import (
	"testing"
	"time"
)

func TestPackedTwoDigitYear(t *testing.T) {
	v := uint32(9 | (9 << 5) | (26 << 9))
	got := FileStamp(v)
	if got.Year() != 2026 || got.Month() != 9 || got.Day() != 9 {
		t.Fatalf("2-digit year 26: got %v, want 2026-09-09", got)
	}
	if FormatDate(v) != "09-09-26" {
		t.Fatalf("FormatDate=%q want 09-09-26", FormatDate(v))
	}
}

func TestPackedNinetiesYear(t *testing.T) {
	v := uint32(8 | (4 << 5) | (95 << 9))
	got := FileStamp(v)
	if got.Year() != 1995 || got.Month() != 4 || got.Day() != 8 {
		t.Fatalf("04-08-95: got %v, want 1995-04-08", got)
	}
	if FormatDate(v) != "04-08-95" {
		t.Fatalf("FormatDate=%q want 04-08-95", FormatDate(v))
	}
	e := FileEntry{Hdr: FilesHdr{FileDate: v}}
	if MatchDays(e, 5) {
		t.Fatal("1995 file must not match a 5-day new scan")
	}
}

func TestPackedDOSYearSince1980(t *testing.T) {
	now := time.Now()
	ybits := now.Year() - 1980
	v := uint32(now.Day() | (int(now.Month()) << 5) | (ybits << 9))
	got := FileStamp(v)
	if got.Year() != now.Year() || got.Month() != now.Month() || got.Day() != now.Day() {
		t.Fatalf("DOS year-since-1980: got %v, want %s", got, now.Format("2006-01-02"))
	}
	e := FileEntry{Hdr: FilesHdr{FileDate: v}}
	if !MatchDays(e, 5) {
		t.Fatal("today as DOS packed date should match a 5-day scan")
	}
}

func TestMatchDaysUsesFileDateNotUpload(t *testing.T) {
	old := uint32(8 | (4 << 5) | (95 << 9))
	recent := uint32(time.Now().Unix())
	e := FileEntry{Hdr: FilesHdr{FileDate: old, UploadDate: recent}}
	if MatchDays(e, 5) {
		t.Fatal("a 1995 file date must not match just because upload date is recent")
	}
}
