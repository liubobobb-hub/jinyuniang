package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvOverridesYAML(t *testing.T) {
	dir := t.TempDir()
	yaml := "server:\n  addr: \":9999\"\ndeepseek:\n  model: from-yaml\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	env := "DEEPSEEK_API_KEY=from-dotenv\nWHALE_ADDR=127.0.0.1:8788\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(env), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WHALE_CONFIG", "")
	t.Setenv("DEEPSEEK_MODEL", "from-env")
	t.Setenv("WHALE_MYSQL_ENABLED", "true")
	cfg, err := LoadFrom(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Addr != "127.0.0.1:8788" {
		t.Fatalf("addr = %s", cfg.Server.Addr)
	}
	if cfg.DeepSeek.APIKey != "from-dotenv" {
		t.Fatalf("key = %q", cfg.DeepSeek.APIKey)
	}
	if cfg.DeepSeek.Model != "from-env" {
		t.Fatalf("model = %s", cfg.DeepSeek.Model)
	}
	if !cfg.Log.MySQLEnabled {
		t.Fatal("mysql flag not applied")
	}
	if cfg.DeepSeek.BaseURL != "https://api.deepseek.com" {
		t.Fatalf("base = %s", cfg.DeepSeek.BaseURL)
	}
	if cfg.Character.Name != "小鲸鱼" {
		t.Fatalf("name = %s", cfg.Character.Name)
	}
	if !strings.HasPrefix(cfg.Server.WebDir, dir) {
		t.Fatalf("web dir = %s", cfg.Server.WebDir)
	}
}
