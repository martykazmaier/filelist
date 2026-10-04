package elebbs

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	usersRecSize      = 1024
	usersNameOff      = 0
	usersHandleOff    = 266
	usersFileAreaOff  = 955
	usersFileGroupOff = 958
	exitUserInfoOff   = 241
)

type DropHint struct {
	NodePath string
	SysPath  string
	UserName string
	Handle   string
	UserRec  int
}

type Session struct {
	FileArea    uint16
	FileGroup   uint16
	AreaSource  string
	GroupSource string
	UserName    string
	Handle      string
	UserRec     int
	MsgBasePath string
	SysPath     string
}

func ResolveSession(h DropHint) Session {
	s := Session{
		UserName: h.UserName,
		Handle:   h.Handle,
		UserRec:  h.UserRec,
		SysPath:  h.SysPath,
	}

	s.MsgBasePath = findMsgBasePath(h.SysPath, h.NodePath)

	readExit := func(dir, label string) bool {
		p := FindFile(dir, "exitinfo.bbs", "EXITINFO.BBS")
		if p == "" {
			return false
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return false
		}
		area, group, ok := fileAreaFromUserBlob(data, h.UserName, h.Handle)
		if !ok {
			area, group, ok = fileAreaAt(data, exitUserInfoOff)
		}
		if !ok {
			return false
		}
		s.FileArea = area
		s.FileGroup = group
		s.AreaSource = label
		s.GroupSource = label
		return true
	}

	readExit(h.NodePath, "EXITINFO.BBS")
	if s.FileArea == 0 {
		if wd, err := os.Getwd(); err == nil {
			_ = readExit(wd, "EXITINFO.BBS")
		}
	}
	if s.FileArea == 0 {
		_ = readExit(".", "EXITINFO.BBS")
	}

	if s.FileArea == 0 {
		if area, group, ok := readUsersBBS(s.MsgBasePath, h); ok {
			s.FileArea = area
			s.FileGroup = group
			s.AreaSource = "USERS.BBS"
			s.GroupSource = "USERS.BBS"
		}
	}

	if s.FileArea == 0 {
		if area, group, ok := readUsersBBS(h.SysPath, h); ok {
			s.FileArea = area
			s.FileGroup = group
			s.AreaSource = "USERS.BBS"
			s.GroupSource = "USERS.BBS"
		}
	}

	if s.FileArea != 0 && s.FileGroup == 0 {
		if area, err := readFilesRA(h.SysPath, s.FileArea); err == nil && area.Group != 0 {
			s.FileGroup = area.Group
			s.GroupSource = "FILES.RA"
		}
	}

	return s
}

func fileAreaAt(data []byte, userOff int) (uint16, uint16, bool) {
	if userOff < 0 || userOff+usersFileGroupOff+1 >= len(data) {
		return 0, 0, false
	}
	area := u16(data, userOff+usersFileAreaOff)
	group := u16(data, userOff+usersFileGroupOff)
	if area == 0 {
		return 0, 0, false
	}
	return area, group, true
}

func fileAreaFromUserBlob(data []byte, name, handle string) (uint16, uint16, bool) {
	if off, ok := findUserRecOffset(data, name, handle); ok {
		return fileAreaAt(data, off)
	}
	return 0, 0, false
}

func findUserRecOffset(data []byte, name, handle string) (int, bool) {
	candidates := []string{name, handle}
	for _, want := range candidates {
		want = normName(want)
		if want == "" {
			continue
		}
		for i := 0; i+usersFileGroupOff+2 <= len(data); i++ {
			if matchPascal(data[i:], want, 35) {
				area := u16(data, i+usersFileAreaOff)
				if area > 0 && area < 65000 {
					return i, true
				}
			}
			if i+usersHandleOff+36 <= len(data) && matchPascal(data[i+usersHandleOff:], want, 35) {
				area := u16(data, i+usersFileAreaOff)
				if area > 0 && area < 65000 {
					return i, true
				}
			}
		}
	}
	return 0, false
}

func matchPascal(buf []byte, want string, max int) bool {
	if len(buf) < 2 {
		return false
	}
	n := int(buf[0])
	if n == 0 || n > max || 1+n > len(buf) {
		return false
	}
	return normName(cp437ToString(buf[1:1+n])) == want
}

