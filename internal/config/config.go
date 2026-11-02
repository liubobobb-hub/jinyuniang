package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	DeepSeek  DeepSeekConfig  `yaml:"deepseek"`
	Character CharacterConfig `yaml:"character"`
	Queue     QueueConfig     `yaml:"queue"`
	Mock      MockConfig      `yaml:"mock"`
	Log       LogConfig       `yaml:"log"`
	Pet       PetConfig       `yaml:"pet"`
	TTS       TTSConfig       `yaml:"tts"`
	Ingest    IngestConfig    `yaml:"ingest"`
}

type ServerConfig struct {
	Addr   string `yaml:"addr"`
	WebDir string `yaml:"web_dir"`
}

type DeepSeekConfig struct {
	APIKey      string  `yaml:"api_key"`
	BaseURL     string  `yaml:"base_url"`
	Model       string  `yaml:"model"`
	TimeoutSec  int     `yaml:"timeout_sec"`
	MaxTokens   int     `yaml:"max_tokens"`
	Temperature float64 `yaml:"temperature"`
}

type CharacterConfig struct {
	Name         string `yaml:"name"`
	SystemPrompt string `yaml:"system_prompt"`
}

type QueueConfig struct {
	MaxSize int `yaml:"max_size"`
}

type MockConfig struct {
	Enabled     bool `yaml:"enabled"`
	IntervalSec int  `yaml:"interval_sec"`
	AutoStart   bool `yaml:"auto_start"`
}

type LogConfig struct {
	File         string `yaml:"file"`
	MySQLEnabled bool   `yaml:"mysql_enabled"`
	MySQLDSN     string `yaml:"mysql_dsn"`
}

type PetConfig struct {
	Width          int    `yaml:"width"`
	Height         int    `yaml:"height"`
	Chroma         string `yaml:"chroma"`
	SpeakMsPerRune int    `yaml:"speak_ms_per_rune"`
	SpeakMinMs     int    `yaml:"speak_min_ms"`
	SpeakMaxMs     int    `yaml:"speak_max_ms"`
}

type IngestConfig struct {
	Token  string `yaml:"token"`
	Strict bool   `yaml:"strict"`
}

type TTSConfig struct {
	Enabled bool   `yaml:"enabled"`
	Provider string `yaml:"provider"`
	Python  string `yaml:"python"`
	Voice   string `yaml:"voice"`
	Rate    string `yaml:"rate"`
	Dir     string `yaml:"dir"`
	TimeoutSec int `yaml:"timeout_sec"`
}

func Defaults() Config {
	return Config{
		Server: ServerConfig{
			Addr:   "127.0.0.1:8787",
			WebDir: "web",
		},
		DeepSeek: DeepSeekConfig{
			BaseURL:     "https://api.deepseek.com",
			Model:       "deepseek-chat",
			TimeoutSec:  25,
			MaxTokens:   180,
			Temperature: 0.8,
		},
		Character: CharacterConfig{Name: "小鲸鱼"},
		Queue:     QueueConfig{MaxSize: 32},
		Mock:      MockConfig{Enabled: true, IntervalSec: 12, AutoStart: false},
		Log:       LogConfig{File: "data/interactions.jsonl"},
		Pet: PetConfig{
			Width:          1280,
			Height:         720,
			Chroma:         "#00FF00",
			SpeakMsPerRune: 240,
			SpeakMinMs:     2800,
			SpeakMaxMs:     14000,
		},
		TTS: TTSConfig{
			Enabled:    true,
			Provider:   "edge",
			Python:     "py",
			Voice:      "zh-CN-XiaoyiNeural",
			Rate:       "+8%",
			Dir:        "data/audio",
			TimeoutSec: 45,
		},
	}
}

// Load reads config.yaml and .env from the project root, then applies the environment.
func Load() (Config, error) {
	return LoadFrom(ProjectRoot())
}

func LoadFrom(root string) (Config, error) {
	cfg := Defaults()
	path := os.Getenv("WHALE_CONFIG")
	explicit := path != ""
	if path == "" {
		path = filepath.Join(root, "config.yaml")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if explicit || !os.IsNotExist(err) {
			return Config{}, fmt.Errorf("read config: %w", err)
		}
	} else if err := yaml.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	dot := readDotEnv(filepath.Join(root, ".env"))
	applyEnv(&cfg, dot)
	cfg.normalize()
	cfg.resolve(root)
	return cfg, nil
}

