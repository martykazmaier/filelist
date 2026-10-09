package ui

import (
	"errors"
	"fmt"
	"os"
	"time"

	"filelist/internal/comm"
)

var errIdle = errors.New("inactivity timeout")

const idleWarnBefore = 30 * time.Second

// nextKey waits for a key under the CONFIG.RA UserTimeOut, warning the
// caller 30 seconds before the limit and hanging up when it runs out.
func (a *App) nextKey() (comm.Event, error) {
	if a.idled {
		return comm.Event{Key: comm.KeyHangup}, errIdle
	}
	if a.Idle <= 0 {
		for {
			ev, err := a.Keys.Next(0)
			if err != nil || ev.Key != comm.KeyNone {
				return ev, err
			}
		}
	}
	limit := time.Now().Add(a.Idle)
	warned := a.Idle <= idleWarnBefore
	for {
		wait := time.Until(limit)
		if !warned {
			wait -= idleWarnBefore
		}
		if wait <= 0 {
			wait = time.Millisecond
		}
		ev, err := a.Keys.Next(wait)
		if errors.Is(err, os.ErrDeadlineExceeded) {
			if !warned {
				warned = true
				a.idleWarn()
				continue
			}
			a.idleHangup()
			return comm.Event{Key: comm.KeyHangup}, errIdle
		}
		if err != nil || ev.Key != comm.KeyNone {
			return ev, err
		}
	}
}

func (a *App) idleWarn() {
	_, _ = fmt.Fprintf(a.Out, "\x1b[%d;1H\x1b[0;1;37;41m%s\x1b[0m\a\a", rowStatus,
		padVis(" You are about to be disconnected for inactivity!", screenW))
}

func (a *App) idleHangup() {
	a.idled = true
	_, _ = fmt.Fprintf(a.Out, "\x1b[%d;1H\x1b[0;1;37;41m%s\x1b[0m\r\n", rowStatus,
		padVis(" * User Inactivity Timeout, Disconnecting *", screenW))
	time.Sleep(time.Second)
	if a.Hangup != nil {
		a.Hangup()
	}
}
