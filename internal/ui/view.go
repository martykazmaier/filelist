package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"filelist/internal/comm"
	"filelist/internal/elebbs"
)

func (a *App) view() error {
	e := a.current()
	if e == nil {
		return fmt.Errorf("no file selected")
	}
	cmdLine := strings.TrimSpace(a.ViewCmd)
	if cmdLine == "" {
		return fmt.Errorf("no view command in filelist.ini")
	}
	full := e.FullPath
	if full == "" {
		name := e.DisplayName
		if name == "" {
			name = e.ShortName
		}
		full = name
	}
	if _, err := os.Stat(full); err != nil {
		alt := filepath.Join(filepath.Dir(full), e.ShortName)
		if _, err2 := os.Stat(alt); err2 == nil {
			full = alt
		}
	}
	full = strings.ReplaceAll(filepath.Clean(full), `/`, `\`)
	expanded := expandView(cmdLine, full, a.NodeNum, a.Handle)
	if isNetFoss(cmdLine) && a.Handle != 0 && !hasHandleToken(cmdLine) {
		expanded = insertAfterFirst(expanded, fmt.Sprintf("/N%d /H%d", a.NodeNum, a.Handle))
		expanded = stripTrailingArg(expanded, fmt.Sprintf("/N%d", a.NodeNum))
		expanded = stripTrailingArg(expanded, fmt.Sprintf("/n%d", a.NodeNum))
	}
	if isNetFoss(cmdLine) {
		return comm.RunInheritedConsole(expanded, a.Node, a.Handle)
	}
	cmd := shellCommand(expanded)
	if a.Node != "" {
		cmd.Dir = a.Node
	}
	out, err := cmd.CombinedOutput()
	if err == nil && strings.TrimSpace(string(out)) == "" {
		return nil
	}
	a.pager(expanded, out, err)
	return nil
}

func expandView(tmpl, fullPath string, node int, handle uintptr) string {
	n := strconv.Itoa(node)
	h := strconv.FormatUint(uint64(handle), 10)
	r := strings.NewReplacer(
		"*N", n,
		"*n", n,
		"*H", h,
		"*h", h,
		"@", fullPath,
	)
	return r.Replace(tmpl)
}

func isNetFoss(cmd string) bool {
	l := strings.ToLower(cmd)
	return strings.Contains(l, "nf.bat") || strings.Contains(l, "nf.exe") || strings.Contains(l, "netfoss")
}

func hasHandleToken(cmd string) bool {
	l := strings.ToLower(cmd)
	return strings.Contains(l, "*h") || strings.Contains(l, "/h")
}

func insertAfterFirst(cmd, flags string) string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return flags
	}
	end := 0
	if cmd[0] == '"' {
		i := strings.Index(cmd[1:], `"`)
		if i >= 0 {
			end = i + 2
		}
	}
	if end == 0 {
		end = strings.IndexByte(cmd, ' ')
		if end < 0 {
			return cmd + " " + flags
		}
	}
	rest := strings.TrimSpace(cmd[end:])
	if rest == "" {
		return strings.TrimSpace(cmd[:end] + " " + flags)
	}
	return strings.TrimSpace(cmd[:end]) + " " + flags + " " + rest
}

func stripTrailingArg(cmd, arg string) string {
	cmd = strings.TrimSpace(cmd)
	if arg == "" {
		return cmd
	}
	if len(cmd) >= len(arg)+1 && strings.EqualFold(cmd[len(cmd)-len(arg):], arg) {
		prev := cmd[:len(cmd)-len(arg)]
		if strings.HasSuffix(prev, " ") {
			return strings.TrimSpace(prev)
		}
	}
	return cmd
}

func (a *App) pager(title string, data []byte, runErr error) {
	text := string(data)
	if runErr != nil {
		text = fmt.Sprintf("View failed: %v\r\n%s", runErr, text)
	}
	if text == "" {
		text = "(no output)"
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")

	page := 0
	rows := 22
	for {
		var b strings.Builder
		b.WriteString("\x1b[?25l\x1b[0m\x1b[2J\x1b[1;1H")
		b.WriteString("\x1b[1;37;44m")
		b.Write(elePad(fmt.Sprintf(" VIEW: %s", title), screenW))
		b.WriteString("\x1b[0m\r\n")
		start := page * rows
		for i := 0; i < rows; i++ {
			line := ""
			if start+i < len(lines) {
				line = lines[start+i]
			}
			b.WriteString("\x1b[0;37m")
			b.Write(elePad(line, screenW))
			b.WriteString("\x1b[0m\r\n")
		}
		pc := (len(lines) + rows - 1) / rows
		if pc < 1 {
			pc = 1
		}
		b.WriteString("\x1b[0;30;47m")
		b.Write(elePad(fmt.Sprintf(" Pg %d/%d  UP/DN PgUp/PgDn  ESC/Q back ", page+1, pc), screenW))
		b.WriteString("\x1b[0m")
		_, _ = a.Out.Write([]byte(b.String()))

		ev, err := a.nextKey()
		if err != nil {
			return
		}
		switch ev.Key {
		case comm.KeyNone:
			continue
		case comm.KeyEsc, comm.KeyHangup:
			return
		case comm.KeyChar:
			if comm.KeyCharEquals(ev, 'Q') {
				return
			}
		case comm.KeyUp, comm.KeyLeft, comm.KeyPgUp:
			if page > 0 {
				page--
			}
		case comm.KeyDown, comm.KeyRight, comm.KeyPgDn, comm.KeySpace, comm.KeyEnter:
			if page+1 < pc {
				page++
			}
		}
	}
}

func elePad(s string, w int) []byte {
	return elebbs.PadCP437(s, w)
}
