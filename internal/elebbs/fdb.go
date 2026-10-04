package elebbs

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type FileEntry struct {
	Hdr         FilesHdr
	ShortName   string
	DisplayName string
	FullPath    string
	Description string
	Tagged      bool
	AreaNum     uint16
	AreaName    string
	AreaAttr    byte
}

type AreaContext struct {
	SysPath      string
	NodePath     string
	FileBasePath string
	Area         FilesArea
	Group        Group
	Entries      []FileEntry
}

func FindSysPath(hint, nodePath, exeDir string) string {
	tried := []string{}
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		tried = append(tried, p)
	}
	add(hint)
	add(os.Getenv("RA"))
	add(os.Getenv("RABBS"))
	add(os.Getenv("ELE"))
	add(os.Getenv("ELEBBS"))
	if nodePath != "" {
		add(nodePath)
		add(filepath.Dir(nodePath))
		add(filepath.Dir(filepath.Dir(nodePath)))
	}
	add(exeDir)
	if wd, err := os.Getwd(); err == nil {
		add(wd)
	}

	for _, p := range tried {
		if hasRAConfig(p) {
			return p
		}
		parent := filepath.Dir(p)
		if parent != p && hasRAConfig(parent) {
			return parent
		}
	}
	for _, p := range tried {
		if p != "" {
			return p
		}
	}
	return "."
}

