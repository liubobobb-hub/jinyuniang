package speak

import (
	"testing"
	"time"
)

func TestDurationBounds(t *testing.T) {
	short := Duration("嗨", 220, 2800, 14000)
	if short != 2800*time.Millisecond {
		t.Fatalf("short = %s", short)
	}
	longText := ""
	for i := 0; i < 200; i++ {
		longText += "字"
	}
	long := Duration(longText, 220, 2800, 14000)
	if long != 14000*time.Millisecond {
		t.Fatalf("long = %s", long)
	}
	mid := Duration("一二三四五六七八九十", 100, 100, 5000)
	if mid != 1000*time.Millisecond {
		t.Fatalf("mid = %s, want 1s", mid)
	}
}
