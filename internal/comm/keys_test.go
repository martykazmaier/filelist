package comm

import (
	"bytes"
	"testing"
)

func TestKeyboardNavigationKeys(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want Key
	}{
		{"vt home", "\x1b[H", KeyHome},
		{"vt end", "\x1b[F", KeyEnd},
		{"ss3 home", "\x1bOH", KeyHome},
		{"ss3 end", "\x1bOF", KeyEnd},
		{"xterm home", "\x1b[1~", KeyHome},
		{"rxvt home", "\x1b[7~", KeyHome},
		{"xterm end", "\x1b[4~", KeyEnd},
		{"rxvt end", "\x1b[8~", KeyEnd},
		{"pgup", "\x1b[5~", KeyPgUp},
		{"pgdn", "\x1b[6~", KeyPgDn},
		{"ansi-bbs end", "\x1b[K", KeyEnd},
		{"ansi-bbs pgup", "\x1b[V", KeyPgUp},
		{"ansi-bbs pgdn", "\x1b[U", KeyPgDn},
		{"doorway home", "\x00\x47", KeyHome},
		{"doorway end", "\x00\x4F", KeyEnd},
		{"doorway pgup", "\x00\x49", KeyPgUp},
		{"doorway pgdn", "\x00\x51", KeyPgDn},
		{"f5 is not home", "\x1b[15~", KeyNone},
		{"enter cr lf", "\r\n", KeyEnter},
		{"enter cr nul", "\r\x00", KeyEnter},
	}
	for _, c := range cases {
		k := NewKeyboard(bytes.NewReader([]byte(c.in + "x")))
		ev, err := k.Next(0)
		if err != nil || ev.Key != c.want {
			t.Errorf("%s: got %v err %v, want %v", c.name, ev.Key, err, c.want)
			continue
		}
		if ev, _ := k.Next(0); !KeyCharEquals(ev, 'x') {
			t.Errorf("%s: trailing bytes not consumed cleanly: %+v", c.name, ev)
		}
	}
}
