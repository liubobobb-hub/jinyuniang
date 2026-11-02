package brain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"whale-live/internal/textutil"
)

// DeepSeek calls an OpenAI-compatible chat completions endpoint.
// A network or API failure degrades to the local persona so the stream never stalls.
type DeepSeek struct {
	apiKey      string
	baseURL     string
	model       string
	system      string
	maxTokens   int
	temperature float64
	client      *http.Client
	log         *slog.Logger
}

func NewDeepSeek(apiKey, baseURL, model, system string, timeout time.Duration, maxTokens int, temperature float64, log *slog.Logger) *DeepSeek {
	if timeout <= 0 {
		timeout = 25 * time.Second
	}
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &DeepSeek{
		apiKey:      strings.TrimSpace(apiKey),
		baseURL:     baseURL,
		model:       model,
		system:      system,
		maxTokens:   maxTokens,
		temperature: temperature,
		log:         log,
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (d *DeepSeek) Mode() string { return "deepseek" }

func (d *DeepSeek) Reply(ctx context.Context, user, text string) Result {
	reply, err := d.complete(ctx, user, text)
	if err != nil {
		d.log.Warn("deepseek failed, using local reply", "err", err)
		return Result{Reply: MockReply(user, text), Mock: true, Degraded: true, Err: err}
	}
	reply = TidyReply(reply)
	if reply == "" {
		return Result{Reply: MockReply(user, text), Mock: true, Degraded: true, Err: fmt.Errorf("empty completion")}
	}
	return Result{Reply: reply}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
	Stream      bool          `json:"stream"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func (d *DeepSeek) complete(ctx context.Context, user, text string) (string, error) {
	name := strings.TrimSpace(user)
	if name == "" {
		name = "观众"
	}
	body, err := json.Marshal(chatRequest{
		Model: d.model,
		Messages: []chatMessage{
			{Role: "system", Content: d.system},
			{Role: "user", Content: fmt.Sprintf("观众「%s」的弹幕：%s", name, text)},
		},
		Temperature: d.temperature,
		MaxTokens:   d.maxTokens,
		Stream:      false,
	})
	if err != nil {
		return "", err
	}
	url := ChatCompletionsURL(d.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.apiKey)
	req.Header.Set("User-Agent", "whale-live/1.0")

	resp, err := d.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return "", fmt.Errorf("deepseek http %d: %s", resp.StatusCode, msg)
	}
	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("decode completion: %w", err)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return "", fmt.Errorf("deepseek: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("deepseek: no choices")
	}
	return parsed.Choices[0].Message.Content, nil
}

// ChatCompletionsURL accepts the documented DeepSeek bases:
// https://api.deepseek.com and https://api.deepseek.com/v1.
func ChatCompletionsURL(base string) string {
	b := strings.TrimRight(strings.TrimSpace(base), "/")
	if b == "" {
		b = "https://api.deepseek.com"
	}
	switch {
	case strings.HasSuffix(b, "/chat/completions"):
		return b
	case strings.HasSuffix(b, "/v1"):
		return b + "/chat/completions"
	default:
		return b + "/chat/completions"
	}
}

var fenceRE = regexp.MustCompile("(?s)```.*?```")

// TidyReply strips markdown the model was asked not to emit, and keeps the line speakable.
func TidyReply(s string) string {
	s = strings.TrimSpace(s)
	s = fenceRE.ReplaceAllString(s, " ")
	s = strings.NewReplacer("**", "", "__", "", "`", "", "#", "").Replace(s)
	s = strings.ReplaceAll(s, "\r\n", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.Trim(s, "\"“”")
	s = strings.Join(strings.Fields(s), " ")
	return textutil.ClipSentence(s, 120)
}