func hasRAConfig(p string) bool {
	if p == "" {
		return false
	}
	for _, name := range []string{"config.ra", "CONFIG.RA", "files.ra", "FILES.RA"} {
		if fileExists(filepath.Join(p, name)) {
			return true
		}
	}
	return false
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func FindFile(dir string, names ...string) string {
	for _, n := range names {
		p := filepath.Join(dir, n)
		if fileExists(p) {
			return p
		}
	}
	return ""
}

func hasFDBHdr(dir string) bool {
	matches, _ := filepath.Glob(filepath.Join(dir, "fdb*.hdr"))
	return len(matches) > 0
}

func looksLikePath(s string) bool {
	if len(s) < 2 {
		return false
	}
	if strings.ContainsAny(s, "\x00\x01\x02\x03\x04\x05") {
		return false
	}
	if (s[0] >= 'A' && s[0] <= 'Z' || s[0] >= 'a' && s[0] <= 'z') && s[1] == ':' {
		return true
	}
	if strings.HasPrefix(s, `\\`) || strings.HasPrefix(s, "/") {
		return true
	}
	return false
}

func forceBack(p string) string {
	p = strings.TrimSpace(p)
	p = strings.ReplaceAll(p, "/", `\`)
	if p != "" && !strings.HasSuffix(p, `\`) {
		p += `\`
	}
	return p
}

func fdbPath(fileBase, kind string, area uint16) string {
	base := strings.TrimRight(fileBase, `\`)
	return filepath.Join(base, strings.ToLower(kind), "fdb"+strconv.Itoa(int(area))+"."+strings.ToLower(kind))
}

func ReadExitFileArea(nodePath string) (area uint16, group uint16, ok bool) {
	p := FindFile(nodePath, "exitinfo.bbs", "EXITINFO.BBS")
	if p == "" {
		return 0, 0, false
	}
	data, err := os.ReadFile(p)
	if err != nil || len(data) < 1202 {
		return 0, 0, false
	}
	const userInfoOff = 241
	const fileAreaOff = userInfoOff + 955
	const fileGroupOff = userInfoOff + 958
	area = u16(data, fileAreaOff)
	group = u16(data, fileGroupOff)
	if area == 0 {
		return 0, 0, false
	}
	return area, group, true
}

func LoadArea(sysPath, nodePath string, areaNum uint16) (*AreaContext, error) {
	ctx := &AreaContext{
		SysPath:      sysPath,
		NodePath:     nodePath,
		FileBasePath: ReadFileBasePath(sysPath),
	}
	area, err := readFilesRA(sysPath, areaNum)
	if err != nil {
		return nil, err
	}
	ctx.Area = area
	ctx.Group = readGroup(sysPath, area.Group)

	entries, err := readFDB(ctx)
	if err != nil {
		return nil, err
	}
	ctx.Entries = entries
	return ctx, nil
}

func readFilesRA(sysPath string, areaNum uint16) (FilesArea, error) {
	p := findRAFile(sysPath, "files.ra", "FILES.RA")
	if p == "" {
		return FilesArea{}, fmt.Errorf("FILES.RA not found in %s", sysPath)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return FilesArea{}, err
	}
	if areaNum == 0 {
		areaNum = 1
	}
	n := len(data) / FilesRecSize
	var fallback FilesArea
	for i := 0; i < n; i++ {
		rec := parseFilesArea(data[i*FilesRecSize : (i+1)*FilesRecSize])
		if rec.AreaNum == 0 && rec.Name == "" {
			continue
		}
		if rec.AreaNum == areaNum {
			return rec, nil
		}
		if fallback.AreaNum == 0 && rec.Name != "" {
			fallback = rec
		}
		if uint16(i+1) == areaNum && rec.AreaNum != 0 {
			fallback = rec
		}
	}
	if fallback.AreaNum != 0 {
		return fallback, nil
	}
	return FilesArea{}, fmt.Errorf("file area %d not found in FILES.RA", areaNum)
}

func LookupGroup(sysPath string, groupNum uint16) Group {
	return readGroup(sysPath, groupNum)
}

func readGroup(sysPath string, groupNum uint16) Group {
	if groupNum == 0 {
		return Group{}
	}
	p := findRAFile(sysPath, "fgroups.ra", "FGROUPS.RA")
	if p == "" {
		return Group{}
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return Group{}
	}
	n := len(data) / GroupRecSize
	for i := 0; i < n; i++ {
		g := parseGroup(data[i*GroupRecSize : (i+1)*GroupRecSize])
		if g.AreaNum == groupNum {
			return g
		}
	}
	return Group{}
}

func ListFileAreas(sysPath string) []FilesArea {
	p := findRAFile(sysPath, "files.ra", "FILES.RA")
	if p == "" {
		return nil
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	n := len(data) / FilesRecSize
	out := make([]FilesArea, 0, n)
	seen := map[uint16]bool{}
	for i := 0; i < n; i++ {
		rec := parseFilesArea(data[i*FilesRecSize : (i+1)*FilesRecSize])
		if rec.AreaNum == 0 || rec.Name == "" {
			continue
		}
		if rec.Attrib&AreaAttrNotInFDB != 0 {
			continue
		}
		if seen[rec.AreaNum] {
			continue
		}
		seen[rec.AreaNum] = true
		out = append(out, rec)
	}
	return out
}

type ScanScope int

const (
	ScopeArea ScanScope = iota
	ScopeGroup
	ScopeAll
)

func (s ScanScope) Label() string {
	switch s {
	case ScopeGroup:
		return "Group"
	case ScopeAll:
		return "All"
	default:
		return "Area"
	}
}

func ScanFiles(ctx *AreaContext, scope ScanScope, sec int, match func(FileEntry) bool) []FileEntry {
	if ctx == nil {
		return nil
	}
	saved := ctx.Area
	defer func() { ctx.Area = saved }()

	var areas []FilesArea
	switch scope {
	case ScopeArea:
		areas = []FilesArea{saved}
	case ScopeGroup:
		group := saved.Group
		if group == 0 {
			group = ctx.Group.AreaNum
		}
		if group == 0 {
			areas = []FilesArea{saved}
			break
		}
		for _, area := range ListFileAreas(ctx.SysPath) {
			if area.Group != group {
				continue
			}
			if sec > 0 && int(area.ListSecurity) > sec {
				continue
			}
			areas = append(areas, area)
		}
	default:
		for _, area := range ListFileAreas(ctx.SysPath) {
			if sec > 0 && int(area.ListSecurity) > sec {
				continue
			}
			areas = append(areas, area)
		}
	}

	var out []FileEntry
	for _, area := range areas {
		ctx.Area = area
		entries, err := readFDB(ctx)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if match(e) {
				out = append(out, e)
			}
		}
	}
	return out
}

func readFDB(ctx *AreaContext) ([]FileEntry, error) {
	area := ctx.Area.AreaNum
	hdrPath := fdbPath(ctx.FileBasePath, "hdr", area)
	txtPath := fdbPath(ctx.FileBasePath, "txt", area)
	if !fileExists(hdrPath) {
		for _, base := range []string{
			ctx.FileBasePath,
			filepath.Join(ctx.SysPath, "filebase"),
			filepath.Join(ctx.SysPath, "FileBase"),
		} {
			p := fdbPath(base, "hdr", area)
			if fileExists(p) {
				hdrPath = p
				txtPath = fdbPath(base, "txt", area)
				ctx.FileBasePath = forceBack(base)
				break
			}
			alt := filepath.Join(strings.TrimRight(base, `\`), fmt.Sprintf("fdb%d.hdr", area))
			if fileExists(alt) {
				hdrPath = alt
				txtPath = strings.TrimSuffix(alt, filepath.Ext(alt)) + ".txt"
				ctx.FileBasePath = forceBack(filepath.Dir(alt))
				break
			}
		}
	}
	hdrData, err := os.ReadFile(hdrPath)
	if err != nil {
		return nil, fmt.Errorf("open FDB header %s: %w", hdrPath, err)
	}
	var txtData []byte
	if fileExists(txtPath) {
		txtData, _ = os.ReadFile(txtPath)
	}

	n := len(hdrData) / FilesHdrSize
	out := make([]FileEntry, 0, n)
	filePath := forceBack(ctx.Area.FilePath)
	for i := 0; i < n; i++ {
		h := parseFilesHdr(hdrData[i*FilesHdrSize:(i+1)*FilesHdrSize], uint16(i+1))
		if h.Comment() || h.Deleted() || h.Unlisted() {
			continue
		}
		e := FileEntry{
			Hdr:       h,
			ShortName: h.Name,
			AreaNum:   ctx.Area.AreaNum,
			AreaName:  ctx.Area.Name,
			AreaAttr:  ctx.Area.Attrib,
		}
		e.DisplayName = resolveDisplayName(filePath, h, txtData)
		e.FullPath = filePath + e.DisplayName
		if h.LongDescPtr >= 0 && int(h.LongDescPtr) < len(txtData) {
			e.Description = readCString(txtData[h.LongDescPtr:])
		}
		out = append(out, e)
	}
	return out, nil
}

func resolveDisplayName(filePath string, h FilesHdr, txtData []byte) string {
	hdr := strings.TrimSpace(h.Name)
	lfn := ""
	if h.LfnPtr > 0 && int(h.LfnPtr) < len(txtData) {
		lfn = strings.TrimSpace(readTxtName(txtData[h.LfnPtr:]))
	}
	if lfn != "" && len(stringToCP437(lfn)) > len(stringToCP437(hdr)) {
		return lfn
	}
	if disk := diskLongName(filePath, hdr, h.Size); disk != "" {
		return disk
	}
	if lfn != "" {
		return lfn
	}
	return hdr
}

func readTxtName(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if c := strings.TrimSpace(readCString(b)); c != "" {
		return c
	}
	n := int(b[0])
	if n > 0 && n <= len(b)-1 {
		return strings.TrimSpace(pascalString(b[:1+n]))
	}
	return ""
}

func readCString(b []byte) string {
	n := 0
	for n < len(b) && b[n] != 0 {
		n++
	}
	raw := b[:n]
	raw = filterDesc(raw)
	return cp437ToString(raw)
}

func filterDesc(b []byte) []byte {
	out := make([]byte, 0, len(b))
	for _, c := range b {
		if c == 0 {
			break
		}
		if c == 0x0D {
			continue
		}
		out = append(out, c)
	}
	return out
}

func WrapText(s string, width int) []string {
	if width < 1 {
		width = 1
	}
	raw := stringToCP437(s)
	var lines []string
	for len(raw) > 0 {
		nl := indexByte(raw, 0x0A)
		var line []byte
		if nl >= 0 {
			line = raw[:nl]
			raw = raw[nl+1:]
		} else {
			line = raw
			raw = nil
		}
		for len(line) > width {
			cut := width
			for i := width; i > width/3; i-- {
				if line[i] == ' ' {
					cut = i
					break
				}
			}
			lines = append(lines, cp437ToString(line[:cut]))
			line = line[cut:]
			if len(line) > 0 && line[0] == ' ' {
				line = line[1:]
			}
		}
		lines = append(lines, cp437ToString(line))
	}
	return lines
}

func indexByte(b []byte, c byte) int {
	for i, v := range b {
		if v == c {
			return i
		}
	}
	return -1
}

func FormatSize(n uint64) string {
	const (
		kb = 1024
		mb = 1024 * 1024
		gb = 1024 * 1024 * 1024
	)
	switch {
	case n >= gb:
		return fmt.Sprintf("%.2f GB", float64(n)/float64(gb))
	case n >= mb:
		if n >= 10*mb {
			return fmt.Sprintf("%.0f MB", float64(n)/float64(mb))
		}
		return fmt.Sprintf("%.1f MB", float64(n)/float64(mb))
	case n >= kb:
		return fmt.Sprintf("%d KB", n/kb)
	default:
		return fmt.Sprintf("%d bytes", n)
	}
}

func FormatComma(n uint32) string {
	s := strconv.FormatUint(uint64(n), 10)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre == 0 {
		pre = 3
	}
	b.WriteString(s[:pre])
	for i := pre; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

func FormatDate(v uint32) string {
	t := FileStamp(v)
	if t.IsZero() {
		return "Never"
	}
	return t.In(time.Local).Format("01-02-06")
}