func (c *Config) normalize() {
	d := Defaults()
	if strings.TrimSpace(c.Server.Addr) == "" {
		c.Server.Addr = d.Server.Addr
	}
	if strings.TrimSpace(c.Server.WebDir) == "" {
		c.Server.WebDir = d.Server.WebDir
	}
	if strings.TrimSpace(c.DeepSeek.BaseURL) == "" {
		c.DeepSeek.BaseURL = d.DeepSeek.BaseURL
	}
	if strings.TrimSpace(c.DeepSeek.Model) == "" {
		c.DeepSeek.Model = d.DeepSeek.Model
	}
	if c.DeepSeek.TimeoutSec <= 0 {
		c.DeepSeek.TimeoutSec = d.DeepSeek.TimeoutSec
	}
	if c.DeepSeek.MaxTokens <= 0 {
		c.DeepSeek.MaxTokens = d.DeepSeek.MaxTokens
	}
	if c.DeepSeek.Temperature <= 0 {
		c.DeepSeek.Temperature = d.DeepSeek.Temperature
	}
	if strings.TrimSpace(c.Character.Name) == "" {
		c.Character.Name = d.Character.Name
	}
	if c.Queue.MaxSize <= 0 {
		c.Queue.MaxSize = d.Queue.MaxSize
	}
	if c.Mock.IntervalSec <= 0 {
		c.Mock.IntervalSec = d.Mock.IntervalSec
	}
	if c.Pet.Width <= 0 {
		c.Pet.Width = d.Pet.Width
	}
	if c.Pet.Height <= 0 {
		c.Pet.Height = d.Pet.Height
	}
	if strings.TrimSpace(c.Pet.Chroma) == "" {
		c.Pet.Chroma = d.Pet.Chroma
	}
	if c.Pet.SpeakMsPerRune <= 0 {
		c.Pet.SpeakMsPerRune = d.Pet.SpeakMsPerRune
	}
	if c.Pet.SpeakMinMs <= 0 {
		c.Pet.SpeakMinMs = d.Pet.SpeakMinMs
	}
	if c.Pet.SpeakMaxMs < c.Pet.SpeakMinMs {
		c.Pet.SpeakMaxMs = d.Pet.SpeakMaxMs
	}
	if strings.TrimSpace(c.Log.File) == "" {
		c.Log.File = d.Log.File
	}
	if strings.TrimSpace(c.TTS.Provider) == "" {
		c.TTS.Provider = d.TTS.Provider
	}
	if strings.TrimSpace(c.TTS.Python) == "" {
		c.TTS.Python = d.TTS.Python
	}
	if strings.TrimSpace(c.TTS.Voice) == "" {
		c.TTS.Voice = d.TTS.Voice
	}
	if strings.TrimSpace(c.TTS.Rate) == "" {
		c.TTS.Rate = d.TTS.Rate
	}
	if strings.TrimSpace(c.TTS.Dir) == "" {
		c.TTS.Dir = d.TTS.Dir
	}
	if c.TTS.TimeoutSec <= 0 {
		c.TTS.TimeoutSec = d.TTS.TimeoutSec
	}
}

func (c *Config) resolve(root string) {
	c.Server.WebDir = underRoot(root, c.Server.WebDir)
	c.Log.File = underRoot(root, c.Log.File)
	c.TTS.Dir = underRoot(root, c.TTS.Dir)
}

func underRoot(root, p string) string {
	if strings.TrimSpace(p) == "" || filepath.IsAbs(p) {
		return p
	}
	if root == "" {
		return p
	}
	return filepath.Join(root, p)
}

func applyEnv(cfg *Config, dot map[string]string) {
	if v, ok := lookup("WHALE_ADDR", dot); ok {
		cfg.Server.Addr = v
	}
	if v, ok := lookup("DEEPSEEK_API_KEY", dot); ok {
		cfg.DeepSeek.APIKey = v
	}
	if v, ok := lookup("DEEPSEEK_BASE_URL", dot); ok {
		cfg.DeepSeek.BaseURL = v
	}
	if v, ok := lookup("DEEPSEEK_MODEL", dot); ok {
		cfg.DeepSeek.Model = v
	}
	if v, ok := lookup("WHALE_INGEST_TOKEN", dot); ok {
		cfg.Ingest.Token = v
	}
	if v, ok := lookup("WHALE_INGEST_STRICT", dot); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.Ingest.Strict = b
		}
	}
	if v, ok := lookup("WHALE_MYSQL_ENABLED", dot); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.Log.MySQLEnabled = b
		}
	}
	if v, ok := lookup("WHALE_MYSQL_DSN", dot); ok {
		cfg.Log.MySQLDSN = v
	}
	if v, ok := lookup("WHALE_LOG_FILE", dot); ok {
		cfg.Log.File = v
	}
	if v, ok := lookup("WHALE_MOCK_AUTOSTART", dot); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.Mock.AutoStart = b
		}
	}
	if v, ok := lookup("WHALE_TTS_ENABLED", dot); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.TTS.Enabled = b
		}
	}
	if v, ok := lookup("WHALE_TTS_VOICE", dot); ok {
		cfg.TTS.Voice = v
	}
	if v, ok := lookup("WHALE_TTS_PYTHON", dot); ok {
		cfg.TTS.Python = v
	}
}

func lookup(key string, dot map[string]string) (string, bool) {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v), true
	}
	if v, ok := dot[key]; ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v), true
	}
	return "", false
}

func readDotEnv(path string) map[string]string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		v = strings.Trim(v, `"'`)
		out[k] = v
	}
	return out
}

// ProjectRoot walks upward from the working directory and the executable
// until it finds the pet page. go run and a built binary both work.
func ProjectRoot() string {
	var starts []string
	if wd, err := os.Getwd(); err == nil {
		starts = append(starts, wd)
	}
	if exe, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(exe))
	}
	for _, start := range starts {
		if d := walkUp(start); d != "" {
			return d
		}
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}

func walkUp(start string) string {
	d := start
	for i := 0; i < 8; i++ {
		if fileExists(filepath.Join(d, "web", "index.html")) {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
