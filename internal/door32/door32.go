package door32

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Drop struct {
	Path       string
	NodePath   string
	CommType   int
	Handle     uintptr
	Baud       int
	BBSID      string
	UserRec    int
	RealName   string
	HandleName string
	Security   int
	TimeLeft   int
	Emulation  int
	Node       int
	Local      bool
}

func Find(arg string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}
	for _, name := range []string{"door32.sys", "DOOR32.SYS"} {
		p := filepath.Join(wd, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("door32.sys not found in current directory")
}

func Parse(path string) (*Drop, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 4096), 64*1024)
	var lines []string
	for sc.Scan() {
		lines = append(lines, strings.TrimRight(sc.Text(), "\r"))
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	d := &Drop{Path: path, NodePath: "."}
	if wd, err := os.Getwd(); err == nil {
		d.NodePath = wd
	}
	get := func(i int) string {
		if i < len(lines) {
			return strings.TrimSpace(lines[i])
		}
		return ""
	}
	d.CommType, _ = strconv.Atoi(get(0))
	h, _ := strconv.ParseUint(get(1), 10, 64)
	d.Handle = uintptr(h)
	d.Baud, _ = strconv.Atoi(get(2))
	d.BBSID = get(3)
	d.UserRec, _ = strconv.Atoi(get(4))
	d.RealName = get(5)
	d.HandleName = get(6)
	d.Security, _ = strconv.Atoi(get(7))
	d.TimeLeft, _ = strconv.Atoi(get(8))
	d.Emulation, _ = strconv.Atoi(get(9))
	d.Node, _ = strconv.Atoi(get(10))
	d.Local = d.CommType == 0 || d.Handle == 0
	return d, nil
}
