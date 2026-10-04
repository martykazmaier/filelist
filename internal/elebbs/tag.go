package elebbs

import (
	"os"
	"path/filepath"
	"strings"
)

func TagListPath(nodePath string) string {
	if nodePath == "" {
		nodePath = "."
	}
	return filepath.Join(nodePath, "taglist.ra")
}

func LoadTagList(nodePath string) ([]TagRecord, error) {
	p := TagListPath(nodePath)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	n := len(data) / TagRecSize
	out := make([]TagRecord, 0, n)
	for i := 0; i < n; i++ {
		t := parseTag(data[i*TagRecSize : (i+1)*TagRecSize])
		if t.Name == "" && t.NameRaw[0] == 0 {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

func SaveTagList(nodePath string, tags []TagRecord) error {
	p := TagListPath(nodePath)
	if len(tags) == 0 {
		_ = os.Remove(p)
		f, err := os.OpenFile(p, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o666)
		if err != nil {
			return err
		}
		return f.Close()
	}
	buf := make([]byte, 0, len(tags)*TagRecSize)
	for _, t := range tags {
		buf = append(buf, encodeTag(t)...)
	}
	return os.WriteFile(p, buf, 0o666)
}

func SameFile(a TagRecord, name string, area uint16, rec uint16) bool {
	return tagMatches(a, FileEntry{ShortName: name, AreaNum: area, Hdr: FilesHdr{RecordNum: rec}}, area)
}

func tagName(t TagRecord) string {
	if s := normalizeName(t.Name); s != "" {
		return s
	}
	return normalizeName(pascalString(t.NameRaw[:]))
}

func normalizeName(s string) string {
	s = strings.TrimSpace(strings.Trim(s, "\x00"))
	return strings.ToUpper(s)
}

func entryArea(e FileEntry, area uint16) uint16 {
	if e.AreaNum != 0 {
		return e.AreaNum
	}
	return area
}

func nameMatches(t TagRecord, e FileEntry) bool {
	tnames := []string{tagName(t), normalizeName(t.Lfn)}
	for _, tname := range tnames {
		if tname == "" {
			continue
		}
		for _, s := range []string{tagFileName(e), e.DisplayName, e.ShortName, e.Hdr.Name} {
			n := normalizeName(s)
			if n == "" {
				continue
			}
			if tname == n || strings.HasPrefix(n, tname) || strings.HasPrefix(tname, n) {
				return true
			}
		}
	}
	return false
}

func tagMatches(t TagRecord, e FileEntry, area uint16) bool {
	ea := entryArea(e, area)
	if t.AreaNum != 0 && ea != 0 && t.AreaNum != ea {
		return false
	}
	if t.RecordNum != 0 && e.Hdr.RecordNum != 0 {
		return t.RecordNum == e.Hdr.RecordNum
	}
	return nameMatches(t, e)
}

func tagFileName(e FileEntry) string {
	name := strings.TrimSpace(strings.Trim(e.DisplayName, "\x00"))
	if name == "" {
		name = strings.TrimSpace(e.ShortName)
	}
	if name == "" {
		name = strings.TrimSpace(e.Hdr.Name)
	}
	name = strings.ReplaceAll(name, "/", `\`)
	if i := strings.LastIndexByte(name, '\\'); i >= 0 {
		name = name[i+1:]
	}
	return name
}

func ToggleTag(tags []TagRecord, entry FileEntry, area FilesArea) ([]TagRecord, bool) {
	areaNum := entryArea(entry, area.AreaNum)
	areaAttr := entry.AreaAttr
	if entry.AreaNum == 0 {
		areaAttr = area.Attrib
	}
	for i, t := range tags {
		if tagMatches(t, entry, areaNum) {
			return append(tags[:i], tags[i+1:]...), false
		}
	}
	if len(tags) >= MaxTagged {
		return tags, false
	}
	if entry.Hdr.NotAvail() {
		return tags, false
	}
	t := TagFromHdr(entry.Hdr, areaNum, areaAttr, tagFileName(entry))
	return append(tags, t), true
}

func IsTagged(tags []TagRecord, entry FileEntry, areaNum uint16) bool {
	areaNum = entryArea(entry, areaNum)
	for _, t := range tags {
		if tagMatches(t, entry, areaNum) {
			return true
		}
	}
	return false
}
