package pipeline

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"whale-live/internal/brain"
	"whale-live/internal/hub"
	"whale-live/internal/model"
	"whale-live/internal/speak"
	"whale-live/internal/store"
)

var (
	ErrEmpty = errors.New("empty danmaku")
	ErrFull  = errors.New("queue full")
)

// Snapshot is the pet-facing state. The page renders idle, thinking, or speaking.
type Snapshot struct {
	Type       string `json:"type"`
	State      string `json:"state"`
	User       string `json:"user,omitempty"`
	Danmaku    string `json:"danmaku,omitempty"`
	Reply      string `json:"reply,omitempty"`
	AudioURL   string `json:"audioUrl,omitempty"`
	Mock       bool   `json:"mock"`
	Degraded   bool   `json:"degraded"`
	DurationMs int64  `json:"durationMs"`
	Queue      int    `json:"queue"`
	StartedAt  string `json:"startedAt,omitempty"`
	Character  string `json:"character"`
}

type Saver interface {
	Save(ctx context.Context, rec store.Record) error
}

// VoiceSynth turns reply text into a playable clip. Optional.
type VoiceSynth interface {
	Speak(ctx context.Context, id, text string) (audioURL string, dur time.Duration, err error)
}

type Options struct {
	Hub          *hub.Hub
	Brain        brain.Brain
	Store        Saver
	Voice        VoiceSynth
	Log          *slog.Logger
	Name         string
	QueueSize    int
	SpeakPerRune int
	SpeakMinMs   int
	SpeakMaxMs   int
	BrainTimeout time.Duration
}

// Pipeline speaks one comment at a time and broadcasts each state change.
type Pipeline struct {
	queue   chan model.Comment
	hub     *hub.Hub
	brain   brain.Brain
	store   Saver
	voice   VoiceSynth
	log     *slog.Logger
	name    string
	per     int
	min     int
	max     int
	timeout time.Duration

	mu   sync.RWMutex
	snap Snapshot
}

func New(opt Options) *Pipeline {
	if opt.QueueSize < 1 {
		opt.QueueSize = 32
	}
	if opt.Name == "" {
		opt.Name = "小鲸鱼"
	}
	if opt.BrainTimeout <= 0 {
		opt.BrainTimeout = 25 * time.Second
	}
	if opt.Log == nil {
		opt.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if opt.Brain == nil {
		opt.Brain = brain.Local{}
	}
	p := &Pipeline{
		queue:   make(chan model.Comment, opt.QueueSize),
		hub:     opt.Hub,
		brain:   opt.Brain,
		store:   opt.Store,
		voice:   opt.Voice,
		log:     opt.Log,
		name:    opt.Name,
		per:     opt.SpeakPerRune,
		min:     opt.SpeakMinMs,
		max:     opt.SpeakMaxMs,
		timeout: opt.BrainTimeout,
	}
	p.snap = Snapshot{Type: "state", State: "idle", Character: p.name}
	return p
}

func (p *Pipeline) Submit(c model.Comment) error {
	c.Text = strings.TrimSpace(c.Text)
	if c.Text == "" {
		return ErrEmpty
	}
	if strings.TrimSpace(c.User) == "" {
		c.User = "观众"
	}
	if strings.TrimSpace(c.Source) == "" {
		c.Source = "api"
	}
	select {
	case p.queue <- c:
		p.publish(p.currentLockedUpdate())
		p.publishDanmaku(c)
		return nil
	default:
		return ErrFull
	}
}

func (p *Pipeline) publishDanmaku(c model.Comment) {
	if p.hub == nil {
		return
	}
	p.hub.Publish(map[string]any{
		"type":   "danmaku",
		"user":   c.User,
		"text":   c.Text,
		"source": c.Source,
		"at":     time.Now().Format(time.RFC3339Nano),
	})
}

func (p *Pipeline) currentLockedUpdate() Snapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.snap.Queue = len(p.queue)
	return p.snap
}

func (p *Pipeline) Snapshot() Snapshot {
	p.mu.RLock()
	s := p.snap
	p.mu.RUnlock()
	s.Queue = len(p.queue)
	return s
}

func (p *Pipeline) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-p.queue:
			if !p.handle(ctx, job) {
				return
			}
		}
	}
}

func (p *Pipeline) handle(ctx context.Context, job model.Comment) bool {
	p.set(Snapshot{
		State:     "thinking",
		User:      job.User,
		Danmaku:   job.Text,
		StartedAt: time.Now().Format(time.RFC3339Nano),
	})
	rctx, cancel := context.WithTimeout(ctx, p.timeout)
	res := p.brain.Reply(rctx, job.User, job.Text)
	cancel()
	if ctx.Err() != nil {
		return false
	}
	reply := strings.TrimSpace(res.Reply)
	if reply == "" {
		reply = brain.Fallback(job.User)
		res.Mock = true
		res.Degraded = true
	}
	if res.Err != nil {
		p.log.Warn("brain", "err", res.Err, "degraded", true)
	}
	dur := speak.Duration(reply, p.per, p.min, p.max)
	audioURL := ""
	if p.voice != nil {
		id := fmt.Sprintf("%d", time.Now().UnixNano())
		vctx, vcancel := context.WithTimeout(ctx, 50*time.Second)
		url, vd, verr := p.voice.Speak(vctx, id, reply)
		vcancel()
		if verr != nil {
			p.log.Warn("tts", "err", verr)
		} else {
			audioURL = url
			if vd > 0 {
				// Pad a little so the mouth does not freeze before audio ends.
				dur = vd + 250*time.Millisecond
				if dur < time.Duration(p.min)*time.Millisecond {
					dur = time.Duration(p.min) * time.Millisecond
				}
			}
		}
	}
	started := time.Now()
	p.set(Snapshot{
		State:      "speaking",
		User:       job.User,
		Danmaku:    job.Text,
		Reply:      reply,
		AudioURL:   audioURL,
		Mock:       res.Mock,
		Degraded:   res.Degraded,
		DurationMs: dur.Milliseconds(),
		StartedAt:  started.Format(time.RFC3339Nano),
	})
	p.log.Info("reply", "user", job.User, "mock", res.Mock, "degraded", res.Degraded, "reply", reply)
	if p.store != nil {
		sctx, scancel := context.WithTimeout(context.Background(), 3*time.Second)
		err := p.store.Save(sctx, store.Record{
			Time:     started,
			User:     job.User,
			Source:   job.Source,
			Danmaku:  job.Text,
			Reply:    reply,
			Mock:     res.Mock,
			Degraded: res.Degraded,
		})
		scancel()
		if err != nil {
			p.log.Warn("store", "err", err)
		}
	}
	timer := time.NewTimer(dur)
	select {
	case <-ctx.Done():
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		return false
	case <-timer.C:
	}
	p.set(Snapshot{State: "idle"})
	return true
}

func (p *Pipeline) set(s Snapshot) {
	s.Type = "state"
	s.Character = p.name
	s.Queue = len(p.queue)
	p.mu.Lock()
	p.snap = s
	p.mu.Unlock()
	p.publish(s)
}

func (p *Pipeline) publish(s Snapshot) {
	if p.hub != nil {
		p.hub.Publish(s)
	}
}
