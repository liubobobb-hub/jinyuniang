package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"whale-live/internal/config"
	"whale-live/internal/danmaku"
	"whale-live/internal/model"
	"whale-live/internal/pipeline"
	"whale-live/internal/store"
	"whale-live/internal/textutil"
)

type History interface {
	Recent(n int) []store.Record
}

type Server struct {
	cfg       config.Config
	pipe      *pipeline.Pipeline
	hub       hubPublisher
	sim       *danmaku.Simulator
	store     History
	log       *slog.Logger
	brainMode string
	version   string
	started   time.Time
	upgrader  websocket.Upgrader
}

// hubPublisher is the broadcast hub the pet socket writes to.
type hubPublisher interface {
	Publish(v any)
	Subscribe() (<-chan []byte, func())
}

func New(cfg config.Config, pipe *pipeline.Pipeline, h hubPublisher, sim *danmaku.Simulator, history History, brainMode, version string, log *slog.Logger) *Server {
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Server{
		cfg:       cfg,
		pipe:      pipe,
		hub:       h,
		sim:       sim,
		store:     history,
		log:       log,
		brainMode: brainMode,
		version:   version,
		started:   time.Now(),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(*http.Request) bool { return true },
		},
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/status", s.status)
	mux.HandleFunc("GET /api/config", s.publicConfig)
	mux.HandleFunc("GET /api/history", s.history)
	mux.HandleFunc("POST /api/danmaku", s.postDanmaku)
	mux.HandleFunc("POST /api/mock/once", s.mockOnce)
	mux.HandleFunc("POST /api/mock/start", s.mockStart)
	mux.HandleFunc("POST /api/mock/stop", s.mockStop)
	mux.HandleFunc("GET /api/mock/samples", s.mockSamples)
	mux.HandleFunc("POST /api/danmaku/douyin/pull", s.douyinPull)
	mux.HandleFunc("GET /ws", s.ws)
	mux.HandleFunc("GET /pet", s.petPage)
	if dir := strings.TrimSpace(s.cfg.TTS.Dir); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
		media := http.StripPrefix("/media/", http.FileServer(http.Dir(dir)))
		mux.Handle("GET /media/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			media.ServeHTTP(w, r)
		}))
	}
	web := http.FileServer(http.Dir(s.cfg.Server.WebDir))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		web.ServeHTTP(w, r)
	}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Ingest-Token")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Cache-Control", "no-store")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"service": "whale-live",
		"version": s.version,
	})
}

func (s *Server) status(w http.ResponseWriter, _ *http.Request) {
	snap := s.pipe.Snapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"state":       snap.State,
		"queue":       snap.Queue,
		"user":        snap.User,
		"danmaku":     snap.Danmaku,
		"reply":       snap.Reply,
		"mock":        snap.Mock,
		"degraded":    snap.Degraded,
		"brain":       s.brainMode,
		"model":       s.cfg.DeepSeek.Model,
		"character":   s.cfg.Character.Name,
		"mockRunning": s.sim.Running(),
		"uptimeSec":   int(time.Since(s.started).Seconds()),
		"version":     s.version,
	})
}

func (s *Server) publicConfig(w http.ResponseWriter, r *http.Request) {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	writeJSON(w, http.StatusOK, map[string]any{
		"character":   s.cfg.Character.Name,
		"width":       s.cfg.Pet.Width,
		"height":      s.cfg.Pet.Height,
		"chroma":      s.cfg.Pet.Chroma,
		"brain":       s.brainMode,
		"model":       s.cfg.DeepSeek.Model,
		"mockEnabled": s.cfg.Mock.Enabled,
		"ttsEnabled":  s.cfg.TTS.Enabled,
		"ttsProvider": s.cfg.TTS.Provider,
		"ttsVoice":    s.cfg.TTS.Voice,
		"version":     s.version,
		"obsURL":      scheme + "://" + host + "/?obs=1",
		"pageURL":     scheme + "://" + host + "/",
	})
}

func (s *Server) history(w http.ResponseWriter, r *http.Request) {
	n := 20
	if q := r.URL.Query().Get("limit"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 && v <= 100 {
			n = v
		}
	}
	items := []store.Record{}
	if s.store != nil {
		items = s.store.Recent(n)
		if items == nil {
			items = []store.Record{}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) postDanmaku(w http.ResponseWriter, r *http.Request) {
	if !s.authorize(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"ok":      false,
			"error":   "unauthorized",
			"message": "缺少或错误的 X-Ingest-Token",
		})
		return
	}
	var req struct {
		User   string `json:"user"`
		Text   string `json:"text"`
		Source string `json:"source"`
	}
	if err := s.readJSON(w, r, &req); err != nil {
		s.writeReadErr(w, err)
		return
	}
	c := model.Comment{
		User:   textutil.Clip(textutil.Clean(req.User), 32),
		Text:   textutil.Clip(textutil.Clean(req.Text), 400),
		Source: textutil.Clip(textutil.Clean(req.Source), 24),
	}
	if c.Source == "" {
		c.Source = "api"
	}
	s.accept(w, c)
}

