package elebbs

import (
	"encoding/binary"
	"unicode/utf8"
)

func pascalString(buf []byte) string {
	if len(buf) == 0 {
		return ""
	}
	n := int(buf[0])
	if n > len(buf)-1 {
		n = len(buf) - 1
	}
	return cp437ToString(buf[1 : 1+n])
}

func putPascal(buf []byte, s string) {
	if len(buf) == 0 {
		return
	}
	raw := stringToCP437(s)
	max := len(buf) - 1
	if len(raw) > max {
		raw = raw[:max]
	}
	buf[0] = byte(len(raw))
	copy(buf[1:], raw)
	for i := 1 + len(raw); i < len(buf); i++ {
		buf[i] = 0
	}
}

func pascalBytes(buf []byte) []byte {
	if len(buf) == 0 {
		return nil
	}
	n := int(buf[0])
	if n > len(buf)-1 {
		n = len(buf) - 1
	}
	out := make([]byte, n)
	copy(out, buf[1:1+n])
	return out
}

func u16(b []byte, off int) uint16 {
	if off+1 >= len(b) {
		return 0
	}
	return binary.LittleEndian.Uint16(b[off:])
}

func u32(b []byte, off int) uint32 {
	if off+3 >= len(b) {
		return 0
	}
	return binary.LittleEndian.Uint32(b[off:])
}

func i32(b []byte, off int) int32 {
	return int32(u32(b, off))
}

func putU16(b []byte, off int, v uint16) {
	binary.LittleEndian.PutUint16(b[off:], v)
}

func putU32(b []byte, off int, v uint32) {
	binary.LittleEndian.PutUint32(b[off:], v)
}

func cp437ToString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	r := make([]rune, 0, len(b))
	for _, c := range b {
		if c < 0x80 {
			r = append(r, rune(c))
			continue
		}
		r = append(r, cp437Table[c-0x80])
	}
	return string(r)
}

func stringToCP437(s string) []byte {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		if r < 0x80 {
			out = append(out, byte(r))
			continue
		}
		if b, ok := unicodeToCP437[r]; ok {
			out = append(out, b)
			continue
		}
		if utf8.RuneLen(r) == 1 && r <= 0xFF {
			out = append(out, byte(r))
			continue
		}
		out = append(out, '?')
	}
	return out
}

func truncateCP437(b []byte, width int, more byte) []byte {
	if width <= 0 {
		return nil
	}
	if len(b) <= width {
		out := make([]byte, width)
		copy(out, b)
		for i := len(b); i < width; i++ {
			out[i] = ' '
		}
		return out
	}
	out := make([]byte, width)
	copy(out, b[:width])
	if width >= 1 {
		out[width-1] = more
	}
	return out
}

func PadCP437(s string, width int) []byte {
	return truncateCP437(stringToCP437(s), width, '.')
}

func PadName(s string, width int) []byte {
	return truncateCP437(stringToCP437(s), width, '>')
}