func normName(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

func readUsersBBS(msgBase string, h DropHint) (uint16, uint16, bool) {
	p := FindFile(msgBase, "users.bbs", "USERS.BBS")
	if p == "" && msgBase != "" {
		p = FindFile(filepath.Dir(strings.TrimRight(msgBase, `\`)), "users.bbs", "USERS.BBS")
	}
	if p == "" {
		return 0, 0, false
	}
	data, err := os.ReadFile(p)
	if err != nil || len(data) < usersRecSize {
		return 0, 0, false
	}

	if h.UserRec > 0 {
		off := (h.UserRec - 1) * usersRecSize
		if off >= 0 && off+usersRecSize <= len(data) {
			rec := data[off : off+usersRecSize]
			if namesMatchUser(rec, h.UserName, h.Handle) || h.UserName == "" {
				if area, group, ok := fileAreaAt(rec, 0); ok {
					return area, group, true
				}
			}
		}
		off0 := h.UserRec * usersRecSize
		if off0 >= 0 && off0+usersRecSize <= len(data) {
			rec := data[off0 : off0+usersRecSize]
			if namesMatchUser(rec, h.UserName, h.Handle) {
				if area, group, ok := fileAreaAt(rec, 0); ok {
					return area, group, true
				}
			}
		}
	}

	if off, ok := findUserRecOffset(data, h.UserName, h.Handle); ok {
		return fileAreaAt(data, off)
	}
	return 0, 0, false
}

func namesMatchUser(rec []byte, name, handle string) bool {
	if len(rec) < usersHandleOff+36 {
		return false
	}
	n := normName(pascalString(rec[usersNameOff : usersNameOff+36]))
	h := normName(pascalString(rec[usersHandleOff : usersHandleOff+36]))
	wantN := normName(name)
	wantH := normName(handle)
	if wantN != "" && (n == wantN || h == wantN) {
		return true
	}
	if wantH != "" && (n == wantH || h == wantH) {
		return true
	}
	return wantN == "" && wantH == ""
}

func findMsgBasePath(sysPath, nodePath string) string {
	if p := doorSysMsgBase(nodePath); p != "" {
		return p
	}
	cfg := FindFile(sysPath, "config.ra", "CONFIG.RA")
	if cfg != "" {
		if data, err := os.ReadFile(cfg); err == nil {
			if p := scanExistingDir(data, func(dir string) bool {
				return fileExists(filepath.Join(dir, "users.bbs")) || fileExists(filepath.Join(dir, "USERS.BBS"))
			}); p != "" {
				return p
			}
		}
	}
	for _, c := range []string{
		filepath.Join(sysPath, "msgbase"),
		filepath.Join(sysPath, "msgs"),
		sysPath,
	} {
		if fileExists(filepath.Join(c, "users.bbs")) || fileExists(filepath.Join(c, "USERS.BBS")) {
			return forceBack(c)
		}
	}
	return forceBack(sysPath)
}

func doorSysMsgBase(nodePath string) string {
	p := FindFile(nodePath, "door.sys", "DOOR.SYS")
	if p == "" {
		return ""
	}
	lines, err := readLines(p)
	if err != nil || len(lines) < 33 {
		return ""
	}
	dir := strings.TrimSpace(lines[32])
	if dirExists(dir) {
		return forceBack(dir)
	}
	return ""
}

func ParseDoorSysUserRec(nodePath string) int {
	p := FindFile(nodePath, "door.sys", "DOOR.SYS")
	if p == "" {
		return 0
	}
	lines, err := readLines(p)
	if err != nil || len(lines) < 26 {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(lines[25]))
	return n
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 4096), 64*1024)
	for sc.Scan() {
		lines = append(lines, strings.TrimRight(sc.Text(), "\r"))
	}
	return lines, sc.Err()
}

func scanExistingDir(cfg []byte, ok func(string) bool) string {
	for i := 0; i+61 <= len(cfg); i++ {
		l := int(cfg[i])
		if l == 0 || l > 60 {
			continue
		}
		s := strings.TrimSpace(string(cfg[i+1 : i+1+l]))
		if !looksLikePath(s) {
			continue
		}
		p := forceBack(s)
		if ok(p) {
			return p
		}
	}
	return ""
}
