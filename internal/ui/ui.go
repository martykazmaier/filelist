package ui

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"filelist/internal/comm"
	"filelist/internal/elebbs"
)

const (
	screenW = 79
	screenH = 24

	leftW  = 30
	rightW = 45

	colLeft   = 1
	colMid    = 32
	colRight  = 33
	colBorder = 78

	rowTop     = 1
	rowFirst   = 2
	rowInfo    = 17
	rowSize    = 18
	rowDate    = 19
	rowTagInfo = 20
	rowTagN    = 21
	rowTagSz   = 22
	rowBottom  = 23
	rowStatus  = 24

	pageSize = 21
	descRows = 15
)

const (
	boxV  byte = 179
	boxH  byte = 196
	boxTL byte = 218
	boxTR byte = 191
	boxBL byte = 192
	boxBR byte = 217
	boxML byte = 195
	boxMR byte = 180
	boxTD byte = 194
	boxBU byte = 193
	chkMk byte = 251
)

type App struct {
	Out      io.Writer
	Keys     *comm.Keyboard
	Ctx      *elebbs.AreaContext
	Tags     []elebbs.TagRecord
	ViewCmd  string
	Node     string
	NodeNum  int
	Handle   uintptr
	User     string
	Security int
	home     []elebbs.FileEntry
	listing  string
	cursor   int
}

func Run(a *App) error {
	if a == nil || a.Ctx == nil {
		return fmt.Errorf("no file area loaded")
	}
	if a.Ctx.Entries == nil {
		a.Ctx.Entries = []elebbs.FileEntry{}
	}
	a.home = append([]elebbs.FileEntry(nil), a.Ctx.Entries...)
	a.syncTags()
	if err := a.draw(); err != nil {
		return err
	}
	for {
		ev, err := a.Keys.Next(30 * time.Minute)
		if err != nil {
			return a.shutdown()
		}
		switch ev.Key {
		case comm.KeyHangup:
			return a.shutdown()
		case comm.KeyEsc:
			if a.listing != "" {
				a.restoreArea()
				break
			}
			return a.shutdown()
		case comm.KeyNone:
			continue
		case comm.KeyUp:
			a.move(-1)
		case comm.KeyDown:
			a.move(1)
		case comm.KeyLeft, comm.KeyPgUp:
			a.page(-1)
		case comm.KeyRight, comm.KeyPgDn:
			a.page(1)
		case comm.KeyHome:
			a.cursor = 0
		case comm.KeyEnd:
			if n := len(a.Ctx.Entries); n > 0 {
				a.cursor = n - 1
			}
		case comm.KeySpace:
			a.toggle()
			if e := a.current(); e != nil && e.Tagged {
				a.move(1)
			}
		case comm.KeyTab:
			if err := a.peekName(); err != nil {
				return a.shutdown()
			}
		case comm.KeyChar:
			if comm.KeyCharEquals(ev, 'Q') {
				if a.listing != "" {
					a.restoreArea()
					break
				}
				return a.shutdown()
			}
			if ev.Raw == '?' || comm.KeyCharEquals(ev, '?') {
				if err := a.help(); err != nil {
					return a.shutdown()
				}
			}
			if a.listing != "" && (comm.KeyCharEquals(ev, 'C') || comm.KeyCharEquals(ev, 'A')) {
				a.restoreArea()
			}
			if comm.KeyCharEquals(ev, 'V') {
				if err := a.view(); err != nil {
					a.flash(err.Error())
				}
			}
			if comm.KeyCharEquals(ev, 'N') {
				if err := a.doSearch(a.searchDays); err != nil {
					return a.shutdown()
				}
			}
			if comm.KeyCharEquals(ev, 'K') {
				if err := a.doSearch(a.searchKeyword); err != nil {
					return a.shutdown()
				}
			}
			if comm.KeyCharEquals(ev, 'W') {
				if err := a.doSearch(a.searchWild); err != nil {
					return a.shutdown()
				}
			}
		}
		if err := a.draw(); err != nil {
			return err
		}
	}
}

func (a *App) shutdown() error {
	_, _ = a.Out.Write([]byte("\x1b[0m\x1b[?25h\x1b[24;1H\x1b[K\r\n"))
	return nil
}

func (a *App) doSearch(fn func() error) error {
	err := fn()
	if err == nil || errors.Is(err, errPromptCancel) {
		return nil
	}
	return err
}

func (a *App) peekName() error {
	e := a.current()
	if e == nil {
		return nil
	}
	name := e.DisplayName
	if name == "" {
		name = e.ShortName
	}
	row := rowFirst + (a.cursor % pageSize)
	text := elebbs.PadName(name, 75)
	if _, err := fmt.Fprintf(a.Out, "\x1b[%d;%dH\x1b[0;30;47m", row, colLeft+1); err != nil {
		return err
	}
	if _, err := a.Out.Write(text); err != nil {
		return err
	}
	if _, err := a.Out.Write([]byte("\x1b[0m")); err != nil {
		return err
	}
	return a.waitDismiss()
}