func (s *Server) accept(w http.ResponseWriter, c model.Comment) {
	if err := s.pipe.Submit(c); err != nil {
		if errors.Is(err, pipeline.ErrFull) {
			writeJSON(w, http.StatusTooManyRequests, map[string]any{
				"ok":      false,
				"error":   "queue_full",
				"message": "队列已满，等小鲸鱼说完这一句",
			})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok":      false,
			"error":   "bad_danmaku",
			"message": "弹幕是空的",
		})
		return
	}
	s.log.Info("danmaku", "user", c.User, "source", c.Source, "text", c.Text)
	snap := s.pipe.Snapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":    true,
		"queue": snap.Queue,
		"state": snap.State,
	})
}

func (s *Server) mockOnce(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.Mock.Enabled {
		s.mockDisabled(w)
		return
	}
	var req struct {
		User string `json:"user"`
		Text string `json:"text"`
	}
	if err := s.readJSON(w, r, &req); err != nil && !errors.Is(err, io.EOF) {
		s.writeReadErr(w, err)
		return
	}
	c, err := s.sim.Once(textutil.Clip(textutil.Clean(req.User), 32), textutil.Clip(textutil.Clean(req.Text), 400))
	if err != nil {
		if errors.Is(err, pipeline.ErrFull) {
			writeJSON(w, http.StatusTooManyRequests, map[string]any{"ok": false, "error": "queue_full", "message": "队列已满，等小鲸鱼说完这一句"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "bad_danmaku"})
		return
	}
	snap := s.pipe.Snapshot()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "user": c.User, "text": c.Text, "queue": snap.Queue, "state": snap.State})
}

func (s *Server) mockStart(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.Mock.Enabled {
		s.mockDisabled(w)
		return
	}
	var req struct {
		IntervalSec int `json:"intervalSec"`
	}
	if err := s.readJSON(w, r, &req); err != nil && !errors.Is(err, io.EOF) {
		s.writeReadErr(w, err)
		return
	}
	d := time.Duration(s.cfg.Mock.IntervalSec) * time.Second
	if req.IntervalSec > 0 {
		d = time.Duration(req.IntervalSec) * time.Second
	}
	if d < 2*time.Second {
		d = 2 * time.Second
	}
	s.sim.Start(d)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "running": true, "intervalSec": int(d.Seconds())})
}

func (s *Server) mockStop(w http.ResponseWriter, _ *http.Request) {
	s.sim.Stop()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "running": false})
}

func (s *Server) mockSamples(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": danmaku.Samples})
}

func (s *Server) douyinPull(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusNotImplemented, map[string]any{
		"ok":      false,
		"error":   "douyin_pull_not_implemented",
		"message": "本服务不内置抖音弹幕抓取。请用你自己的采集脚本把弹幕 POST 到 /api/danmaku。说明见 docs/douyin.md。",
	})
}

func (s *Server) mockDisabled(w http.ResponseWriter) {
	writeJSON(w, http.StatusForbidden, map[string]any{
		"ok":      false,
		"error":   "mock_disabled",
		"message": "配置里的 mock.enabled 为 false",
	})
}

func (s *Server) petPage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join(s.cfg.Server.WebDir, "index.html"))
}

func (s *Server) ws(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Warn("ws upgrade", "err", err)
		return
	}
	defer conn.Close()
	events, unsub := s.hub.Subscribe()
	defer unsub()

	_ = conn.SetReadDeadline(time.Now().Add(70 * time.Second))
	conn.SetReadLimit(4096)
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(70 * time.Second))
	})

	ctx, stop := context.WithCancel(r.Context())
	defer stop()
	go func() {
		defer stop()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	hello := map[string]any{
		"type":      "hello",
		"character": s.cfg.Character.Name,
		"width":     s.cfg.Pet.Width,
		"height":    s.cfg.Pet.Height,
		"chroma":    s.cfg.Pet.Chroma,
		"brain":     s.brainMode,
		"version":   s.version,
	}
	if err := writeWS(conn, hello); err != nil {
		return
	}
	if err := writeWS(conn, s.pipe.Snapshot()); err != nil {
		return
	}

	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ping.C:
			_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case msg, ok := <-events:
			if !ok {
				return
			}
			_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
	}
}

func (s *Server) authorize(r *http.Request) bool {
	token := strings.TrimSpace(s.cfg.Ingest.Token)
	if token == "" {
		return true
	}
	if !s.cfg.Ingest.Strict && isLoopback(r) {
		return true
	}
	got := strings.TrimSpace(r.Header.Get("X-Ingest-Token"))
	if got == "" {
		const prefix = "Bearer "
		h := r.Header.Get("Authorization")
		if strings.HasPrefix(h, prefix) {
			got = strings.TrimSpace(strings.TrimPrefix(h, prefix))
		}
	}
	if len(got) != len(token) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}

func isLoopback(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

var errBodyTooLarge = errors.New("body too large")

func (s *Server) readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	err := dec.Decode(dst)
	if err == nil {
		return nil
	}
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return errBodyTooLarge
	}
	return err
}

func (s *Server) writeReadErr(w http.ResponseWriter, err error) {
	if errors.Is(err, errBodyTooLarge) {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"ok": false, "error": "body_too_large"})
		return
	}
	writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid_json", "message": "请求体不是 JSON"})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func writeWS(conn *websocket.Conn, v any) error {
	_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return conn.WriteJSON(v)
}
