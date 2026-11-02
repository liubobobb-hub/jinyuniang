package tts

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Result is one synthesized clip ready for the pet page.
type Result struct {
	Path     string
	URL      string
	Duration time.Duration
}

// Edge calls local `python -m edge_tts` (Microsoft Edge online voices).
type Edge struct {
	Enabled bool
	Python  string
	Voice   string
	Rate    string
	Dir     string
	Timeout time.Duration
}

func (e *Edge) ready() bool {
	return e != nil && e.Enabled && strings.TrimSpace(e.Dir) != ""
}

// Speak writes an mp3 under Dir and returns a /media URL plus measured duration.
func (e *Edge) Speak(ctx context.Context, id, text string) (Result, error) {
	if !e.ready() {
		return Result{}, errors.New("edge tts disabled")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return Result{}, errors.New("empty text")
	}
	if err := os.MkdirAll(e.Dir, 0o755); err != nil {
		return Result{}, err
	}
	id = sanitizeID(id)
	out := filepath.Join(e.Dir, id+".mp3")
	_ = os.Remove(out)

	py := strings.TrimSpace(e.Python)
	if py == "" {
		py = "py"
	}
	voice := e.Voice
	if voice == "" {
		voice = "zh-CN-XiaoyiNeural"
	}
	rate := e.Rate
	if rate == "" {
		rate = "+8%"
	}
	timeout := e.Timeout
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ttsArgs := []string{"-m", "edge_tts", "--voice", voice, "--rate", rate, "--text", text, "--write-media", out}
	if err := runPython(cctx, py, ttsArgs); err != nil {
		return Result{}, err
	}
	if st, err := os.Stat(out); err != nil || st.Size() < 32 {
		return Result{}, fmt.Errorf("edge-tts produced no audio: %v", err)
	}
	dur, err := mp3Duration(cctx, py, out)
	if err != nil {
		dur = 0
	}
	return Result{
		Path:     out,
		URL:      "/media/" + id + ".mp3",
		Duration: dur,
	}, nil
}

func runPython(ctx context.Context, py string, moduleArgs []string) error {
	try := func(bin string, prefix []string) error {
		args := append(append([]string{}, prefix...), moduleArgs...)
		cmd := exec.CommandContext(ctx, bin, args...)
		b, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s: %w (%s)", bin, err, truncate(string(b)))
		}
		return nil
	}
	var errs []string
	order := []struct {
		bin    string
		prefix []string
	}{}
	if py == "py" {
		order = append(order, struct {
			bin    string
			prefix []string
		}{"py", []string{"-3"}}, struct {
			bin    string
			prefix []string
		}{"python", nil})
	} else {
		order = append(order, struct {
			bin    string
			prefix []string
		}{py, nil})
		if py != "python" {
			order = append(order, struct {
				bin    string
				prefix []string
			}{"python", nil})
		}
	}
	for _, o := range order {
		if err := try(o.bin, o.prefix); err == nil {
			return nil
		} else {
			errs = append(errs, err.Error())
		}
	}
	return fmt.Errorf("edge-tts failed: %s", strings.Join(errs, " | "))
}

func mp3Duration(ctx context.Context, py, path string) (time.Duration, error) {
	code := "import sys; from mutagen.mp3 import MP3; print(MP3(sys.argv[1]).info.length)"
	try := func(bin string, prefix []string) (time.Duration, error) {
		args := append(append([]string{}, prefix...), "-c", code, path)
		cmd := exec.CommandContext(ctx, bin, args...)
		b, err := cmd.Output()
		if err != nil {
			return 0, err
		}
		sec, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
		if err != nil {
			return 0, err
		}
		if sec <= 0 {
			return 0, errors.New("non-positive duration")
		}
		return time.Duration(sec*float64(time.Second) + 0.5*float64(time.Second)/10), nil
	}
	if py == "py" {
		if d, err := try("py", []string{"-3"}); err == nil {
			return d, nil
		}
	}
	if d, err := try(py, nil); err == nil {
		return d, nil
	}
	return try("python", nil)
}

func sanitizeID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	var b strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	s := b.String()
	if s == "" {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if len(s) > 80 {
		s = s[:80]
	}
	return s
}

func truncate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 240 {
		return s[:240] + "…"
	}
	return s
}
