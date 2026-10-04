package ui

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"filelist/internal/comm"
	"filelist/internal/elebbs"
)

var errPromptCancel = errors.New("cancel")

func (a *App) prompt(label string) (string, error) {
	buf := make([]byte, 0, 48)
	for {
		line := label + string(buf) + "_"
		if _, err := fmt.Fprintf(a.Out, "\x1b[%d;1H\x1b[?25h\x1b[0;30;47m%s\x1b[0m", rowStatus, padVis(line, screenW)); err != nil {
			return "", err
		}
		ev, err := a.Keys.Next(5 * time.Minute)
		if err != nil || ev.Key == comm.KeyHangup {
			return "", io.EOF
		}
		switch ev.Key {
		case comm.KeyNone:
			continue
		case comm.KeyEsc:
			return "", errPromptCancel
		case comm.KeyEnter:
			return strings.TrimSpace(string(buf)), nil
		case comm.KeyBackspace:
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
			}
		case comm.KeySpace:
			if len(buf) < 48 {
				buf = append(buf, ' ')
			}
		case comm.KeyChar:
			ch := ev.Raw
			if ch >= 32 && ch < 127 && len(buf) < 48 {
				buf = append(buf, ch)
			}
		}
	}
}

func (a *App) promptScope() (elebbs.ScanScope, error) {
	for {
		line := "Scope: A=Area  G=Group  L=All groups  ESC=Cancel"
		if _, err := fmt.Fprintf(a.Out, "\x1b[%d;1H\x1b[?25h\x1b[0;30;47m%s\x1b[0m", rowStatus, padVis(line, screenW)); err != nil {
			return 0, err
		}
		ev, err := a.Keys.Next(5 * time.Minute)
		if err != nil || ev.Key == comm.KeyHangup {
			return 0, io.EOF
		}
		switch ev.Key {
		case comm.KeyNone:
			continue
		case comm.KeyEsc:
			return 0, errPromptCancel
		case comm.KeyChar:
			switch {
			case comm.KeyCharEquals(ev, 'A'), comm.KeyCharEquals(ev, '1'):
				return elebbs.ScopeArea, nil
			case comm.KeyCharEquals(ev, 'G'), comm.KeyCharEquals(ev, '2'):
				return elebbs.ScopeGroup, nil
			case comm.KeyCharEquals(ev, 'L'), comm.KeyCharEquals(ev, '3'):
				return elebbs.ScopeAll, nil
			}
		}
	}
}

func (a *App) searchDays() error {
	s, err := a.prompt("New files, days: ")
	if err != nil {
		return err
	}
	if s == "" {
		return errPromptCancel
	}
	days, err := strconv.Atoi(s)
	if err != nil || days < 1 || days > 3650 {
		a.flash("Enter days 1-3650")
		return errPromptCancel
	}
	scope, err := a.promptScope()
	if err != nil {
		return err
	}
	title := fmt.Sprintf("New %d days / %s", days, scope.Label())
	return a.runScan(title, scope, func(e elebbs.FileEntry) bool {
		return elebbs.MatchDays(e, days)
	})
}

func (a *App) searchKeyword() error {
	s, err := a.prompt("Keyword (name/desc): ")
	if err != nil {
		return err
	}
	if s == "" {
		return errPromptCancel
	}
	scope, err := a.promptScope()
	if err != nil {
		return err
	}
	title := fmt.Sprintf("Key %s / %s", s, scope.Label())
	key := s
	return a.runScan(title, scope, func(e elebbs.FileEntry) bool {
		return elebbs.MatchKeyword(e, key)
	})
}

func (a *App) searchWild() error {
	s, err := a.prompt("Filename (wildcards * ?): ")
	if err != nil {
		return err
	}
	if s == "" {
		return errPromptCancel
	}
	scope, err := a.promptScope()
	if err != nil {
		return err
	}
	title := fmt.Sprintf("Wild %s / %s", s, scope.Label())
	pat := s
	return a.runScan(title, scope, func(e elebbs.FileEntry) bool {
		return elebbs.MatchWild(e, pat)
	})
}

func (a *App) runScan(title string, scope elebbs.ScanScope, match func(elebbs.FileEntry) bool) error {
	_, _ = fmt.Fprintf(a.Out, "\x1b[%d;1H\x1b[?25l\x1b[0;30;47m%s\x1b[0m", rowStatus, padVis("Scanning...", screenW))
	entries := elebbs.ScanFiles(a.Ctx, scope, a.Security, match)
	a.applyListing(fmt.Sprintf("%s (%d)", title, len(entries)), entries)
	return nil
}

func (a *App) applyListing(title string, entries []elebbs.FileEntry) {
	if entries == nil {
		entries = []elebbs.FileEntry{}
	}
	a.Ctx.Entries = entries
	a.listing = title
	a.cursor = 0
	a.syncTags()
}

func (a *App) restoreArea() {
	a.Ctx.Entries = append([]elebbs.FileEntry(nil), a.home...)
	a.listing = ""
	a.cursor = 0
	a.syncTags()
}
