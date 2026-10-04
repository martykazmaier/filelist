package elebbs

import (
	"regexp"
	"strings"
	"time"
)

func FileStamp(v uint32) time.Time {
	if v == 0 {
		return time.Time{}
	}
	now := time.Now()
	latest := now.Add(36 * time.Hour)
	var best time.Time
	consider := func(t time.Time) {
		if t.IsZero() || t.After(latest) || t.Year() < 1980 {
			return
		}
		if best.IsZero() || closer(now, t, best) {
			best = t
		}
	}
	// DOS packed dates live in the low word. Don't also treat that
	// small value as Unix or as a FAT high-word.
	if v <= 0xFFFF {
		consider(packedYear(v, true))
		consider(packedYear(v, false))
		if !best.IsZero() {
			return best
		}
	}
	if v > 100000000 && v < 4000000000 {
		consider(time.Unix(int64(v), 0))
	}
	consider(packedYear(v, true))
	consider(packedYear(v, false))
	if v > 0xFFFF {
		consider(packedYear(v>>16, true))
		consider(packedYear(v>>16, false))
	}
	consider(numericStamp(v))
	return best
}

func closer(now, a, b time.Time) bool {
	da := now.Sub(a)
	if da < 0 {
		da = -da
	}
	db := now.Sub(b)
	if db < 0 {
		db = -db
	}
	return da < db
}

func packedYear(v uint32, twoDigit bool) time.Time {
	if v == 0 || v > 0x1FFFF {
		return time.Time{}
	}
	day := int(v & 0x1F)
	month := int((v >> 5) & 0x0F)
	y := int((v >> 9) & 0x7F)
	if month < 1 || month > 12 || day < 1 || day > 31 {
		return time.Time{}
	}
	year := 1980 + y
	if twoDigit {
		year = twoDigitYear(y)
	}
	return validYMD(year, month, day)
}

func numericStamp(v uint32) time.Time {
	if v >= 19800101 && v <= 20991231 {
		year := int(v / 10000)
		month := int((v / 100) % 100)
		day := int(v % 100)
		return validYMD(year, month, day)
	}
	if v >= 10101 && v <= 991231 {
		yy := int(v / 10000)
		month := int((v / 100) % 100)
		day := int(v % 100)
		return validYMD(twoDigitYear(yy), month, day)
	}
	return time.Time{}
}

func twoDigitYear(y int) int {
	if y < 0 {
		return y
	}
	if y > 99 {
		return 1980 + y
	}
	if y >= 80 {
		return 1900 + y
	}
	return 2000 + y
}

func validYMD(year, month, day int) time.Time {
	if year < 1980 || year > 2099 || month < 1 || month > 12 || day < 1 || day > 31 {
		return time.Time{}
	}
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
	if t.Year() != year || t.Month() != time.Month(month) || t.Day() != day {
		return time.Time{}
	}
	return t
}

func MatchDays(e FileEntry, days int) bool {
	if days < 1 {
		return false
	}
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	cutoff := midnight.AddDate(0, 0, -(days - 1))
	latest := now.Add(36 * time.Hour)
	t := FileStamp(e.Hdr.FileDate)
	if t.IsZero() {
		t = FileStamp(e.Hdr.UploadDate)
	}
	if t.IsZero() || t.After(latest) {
		return false
	}
	return !t.Before(cutoff)
}

func MatchKeyword(e FileEntry, key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" {
		return false
	}
	return strings.Contains(strings.ToLower(e.DisplayName), key) ||
		strings.Contains(strings.ToLower(e.ShortName), key) ||
		strings.Contains(strings.ToLower(e.Description), key)
}

func MatchWild(e FileEntry, pat string) bool {
	pat = strings.TrimSpace(pat)
	if pat == "" {
		return false
	}
	re := globRegexp(pat)
	if re == nil {
		return false
	}
	return re.MatchString(e.DisplayName) || re.MatchString(e.ShortName)
}

func globRegexp(pat string) *regexp.Regexp {
	if !strings.ContainsAny(pat, "*?") {
		pat = "*" + pat + "*"
	}
	var b strings.Builder
	b.WriteString("(?i)^")
	for i := 0; i < len(pat); i++ {
		switch pat[i] {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteByte('.')
		default:
			b.WriteString(regexp.QuoteMeta(string(pat[i])))
		}
	}
	b.WriteByte('$')
	re, err := regexp.Compile(b.String())
	if err != nil {
		return nil
	}
	return re
}
