package danmaku

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"whale-live/internal/model"
)

// Samples are the local demo lines. The first one matches the architecture diagram.
var Samples = []model.Comment{
	{User: "小鱼干", Text: "小鲸鱼，帮我讲讲这段代码", Source: "mock"},
	{User: "阿橙", Text: "这个 nil pointer 怎么回事呀", Source: "mock"},
	{User: "南风", Text: "for 循环和 range 哪个好", Source: "mock"},
	{User: "晚晚", Text: "递归怎么才能看懂", Source: "mock"},
	{User: "木木", Text: "channel 阻塞是什么意思", Source: "mock"},
	{User: "小满", Text: "goroutine 会不会把变量抢坏", Source: "mock"},
	{User: "阿梨", Text: "小鲸鱼下午好，今天好可爱", Source: "mock"},
}

type Submitter interface {
	Submit(c model.Comment) error
}

// Simulator feeds the pipeline with sample comments for a local rehearsal.
type Simulator struct {
	mu       sync.Mutex
	sub      Submitter
	log      *slog.Logger
	cancel   context.CancelFunc
	running  bool
	interval time.Duration
	next     int
}

func NewSimulator(sub Submitter, interval time.Duration, log *slog.Logger) *Simulator {
	if interval <= 0 {
		interval = 12 * time.Second
	}
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Simulator{sub: sub, interval: interval, log: log}
}

func (s *Simulator) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *Simulator) Interval() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.interval
}

// Start begins periodic sample comments. The first one is sent immediately.
func (s *Simulator) Start(interval time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return
	}
	if interval > 0 {
		s.interval = interval
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.running = true
	go s.loop(ctx, s.interval)
}

func (s *Simulator) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.running = false
}

func (s *Simulator) Once(user, text string) (model.Comment, error) {
	c := s.pick(user, text)
	err := s.sub.Submit(c)
	return c, err
}

func (s *Simulator) loop(ctx context.Context, every time.Duration) {
	s.fire()
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.fire()
		}
	}
}

func (s *Simulator) fire() {
	c := s.pick("", "")
	if err := s.sub.Submit(c); err != nil {
		s.log.Warn("mock submit", "err", err, "text", c.Text)
	}
}

func (s *Simulator) pick(user, text string) model.Comment {
	if strings.TrimSpace(text) != "" {
		if strings.TrimSpace(user) == "" {
			user = "小鱼干"
		}
		return model.Comment{User: strings.TrimSpace(user), Text: strings.TrimSpace(text), Source: "mock"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c := Samples[s.next%len(Samples)]
	s.next++
	return c
}
