package store

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Record is one danmaku/reply pair kept for the studio history and optional MySQL.
type Record struct {
	Time     time.Time `json:"time"`
	User     string    `json:"user"`
	Source   string    `json:"source"`
	Danmaku  string    `json:"danmaku"`
	Reply    string    `json:"reply"`
	Mock     bool      `json:"mock"`
	Degraded bool      `json:"degraded"`
}

// Memory is a small ring of recent interactions. It is always on,
// so the demo works with no database.
type Memory struct {
	mu  sync.Mutex
	buf []Record
	max int
}

func NewMemory(max int) *Memory {
	if max < 1 {
		max = 50
	}
	return &Memory{max: max}
}

func (m *Memory) Save(_ context.Context, r Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buf = append(m.buf, r)
	if len(m.buf) > m.max {
		m.buf = append([]Record(nil), m.buf[len(m.buf)-m.max:]...)
	}
	return nil
}

// Recent returns up to n records, newest first.
func (m *Memory) Recent(n int) []Record {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n <= 0 || n > len(m.buf) {
		n = len(m.buf)
	}
	out := make([]Record, n)
	for i := 0; i < n; i++ {
		out[i] = m.buf[len(m.buf)-1-i]
	}
	return out
}

// File appends JSON lines. Reads still come from Memory.
type File struct {
	mu sync.Mutex
	f  *os.File
}

func OpenFile(path string) (*File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &File{f: f}, nil
}

func (f *File) Save(_ context.Context, r Record) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	_, err = f.f.Write(append(b, '\n'))
	return err
}

func (f *File) Close() error { return f.f.Close() }

// ReadTail loads the last max complete JSON lines, oldest first.
// Only the trailing 256KiB is read so a long-running log cannot stall startup.
func ReadTail(path string, max int) ([]Record, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	const window = 256 * 1024
	start := int64(0)
	if info.Size() > window {
		start = info.Size() - window
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var lines [][]byte
	first := start > 0
	for sc.Scan() {
		line := append([]byte(nil), sc.Bytes()...)
		if first {
			// The window may begin mid-line.
			first = false
			continue
		}
		if len(bytesTrim(line)) == 0 {
			continue
		}
		lines = append(lines, line)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(lines) > max && max > 0 {
		lines = lines[len(lines)-max:]
	}
	out := make([]Record, 0, len(lines))
	for _, line := range lines {
		var rec Record
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}

func bytesTrim(b []byte) []byte {
	i := 0
	j := len(b)
	for i < j && (b[i] == ' ' || b[i] == '\t') {
		i++
	}
	for j > i && (b[j-1] == ' ' || b[j-1] == '\t' || b[j-1] == '\r') {
		j--
	}
	return b[i:j]
}

// Bundle writes to memory first, then to every extra sink (file, MySQL).
type Bundle struct {
	mem   *Memory
	extra []interface {
		Save(context.Context, Record) error
	}
}

func NewBundle(mem *Memory) *Bundle {
	if mem == nil {
		mem = NewMemory(100)
	}
	return &Bundle{mem: mem}
}

func (b *Bundle) Add(s interface {
	Save(context.Context, Record) error
}) {
	if s != nil {
		b.extra = append(b.extra, s)
	}
}

func (b *Bundle) Save(ctx context.Context, r Record) error {
	if err := b.mem.Save(ctx, r); err != nil {
		return err
	}
	var first error
	for _, extra := range b.extra {
		if err := extra.Save(ctx, r); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (b *Bundle) Recent(n int) []Record { return b.mem.Recent(n) }
