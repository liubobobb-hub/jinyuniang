package pipeline

import (
	"context"
	"sync"
	"testing"
	"time"

	"whale-live/internal/brain"
	"whale-live/internal/model"
	"whale-live/internal/store"
)

type gateBrain struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
	cur     int
	max     int
	mu      sync.Mutex
}

func (g *gateBrain) Mode() string { return "mock" }

func (g *gateBrain) Reply(ctx context.Context, user, text string) brain.Result {
	g.mu.Lock()
	g.cur++
	if g.cur > g.max {
		g.max = g.cur
	}
	g.mu.Unlock()
	g.once.Do(func() { close(g.started) })
	select {
	case <-g.release:
	case <-ctx.Done():
	}
	g.mu.Lock()
	g.cur--
	g.mu.Unlock()
	return brain.Result{Reply: "好，先看输入再看输出。", Mock: true}
}

func TestOneAtATimeAndHistory(t *testing.T) {
	mem := store.NewBundle(store.NewMemory(10))
	g := &gateBrain{started: make(chan struct{}), release: make(chan struct{})}
	p := New(Options{
		Brain:        g,
		Store:        mem,
		QueueSize:    1,
		SpeakPerRune: 1,
		SpeakMinMs:   30,
		SpeakMaxMs:   60,
		BrainTimeout: time.Second,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go p.Run(ctx)

	if err := p.Submit(model.Comment{User: "小鱼干", Text: "小鲸鱼，帮我讲讲这段代码", Source: "test"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-g.started:
	case <-time.After(2 * time.Second):
		t.Fatal("brain did not start")
	}
	if err := p.Submit(model.Comment{User: "b", Text: "第二句", Source: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := p.Submit(model.Comment{User: "c", Text: "第三句", Source: "test"}); !errorsIsFull(err) {
		t.Fatalf("third submit err = %v, want full", err)
	}
	close(g.release)

	deadline := time.Now().Add(3 * time.Second)
	var sawSpeak bool
	for time.Now().Before(deadline) {
		s := p.Snapshot()
		if s.State == "speaking" && s.Reply != "" {
			sawSpeak = true
		}
		if sawSpeak && s.State == "idle" && len(mem.Recent(10)) >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !sawSpeak {
		t.Fatalf("never spoke: %+v", p.Snapshot())
	}
	recent := mem.Recent(10)
	if len(recent) < 2 {
		t.Fatalf("history = %+v", recent)
	}
	g.mu.Lock()
	max := g.max
	g.mu.Unlock()
	if max != 1 {
		t.Fatalf("concurrent brains = %d", max)
	}
}

func errorsIsFull(err error) bool {
	return err == ErrFull
}

func TestLocalReplyReachesIdle(t *testing.T) {
	p := New(Options{
		Brain:        brain.Local{},
		SpeakPerRune: 1,
		SpeakMinMs:   80,
		SpeakMaxMs:   150,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go p.Run(ctx)
	if err := p.Submit(model.Comment{User: "阿梨", Text: "小鲸鱼下午好", Source: "test"}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s := p.Snapshot()
		if s.State == "idle" && s.Reply == "" {
			// May still be the initial idle. Wait until we have passed speaking.
		}
		if s.State == "speaking" {
			deadline = time.Now().Add(time.Second)
			for time.Now().Before(deadline) {
				if p.Snapshot().State == "idle" {
					return
				}
				time.Sleep(5 * time.Millisecond)
			}
			t.Fatal("stuck speaking")
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("snapshot = %+v", p.Snapshot())
}
