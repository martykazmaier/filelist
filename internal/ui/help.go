package ui

import (
	"fmt"
	"strings"

	"filelist/internal/elebbs"
)

func (a *App) help() error {
	lines := []string{
		" SPACE   Tag file (moves to next) / untag (stays)",
		" TAB     Show up to 75 characters of the filename",
		"         Any key after TAB restores the list",
		" V       View the selected file",
		" N       List files from the last n days",
		" K       Scan name and description for a keyword",
		" W       Search filenames with wildcards * ?",
		"         After N/K/W: A=Area  G=Group  L=All groups",
		" UP/DN   Previous / next file",
		" L/R     Previous / next page",
		" HOME    First file     END  Last file",
		" Q / ESC Quit, or return from a listing to this area",
		" ?       This help",
		"",
		" Press any key to return",
	}

	var b strings.Builder
	b.Grow(2048)
	b.WriteString("\x1b[?25l\x1b[0m\x1b[2J\x1b[1;1H")
	b.WriteString("\x1b[0;36m")
	top := make([]byte, screenW)
	top[0] = boxTL
	for i := 1; i < screenW-1; i++ {
		top[i] = boxH
	}
	top[screenW-1] = boxTR
	title := []byte(" Help ")
	copy(top[(screenW-len(title))/2:], title)
	b.Write(top)
	b.WriteString("\x1b[0m\r\n")

	inner := screenH - 2
	for i := 0; i < inner; i++ {
		b.WriteString("\x1b[0;36m")
		b.WriteByte(boxV)
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		b.WriteString("\x1b[0;37m")
		b.Write(elebbs.PadCP437(line, screenW-2))
		b.WriteString("\x1b[0;36m")
		b.WriteByte(boxV)
		b.WriteString("\x1b[0m\r\n")
	}

	bot := make([]byte, screenW)
	bot[0] = boxBL
	for i := 1; i < screenW-1; i++ {
		bot[i] = boxH
	}
	bot[screenW-1] = boxBR
	b.WriteString("\x1b[0;36m")
	b.Write(bot)
	b.WriteString("\x1b[0m")
	if _, err := fmt.Fprint(a.Out, b.String()); err != nil {
		return err
	}
	return a.waitDismiss()
}
