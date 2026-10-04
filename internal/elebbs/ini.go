package elebbs

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

func LoadINI(exeDir string) (viewCmd string, extra map[string]string) {
	extra = map[string]string{}
	cands := []string{
		filepath.Join(exeDir, "filelist.ini"),
		"filelist.ini",
	}
	var path string
	for _, p := range cands {
		if fileExists(p) {
			path = p
			break
		}
	}
	if path == "" {
		return "", extra
	}
	f, err := os.Open(path)
	if err != nil {
		return "", extra
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	first := true
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimRight(sc.Text(), "\r"))
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			if first && (strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";")) {
				continue
			}
			if first && line == "" {
				continue
			}
		}
		if first {
			viewCmd = line
			first = false
			continue
		}
		if i := strings.IndexByte(line, '='); i > 0 {
			extra[strings.ToLower(strings.TrimSpace(line[:i]))] = strings.TrimSpace(line[i+1:])
		}
	}
	return viewCmd, extra
}
