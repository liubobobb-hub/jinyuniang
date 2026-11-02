package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logs", "interactions.jsonl")
	f, err := OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rec := Record{
		Time:    time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC),
		User:    "小鱼干",
		Source:  "test",
		Danmaku: "小鲸鱼，帮我讲讲这段代码",
		Reply:   "先看输入，再看输出。",
		Mock:    true,
	}
	if err := f.Save(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := ReadTail(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Reply != rec.Reply || got[0].User != rec.User {
		t.Fatalf("tail = %+v", got)
	}
	mem := NewMemory(10)
	for _, item := range got {
		if err := mem.Save(context.Background(), item); err != nil {
			t.Fatal(err)
		}
	}
	recent := mem.Recent(5)
	if len(recent) != 1 || recent[0].Danmaku != rec.Danmaku {
		t.Fatalf("recent = %+v", recent)
	}
}
