package comm

import (
	"io"
	"time"
	"unicode/utf8"
)

type Key int

const (
	KeyNone Key = iota
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyHome
	KeyEnd
	KeyPgUp
	KeyPgDn
	KeyEsc
	KeyEnter
	KeySpace
	KeyBackspace
	KeyTab
	KeyChar
	KeyHangup
)

type Event struct {
	Key  Key
	Rune rune
	Raw  byte
}

type Keyboard struct {
	r      io.Reader
	buf    []byte
	iac    bool
	iacCmd byte
}

func NewKeyboard(r io.Reader) *Keyboard {
	return &Keyboard{r: r}
}

func (k *Keyboard) Next(idle time.Duration) (Event, error) {
	b, err := k.readByte(idle)
	if err != nil {
		return Event{Key: KeyHangup}, err
	}

	if b == 255 {
		return k.eatIAC(idle)
	}

	switch b {
	case 3:
		return Event{Key: KeyEsc, Raw: b}, nil
	case 8, 127:
		return Event{Key: KeyBackspace, Raw: b}, nil
	case 9:
		return Event{Key: KeyTab, Raw: b}, nil
	case 10, 13:
		if b == 13 {
			k.peekEat(10, 20*time.Millisecond)
		}
		return Event{Key: KeyEnter, Raw: b}, nil
	case 32:
		return Event{Key: KeySpace, Raw: b}, nil
	case 27:
		return k.escape(idle)
	}

	if b < 32 {
		return Event{Key: KeyNone, Raw: b}, nil
	}

	if b < 0x80 {
		return Event{Key: KeyChar, Rune: rune(b), Raw: b}, nil
	}

	return Event{Key: KeyChar, Rune: rune(b), Raw: b}, nil
}

func (k *Keyboard) readByte(idle time.Duration) (byte, error) {
	if len(k.buf) > 0 {
		b := k.buf[0]
		k.buf = k.buf[1:]
		return b, nil
	}
	tmp := make([]byte, 64)
	if dl, ok := k.r.(interface{ SetReadDeadline(time.Time) error }); ok && idle > 0 {
		_ = dl.SetReadDeadline(time.Now().Add(idle))
	}
	n, err := k.r.Read(tmp)
	if dl, ok := k.r.(interface{ SetReadDeadline(time.Time) error }); ok {
		_ = dl.SetReadDeadline(time.Time{})
	}
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, io.EOF
	}
	k.buf = append(k.buf, tmp[1:n]...)
	return tmp[0], nil
}

func (k *Keyboard) peekEat(want byte, d time.Duration) {
	b, err := k.readByte(d)
	if err != nil {
		return
	}
	if b != want {
		k.buf = append([]byte{b}, k.buf...)
	}
}

func (k *Keyboard) escape(idle time.Duration) (Event, error) {
	b, err := k.readByte(150 * time.Millisecond)
	if err != nil {
		return Event{Key: KeyEsc, Raw: 27}, nil
	}
	if b == '[' || b == 'O' {
		seq := []byte{b}
		for i := 0; i < 8; i++ {
			nb, err := k.readByte(80 * time.Millisecond)
			if err != nil {
				break
			}
			seq = append(seq, nb)
			if (nb >= 'A' && nb <= 'Z') || (nb >= 'a' && nb <= 'z') || nb == '~' {
				break
			}
		}
		return decodeCSI(seq), nil
	}
	k.buf = append([]byte{b}, k.buf...)
	return Event{Key: KeyEsc, Raw: 27}, nil
}

func decodeCSI(seq []byte) Event {
	if len(seq) == 0 {
		return Event{Key: KeyNone, Raw: 27}
	}
	last := seq[len(seq)-1]
	switch last {
	case 'A':
		return Event{Key: KeyUp}
	case 'B':
		return Event{Key: KeyDown}
	case 'C':
		return Event{Key: KeyRight}
	case 'D':
		return Event{Key: KeyLeft}
	case 'H':
		return Event{Key: KeyHome}
	case 'F':
		return Event{Key: KeyEnd}
	case '~':
		s := string(seq)
		switch {
		case containsNum(s, '1'), containsNum(s, '7'):
			return Event{Key: KeyHome}
		case containsNum(s, '4'), containsNum(s, '8'):
			return Event{Key: KeyEnd}
		case containsNum(s, '5'):
			return Event{Key: KeyPgUp}
		case containsNum(s, '6'):
			return Event{Key: KeyPgDn}
		}
	}
	return Event{Key: KeyNone, Raw: 27}
}

func containsNum(s string, d byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == d {
			return true
		}
	}
	return false
}

func (k *Keyboard) eatIAC(idle time.Duration) (Event, error) {
	cmd, err := k.readByte(idle)
	if err != nil {
		return Event{Key: KeyHangup}, err
	}
	if cmd == 255 {
		return Event{Key: KeyChar, Rune: 255, Raw: 255}, nil
	}
	if cmd == 250 {
		for {
			b, err := k.readByte(idle)
			if err != nil {
				return Event{Key: KeyHangup}, err
			}
			if b == 240 {
				break
			}
			if b == 255 {
				_, _ = k.readByte(idle)
			}
		}
		return Event{Key: KeyNone}, nil
	}
	if cmd >= 251 && cmd <= 254 {
		_, _ = k.readByte(idle)
		return Event{Key: KeyNone}, nil
	}
	return Event{Key: KeyNone, Raw: cmd}, nil
}

func KeyCharEquals(ev Event, ch rune) bool {
	if ev.Key != KeyChar {
		return false
	}
	r := ev.Rune
	if r >= 'a' && r <= 'z' {
		r -= 32
	}
	if ch >= 'a' && ch <= 'z' {
		ch -= 32
	}
	return r == ch
}

func DecodeRune(b byte) rune {
	if b < utf8.RuneSelf {
		return rune(b)
	}
	return rune(b)
}
