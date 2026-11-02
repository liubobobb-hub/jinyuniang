package speak

import (
	"strings"
	"time"
	"unicode/utf8"
)

// Duration estimates how long the pet should keep its mouth moving.
// The queue waits this long before accepting the next line, so the
// on-stream read finishes before the next reply starts.
func Duration(text string, perRune, minMs, maxMs int) time.Duration {
	if perRune <= 0 {
		perRune = 240
	}
	if minMs <= 0 {
		minMs = 2800
	}
	if maxMs < minMs {
		maxMs = minMs
	}
	n := utf8.RuneCountInString(strings.TrimSpace(text))
	if n == 0 {
		n = 1
	}
	ms := n * perRune
	if ms < minMs {
		ms = minMs
	}
	if ms > maxMs {
		ms = maxMs
	}
	return time.Duration(ms) * time.Millisecond
}
