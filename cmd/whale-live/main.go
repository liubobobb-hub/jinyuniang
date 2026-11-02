package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"whale-live/internal/brain"
	"whale-live/internal/character"
	"whale-live/internal/config"
	"whale-live/internal/danmaku"
	"whale-live/internal/hub"
	"whale-live/internal/pipeline"
	"whale-live/internal/server"
	"whale-live/internal/store"
	"whale-live/internal/tts"
	"whale-live/internal/version"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取配置失败: %v\n", err)
		os.Exit(1)
	}

	st := openStore(cfg, log)
	br := newBrain(cfg, log)
	h := hub.New()
	var voice pipeline.VoiceSynth
	if cfg.TTS.Enabled && cfg.TTS.Provider == "edge" {
		voice = tts.Adapter{Edge: &tts.Edge{
			Enabled: true,
			Python:  cfg.TTS.Python,
			Voice:   cfg.TTS.Voice,
			Rate:    cfg.TTS.Rate,
			Dir:     cfg.TTS.Dir,
			Timeout: time.Duration(cfg.TTS.TimeoutSec) * time.Second,
		}}
		log.Info("tts", "provider", "edge", "voice", cfg.TTS.Voice, "dir", cfg.TTS.Dir)
	} else {
		log.Info("tts", "enabled", false)
	}
	pipe := pipeline.New(pipeline.Options{
		Hub:          h,
		Brain:        br,
		Store:        st,
		Voice:        voice,
		Log:          log,
		Name:         cfg.Character.Name,
		QueueSize:    cfg.Queue.MaxSize,
		SpeakPerRune: cfg.Pet.SpeakMsPerRune,
		SpeakMinMs:   cfg.Pet.SpeakMinMs,
		SpeakMaxMs:   cfg.Pet.SpeakMaxMs,
		BrainTimeout: time.Duration(cfg.DeepSeek.TimeoutSec) * time.Second,
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go pipe.Run(ctx)

	sim := danmaku.NewSimulator(pipe, time.Duration(cfg.Mock.IntervalSec)*time.Second, log)
	if cfg.Mock.Enabled && cfg.Mock.AutoStart {
		sim.Start(time.Duration(cfg.Mock.IntervalSec) * time.Second)
		log.Info("mock danmaku auto-started", "intervalSec", cfg.Mock.IntervalSec)
	}

	api := server.New(cfg, pipe, h, sim, st, br.Mode(), version.Version, log)
	httpServer := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.ListenAndServe()
	}()

	printBanner(cfg, br.Mode())

	select {
	case <-ctx.Done():
		sh, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(sh)
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "服务启动失败: %v\n", err)
			if strings.Contains(err.Error(), "address already in use") {
				fmt.Fprintf(os.Stderr, "端口已被占用。换一个 WHALE_ADDR，或关掉正在跑的小鲸鱼。\n")
			}
			os.Exit(1)
		}
	}
}

func newBrain(cfg config.Config, log *slog.Logger) brain.Brain {
	prompt := character.SystemPrompt(cfg.Character.Name, cfg.Character.SystemPrompt)
	if strings.TrimSpace(cfg.DeepSeek.APIKey) == "" {
		return brain.Local{}
	}
	return brain.NewDeepSeek(
		cfg.DeepSeek.APIKey,
		cfg.DeepSeek.BaseURL,
		cfg.DeepSeek.Model,
		prompt,
		time.Duration(cfg.DeepSeek.TimeoutSec)*time.Second,
		cfg.DeepSeek.MaxTokens,
		cfg.DeepSeek.Temperature,
		log,
	)
}

func openStore(cfg config.Config, log *slog.Logger) *store.Bundle {
	mem := store.NewMemory(100)
	if cfg.Log.File != "" {
		recs, err := store.ReadTail(cfg.Log.File, 100)
		if err != nil {
			log.Warn("read history", "err", err)
		}
		for _, rec := range recs {
			_ = mem.Save(context.Background(), rec)
		}
	}
	b := store.NewBundle(mem)
	if cfg.Log.File != "" {
		f, err := store.OpenFile(cfg.Log.File)
		if err != nil {
			log.Warn("file log disabled", "path", cfg.Log.File, "err", err)
		} else {
			b.Add(f)
			log.Info("interaction log", "path", cfg.Log.File)
		}
	}
	if cfg.Log.MySQLEnabled {
		if strings.TrimSpace(cfg.Log.MySQLDSN) == "" {
			log.Warn("mysql enabled but dsn is empty; staying on the file log")
		} else if db, err := store.OpenMySQL(cfg.Log.MySQLDSN); err != nil {
			log.Warn("mysql unavailable, staying on the file log", "err", err)
		} else {
			b.Add(db)
			log.Info("mysql log enabled")
		}
	}
	return b
}

func printBanner(cfg config.Config, mode string) {
	page := displayURL(cfg.Server.Addr)
	brainLine := "本地模拟（未设置 DEEPSEEK_API_KEY，小鲸鱼会自己接话）"
	if mode == "deepseek" {
		brainLine = "DeepSeek / " + cfg.DeepSeek.Model
	}
	ttsLine := "浏览器朗读（未开 edge-tts）"
	if cfg.TTS.Enabled && cfg.TTS.Provider == "edge" {
		ttsLine = "edge-tts / " + cfg.TTS.Voice
	}
	fmt.Printf("鲸鱼娘已出水\n  桌宠页面  %s\n  OBS 采集  %s\n  建议尺寸  %d×%d\n  绿幕颜色  %s\n  大脑      %s\n  语音      %s\n",
		page,
		strings.TrimRight(page, "/")+"/?obs=1",
		cfg.Pet.Width,
		cfg.Pet.Height,
		cfg.Pet.Chroma,
		brainLine,
		ttsLine,
	)
}

func displayURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://127.0.0.1:8787/"
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port) + "/"
}
