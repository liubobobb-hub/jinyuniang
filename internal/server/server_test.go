package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"whale-live/internal/brain"
	"whale-live/internal/config"
	"whale-live/internal/danmaku"
	"whale-live/internal/hub"
	"whale-live/internal/pipeline"
	"whale-live/internal/store"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}

type harness struct {
	ts   *httptest.Server
	pipe *pipeline.Pipeline
}

func startHarness(t *testing.T, cfg config.Config) *harness {
	t.Helper()
	if cfg.Server.WebDir == "" {
		cfg.Server.WebDir = t.TempDir()
	}
	h := hub.New()
	mem := store.NewBundle(store.NewMemory(20))
	p := pipeline.New(pipeline.Options{
		Hub:          h,
		Brain:        brain.Local{},
		Store:        mem,
		QueueSize:    cfg.Queue.MaxSize,
		SpeakPerRune: cfg.Pet.SpeakMsPerRune,
		SpeakMinMs:   cfg.Pet.SpeakMinMs,
		SpeakMaxMs:   cfg.Pet.SpeakMaxMs,
		Name:         cfg.Character.Name,
		BrainTimeout: time.Second,
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go p.Run(ctx)
	sim := danmaku.NewSimulator(p, time.Duration(cfg.Mock.IntervalSec)*time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	s := New(cfg, p, h, sim, mem, "mock", "test", slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return &harness{ts: ts, pipe: p}
}

func testConfig() config.Config {
	cfg := config.Defaults()
	cfg.Pet.SpeakMsPerRune = 1
	cfg.Pet.SpeakMinMs = 40
	cfg.Pet.SpeakMaxMs = 80
	cfg.Character.Name = "小鲸鱼"
	cfg.Queue.MaxSize = 8
	cfg.Mock.Enabled = true
	cfg.Mock.IntervalSec = 12
	return cfg
}

func TestDanmakuSpeaksOverWebSocket(t *testing.T) {
	h := startHarness(t, testConfig())
	wsURL := "ws" + strings.TrimPrefix(h.ts.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	var hello map[string]any
	if err := conn.ReadJSON(&hello); err != nil {
		t.Fatal(err)
	}
	if hello["type"] != "hello" || hello["character"] != "小鲸鱼" {
		t.Fatalf("hello = %#v", hello)
	}
	var snap map[string]any
	if err := conn.ReadJSON(&snap); err != nil {
		t.Fatal(err)
	}
	if snap["state"] != "idle" {
		t.Fatalf("snap = %#v", snap)
	}

	body := []byte(`{"user":"小鱼干","text":"小鲸鱼，帮我讲讲这段代码","source":"test"}`)
	res, err := http.Post(h.ts.URL+"/api/danmaku", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d %s", res.StatusCode, b)
	}

	deadline := time.Now().Add(3 * time.Second)
	_ = conn.SetReadDeadline(deadline)
	var spoke string
	for time.Now().Before(deadline) {
		var ev map[string]any
		if err := conn.ReadJSON(&ev); err != nil {
			t.Fatal(err)
		}
		if ev["state"] == "speaking" {
			spoke, _ = ev["reply"].(string)
			if ev["mock"] != true {
				t.Fatalf("expected mock reply, got %#v", ev)
			}
			break
		}
	}
	if spoke == "" {
		t.Fatal("pet never reached speaking")
	}
}

func TestIngestTokenAndDouyinStub(t *testing.T) {
	cfg := testConfig()
	cfg.Ingest.Token = "secret"
	cfg.Ingest.Strict = true
	h := startHarness(t, cfg)

	res, err := http.Post(h.ts.URL+"/api/danmaku", "application/json", strings.NewReader(`{"text":"你好"}`))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d", res.StatusCode)
	}

	req, err := http.NewRequest(http.MethodPost, h.ts.URL+"/api/danmaku", strings.NewReader(`{"text":"你好呀"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Ingest-Token", "secret")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("authorized status = %d", res.StatusCode)
	}

	res, err = http.Post(h.ts.URL+"/api/danmaku/douyin/pull", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotImplemented {
		t.Fatalf("douyin status = %d", res.StatusCode)
	}
	var payload map[string]any
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload["error"] != "douyin_pull_not_implemented" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestPetPageServed(t *testing.T) {
	cfg := testConfig()
	cfg.Server.WebDir = filepath.Join(repoRoot(t), "web")
	h := startHarness(t, cfg)
	res, err := http.Get(h.ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK || !bytes.Contains(b, []byte("小鲸鱼")) {
		n := len(b)
		if n > 180 {
			n = 180
		}
		t.Fatalf("page status %d body %q", res.StatusCode, b[:n])
	}
}
