package textutil

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Clean trims surrounding space, drops control characters, and collapses gaps.
func Clean(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t':
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

// Clip limits s to at most n runes.
func Clip(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// ClipSentence limits s to n runes, preferring to end on a full stop.
func ClipSentence(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if n <= 0 || len(r) <= n {
		return string(r)
	}
	for i := n; i >= n/2; i-- {
		switch r[i-1] {
		case '。', '！', '？', '.', '!', '?':
			return strings.TrimSpace(string(r[:i]))
		}
	}
	return strings.TrimSpace(string(r[:n]))
}
