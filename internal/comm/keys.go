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
	case 0:
		return k.doorway()
	case 3:
		return Event{Key: KeyEsc, Raw: b}, nil
	case 8, 127:
		return Event{Key: KeyBackspace, Raw: b}, nil
	case 9:
		return Event{Key: KeyTab, Raw: b}, nil
	case 10, 13:
		if b == 13 {
			k.peekEat(20*time.Millisecond, 10, 0)
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

// peekEat swallows the next byte if it is one of want (CR LF, telnet CR NUL).
func (k *Keyboard) peekEat(d time.Duration, want ...byte) {
	b, err := k.readByte(d)
	if err != nil {
		return
	}
	for _, w := range want {
		if b == w {
			return
		}
	}
	k.buf = append([]byte{b}, k.buf...)
}

// doorway decodes DOS "doorway mode" keys: NUL followed by a BIOS scan code.
func (k *Keyboard) doorway() (Event, error) {
	sc, err := k.readByte(100 * time.Millisecond)
	if err != nil {
		return Event{Key: KeyNone}, nil
	}
	switch sc {
	case 0x48:
		return Event{Key: KeyUp}, nil
	case 0x50:
		return Event{Key: KeyDown}, nil
	case 0x4B:
		return Event{Key: KeyLeft}, nil
	case 0x4D:
		return Event{Key: KeyRight}, nil
	case 0x47:
		return Event{Key: KeyHome}, nil
	case 0x4F:
		return Event{Key: KeyEnd}, nil
	case 0x49:
		return Event{Key: KeyPgUp}, nil
	case 0x51:
		return Event{Key: KeyPgDn}, nil
	}
	return Event{Key: KeyNone, Raw: sc}, nil
}

func (k *Keyboard) escape(idle time.Duration) (Event, error) {
	b, err := k.readByte(300 * time.Millisecond)
	if err != nil {
		return Event{Key: KeyEsc, Raw: 27}, nil
	}
	if b == '[' || b == 'O' {
		seq := []byte{b}
		for i := 0; i < 8; i++ {
			nb, err := k.readByte(200 * time.Millisecond)
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
	case 'F', 'K':
		// ESC[K is End in ANSI-BBS terminals (SyncTERM, NetRunner).
		return Event{Key: KeyEnd}
	case 'V':
		return Event{Key: KeyPgUp}
	case 'U':
		return Event{Key: KeyPgDn}
	case '~':
		switch csiParam(seq) {
		case 1, 7:
			return Event{Key: KeyHome}
		case 4, 8:
			return Event{Key: KeyEnd}
		case 5:
			return Event{Key: KeyPgUp}
		case 6:
			return Event{Key: KeyPgDn}
		}
	}
	return Event{Key: KeyNone, Raw: 27}
}

// csiParam returns the first numeric parameter of a CSI sequence ("[5;2~" -> 5).
func csiParam(seq []byte) int {
	n := 0
	for _, c := range seq[1:] {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
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
