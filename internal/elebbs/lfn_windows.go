//go:build windows

package elebbs

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func diskLongName(dir, prefix string, size uint32) string {
	dir = strings.TrimRight(strings.TrimSpace(dir), `\`)
	prefix = strings.TrimSpace(prefix)
	if dir == "" || prefix == "" {
		return ""
	}
	pattern := filepath.Join(dir, prefix+"*")
	p, err := windows.UTF16PtrFromString(pattern)
	if err != nil {
		return ""
	}
	var fd windows.Win32finddata
	h, err := windows.FindFirstFile(p, &fd)
	if err != nil {
		return ""
	}
	defer windows.FindClose(h)

	var matches []string
	var sized string
	for {
		name := windows.UTF16ToString(fd.FileName[:])
		attr := fd.FileAttributes
		if name != "." && name != ".." && attr&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
			matches = append(matches, name)
			sz := uint64(fd.FileSizeHigh)<<32 | uint64(fd.FileSizeLow)
			if size > 0 && uint32(sz) == size {
				sized = name
			}
		}
		if err := windows.FindNextFile(h, &fd); err != nil {
			break
		}
		if len(matches) > 32 {
			break
		}
	}
	if sized != "" {
		return sized
	}
	if len(matches) == 1 {
		return matches[0]
	}
	return ""
}
