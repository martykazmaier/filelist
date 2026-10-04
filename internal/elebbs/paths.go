package elebbs

import (
	"os"
	"path/filepath"
	"strings"
)

func ReadFileBasePath(sysPath string) string {
	p, _ := ResolvePaths(sysPath)
	return p
}

func ResolvePaths(sysPath string) (fileBase, raSys string) {
	raSys = forceBack(sysPath)
	cands := []string{}

	add := func(p string) {
		p = strings.TrimSpace(p)
		p = strings.ReplaceAll(p, "/", `\`)
		p = strings.TrimRight(p, `\`)
		if p == "" {
			return
		}
		for _, e := range cands {
			if strings.EqualFold(e, p) {
				return
			}
		}
		cands = append(cands, p)
	}

	for _, name := range []string{"config.ra", "CONFIG.RA", "config.ele", "CONFIG.ELE"} {
		fp := FindFile(sysPath, name)
		if fp == "" {
			continue
		}
		if data, err := os.ReadFile(fp); err == nil {
			for _, p := range extractPaths(data) {
				add(p)
			}
		}
	}

	add(filepath.Join(sysPath, "filebase"))
	add(filepath.Join(sysPath, "FileBase"))
	add(filepath.Join(sysPath, "FILEBASE"))
	add(filepath.Join(sysPath, "fdb"))
	add(sysPath)

	best := ""
	bestScore := -1
	for _, p := range cands {
		s := scoreFileBase(p)
		if s > bestScore {
			bestScore = s
			best = p
		}
		if hdr := filepath.Join(p, "hdr"); dirExists(hdr) && hasFDBHdr(hdr) {
			return forceBack(p), raSys
		}
		if strings.EqualFold(filepath.Base(p), "hdr") && hasFDBHdr(p) {
			return forceBack(filepath.Dir(p)), raSys
		}
	}
	if best != "" && bestScore > 0 {
		if strings.EqualFold(filepath.Base(best), "hdr") {
			return forceBack(filepath.Dir(best)), raSys
		}
		return forceBack(best), raSys
	}
	fb := filepath.Join(sysPath, "filebase")
	if dirExists(fb) {
		return forceBack(fb), raSys
	}
	return forceBack(sysPath), raSys
}

func scoreFileBase(p string) int {
	score := 0
	hdr := filepath.Join(p, "hdr")
	if hasFDBHdr(hdr) {
		return 100
	}
	if dirExists(hdr) {
		score += 50
	}
	if hasFDBHdr(p) {
		score += 40
	}
	low := strings.ToLower(p)
	if strings.Contains(low, "filebase") || strings.Contains(low, "fdb") {
		score += 20
	}
	if dirExists(p) {
		score += 5
	}
	return score
}

func extractPaths(data []byte) []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if !looksLikePath(s) {
			return
		}
		key := strings.ToLower(s)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, s)
	}

	maxLen := []int{60, 250}
	for _, max := range maxLen {
		for i := 0; i+max+1 <= len(data); i++ {
			l := int(data[i])
			if l < 3 || l > max {
				continue
			}
			raw := data[i+1 : i+1+l]
			if !printablePath(raw) {
				continue
			}
			add(string(raw))
		}
	}

	// Text / INI-style lines in case CONFIG.ELE was saved as text
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if i := strings.IndexByte(line, '='); i > 0 {
			key := strings.ToLower(strings.TrimSpace(line[:i]))
			val := strings.TrimSpace(line[i+1:])
			if strings.Contains(key, "filebase") || strings.Contains(key, "filepath") || key == "syspath" {
				add(val)
			}
		}
		if looksLikePath(line) {
			add(line)
		}
	}
	return out
}

func printablePath(b []byte) bool {
	if len(b) < 2 {
		return false
	}
	for _, c := range b {
		if c < 32 || c > 126 {
			return false
		}
	}
	return true
}

func findRAFile(sysPath string, names ...string) string {
	dirs := []string{
		sysPath,
		filepath.Join(sysPath, "filebase"),
		filepath.Dir(strings.TrimRight(sysPath, `\`)),
	}
	if fb, _ := ResolvePaths(sysPath); fb != "" {
		dirs = append([]string{strings.TrimRight(fb, `\`)}, dirs...)
	}
	for _, d := range dirs {
		if p := FindFile(d, names...); p != "" {
			return p
		}
	}
	return ""
}