func (a *App) waitDismiss() error {
	for {
		ev, err := a.Keys.Next(30 * time.Minute)
		if err != nil || ev.Key == comm.KeyHangup {
			return io.EOF
		}
		if ev.Key == comm.KeyNone {
			continue
		}
		return nil
	}
}

func (a *App) n() int { return len(a.Ctx.Entries) }

func (a *App) pageCount() int {
	if a.n() == 0 {
		return 1
	}
	return (a.n() + pageSize - 1) / pageSize
}

func (a *App) curPage() int {
	if a.n() == 0 {
		return 0
	}
	return a.cursor / pageSize
}

func (a *App) move(delta int) {
	if a.n() == 0 {
		return
	}
	a.cursor += delta
	if a.cursor < 0 {
		a.cursor = a.n() - 1
	}
	if a.cursor >= a.n() {
		a.cursor = 0
	}
}

func (a *App) page(delta int) {
	if a.n() == 0 {
		return
	}
	p := a.curPage() + delta
	pc := a.pageCount()
	if p < 0 {
		p = pc - 1
	}
	if p >= pc {
		p = 0
	}
	a.cursor = p * pageSize
	if a.cursor >= a.n() {
		a.cursor = a.n() - 1
	}
}

func (a *App) current() *elebbs.FileEntry {
	if a.cursor < 0 || a.cursor >= a.n() {
		return nil
	}
	return &a.Ctx.Entries[a.cursor]
}

func (a *App) toggle() {
	e := a.current()
	if e == nil {
		return
	}
	tags, _ := elebbs.ToggleTag(a.Tags, *e, a.Ctx.Area)
	a.Tags = tags
	_ = elebbs.SaveTagList(a.Node, a.Tags)
	a.syncTags()
}

func (a *App) syncTags() {
	for i := range a.Ctx.Entries {
		a.Ctx.Entries[i].Tagged = elebbs.IsTagged(a.Tags, a.Ctx.Entries[i], a.Ctx.Area.AreaNum)
	}
}

func (a *App) draw() error {
	var b strings.Builder
	b.Grow(4096)
	b.WriteString("\x1b[?25l\x1b[0m\x1b[2J\x1b[1;1H")

	titleL := a.leftTitle()
	titleR := "Description"
	a.hline(&b, rowTop, boxTL, boxTD, boxTR, titleL, titleR, "\x1b[0;36m")

	start := a.curPage() * pageSize
	sel := a.current()
	descLines := []string{}
	if sel != nil {
		if a.listing != "" && sel.AreaName != "" {
			descLines = append(descLines, "Area: "+sel.AreaName)
		}
		descLines = append(descLines, elebbs.WrapText(sel.Description, rightW)...)
	}

	for i := 0; i < pageSize; i++ {
		row := rowFirst + i
		idx := start + i
		a.leftFile(&b, row, idx, idx == a.cursor)
		if i < descRows {
			line := ""
			if i < len(descLines) {
				line = descLines[i]
			}
			a.rightText(&b, row, line, "\x1b[0;37m")
		}
	}

	a.rightInfoBar(&b, rowInfo, " Size / Date / DLs ")

	sizeStr, dateStr := "No file selected", ""
	if sel != nil {
		sizeStr = fmt.Sprintf("Size: %s bytes   DLs: %d", elebbs.FormatComma(sel.Hdr.Size), sel.Hdr.TimesDL)
		dl := elebbs.FormatDate(sel.Hdr.LastDL)
		fd := elebbs.FormatDate(sel.Hdr.FileDate)
		dateStr = fmt.Sprintf("Date: %s   Last DL: %s", fd, dl)
	}
	a.rightText(&b, rowSize, sizeStr, "\x1b[1;33m")
	a.rightText(&b, rowDate, dateStr, "\x1b[1;33m")

	nTag, tagBytes := a.tagTotals()
	a.rightInfoBar(&b, rowTagInfo, " Tagged ")
	a.rightText(&b, rowTagN, fmt.Sprintf("Files: %d", nTag), "\x1b[1;33m")
	a.rightText(&b, rowTagSz, fmt.Sprintf("Total: %s", elebbs.FormatSize(tagBytes)), "\x1b[1;33m")

	a.hline(&b, rowBottom, boxBL, boxBU, boxBR, "", "", "\x1b[0;36m")

	tagged := 0
	for _, e := range a.Ctx.Entries {
		if e.Tagged {
			tagged++
		}
	}
	status := fmt.Sprintf(" N Days  K Key  W Wild  V View  SPACE Tag  TAB  ?  Q  %d/%d P%d/%d T:%d",
		min(a.cursor+1, a.n()), a.n(), a.curPage()+1, a.pageCount(), tagged)
	b.WriteString(fmt.Sprintf("\x1b[%d;1H\x1b[0;30;47m%s\x1b[0m", rowStatus, padVis(status, screenW)))
	_, err := io.WriteString(a.Out, b.String())
	return err
}

func (a *App) leftTitle() string {
	if a.listing != "" {
		return " " + a.listing + " "
	}
	name := a.Ctx.Area.Name
	if a.Ctx.Group.Name != "" {
		name = a.Ctx.Group.Name + " / " + name
	}
	if name == "" {
		name = fmt.Sprintf("Area %d", a.Ctx.Area.AreaNum)
	}
	return " " + name + " "
}

func (a *App) leftFile(b *strings.Builder, row, idx int, selected bool) {
	a.cell(b, row, colLeft, "\x1b[0;36m", []byte{boxV})
	a.cell(b, row, colMid, "\x1b[0;36m", []byte{boxV})

	attr := "\x1b[0;36m"
	text := bytesRepeat(byte(' '), leftW)
	if idx >= 0 && idx < a.n() {
		e := a.Ctx.Entries[idx]
		tagged := elebbs.IsTagged(a.Tags, e, a.Ctx.Area.AreaNum)
		mark := byte(' ')
		if tagged {
			mark = chkMk
		}
		shown := e.DisplayName
		if shown == "" {
			shown = e.ShortName
		}
		name := elebbs.PadName(shown, leftW-2)
		text = append([]byte{mark, ' '}, name...)
		if e.Hdr.Missing() {
			attr = "\x1b[1;30m"
		} else if tagged {
			attr = "\x1b[1;33m"
		} else {
			attr = "\x1b[1;36m"
		}
		if selected {
			attr = "\x1b[0;30;46m"
		}
	} else if selected {
		attr = "\x1b[0;30;46m"
	}
	b.WriteString(fmt.Sprintf("\x1b[%d;%dH%s", row, colLeft+1, attr))
	b.Write(text)
	b.WriteString("\x1b[0m")
}

func (a *App) rightText(b *strings.Builder, row int, s, color string) {
	a.cell(b, row, colBorder, "\x1b[0;36m", []byte{boxV})
	padded := elebbs.PadCP437(s, rightW)
	b.WriteString(fmt.Sprintf("\x1b[%d;%dH%s", row, colRight, color))
	b.Write(padded)
	b.WriteString("\x1b[0m")
}

func (a *App) rightRaw(b *strings.Builder, row int, s, color string) {
	raw := []byte(s)
	if len(raw) > rightW {
		raw = raw[:rightW]
	}
	for len(raw) < rightW {
		raw = append(raw, boxH)
	}
	b.WriteString(fmt.Sprintf("\x1b[%d;%dH%s", row, colRight, color))
	b.Write(raw)
	b.WriteString("\x1b[0m")
}

func (a *App) hline(b *strings.Builder, row int, left, mid, right byte, titleL, titleR, color string) {
	line := make([]byte, screenW)
	for i := range line {
		line[i] = ' '
	}
	line[colLeft-1] = left
	for i := colLeft; i < colMid-1; i++ {
		line[i] = boxH
	}
	line[colMid-1] = mid
	for i := colMid; i < colBorder-1; i++ {
		line[i] = boxH
	}
	line[colBorder-1] = right

	putTitle := func(start, width int, title string) {
		t := []byte(title)
		if len(t) > width {
			t = t[:width]
		}
		off := start + (width-len(t))/2
		copy(line[off:], t)
	}
	if titleL != "" {
		putTitle(colLeft-1+1, leftW, titleL)
	}
	if titleR != "" {
		putTitle(colRight-1, rightW, " "+titleR+" ")
	}
	b.WriteString(fmt.Sprintf("\x1b[%d;1H%s", row, color))
	b.Write(line)
	b.WriteString("\x1b[0m")
}

func (a *App) rightInfoBar(b *strings.Builder, row int, title string) {
	a.cell(b, row, colMid, "\x1b[0;36m", []byte{boxML})
	a.cell(b, row, colBorder, "\x1b[0;36m", []byte{boxMR})
	rest := rightW - len(title)
	if rest < 0 {
		rest = 0
	}
	leftPad := rest / 2
	rightPad := rest - leftPad
	info := strings.Repeat(string([]byte{boxH}), leftPad) + title + strings.Repeat(string([]byte{boxH}), rightPad)
	a.rightRaw(b, row, info, "\x1b[0;36m")
}

func (a *App) tagTotals() (int, uint64) {
	n := 0
	var bytes uint64
	for _, t := range a.Tags {
		n++
		bytes += uint64(t.Size)
	}
	return n, bytes
}

func (a *App) cell(b *strings.Builder, row, col int, color string, ch []byte) {
	b.WriteString(fmt.Sprintf("\x1b[%d;%dH%s", row, col, color))
	b.Write(ch)
}

func (a *App) flash(msg string) {
	_, _ = fmt.Fprintf(a.Out, "\x1b[%d;1H\x1b[0;37;41m%s\x1b[0m", rowStatus, padVis(msg, screenW))
	time.Sleep(1200 * time.Millisecond)
}

func bytesRepeat(ch byte, n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = ch
	}
	return b
}

func padVis(s string, w int) string {
	r := []rune(s)
	if len(r) > w {
		return string(r[:w])
	}
	return s + strings.Repeat(" ", w-len(r))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
