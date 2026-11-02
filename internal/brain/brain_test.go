package brain

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestChatCompletionsURL(t *testing.T) {
	cases := map[string]string{
		"":                                        "https://api.deepseek.com/chat/completions",
		"https://api.deepseek.com":                "https://api.deepseek.com/chat/completions",
		"https://api.deepseek.com/":               "https://api.deepseek.com/chat/completions",
		"https://api.deepseek.com/v1":             "https://api.deepseek.com/v1/chat/completions",
		"https://api.deepseek.com/v1/":            "https://api.deepseek.com/v1/chat/completions",
		"https://example.com/v1/chat/completions": "https://example.com/v1/chat/completions",
	}
	for in, want := range cases {
		if got := ChatCompletionsURL(in); got != want {
			t.Errorf("ChatCompletionsURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMockReplyShortAndPlain(t *testing.T) {
	lines := []string{
		"小鲸鱼，帮我讲讲这段代码",
		"这个 nil pointer 怎么回事呀",
		"for 循环和 range 哪个好",
		"递归怎么才能看懂",
		"channel 阻塞是什么意思",
		"goroutine 会不会把变量抢坏",
		"小鲸鱼下午好，今天好可爱",
		"随便说点什么",
	}
	for _, line := range lines {
		reply := MockReply("小鱼干", line)
		if strings.TrimSpace(reply) == "" {
			t.Fatalf("empty reply for %q", line)
		}
		if strings.Contains(reply, "```") || strings.Contains(reply, "**") {
			t.Fatalf("markdown in reply %q", reply)
		}
		if n := utf8.RuneCountInString(reply); n > 110 {
			t.Fatalf("reply too long (%d): %s", n, reply)
		}
	}
}

func TestDeepSeekParsesAndFallsBack(t *testing.T) {
	okSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"先看输入，再看输出。"}}]}`)
	}))
	defer okSrv.Close()

	d := NewDeepSeek("test-key", okSrv.URL, "deepseek-chat", "sys", 0, 80, 0.5, nil)
	res := d.Reply(context.Background(), "小鱼干", "讲讲这段代码")
	if res.Err != nil || res.Mock || res.Reply != "先看输入，再看输出。" {
		t.Fatalf("result = %+v", res)
	}

	badSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer badSrv.Close()
	d2 := NewDeepSeek("test-key", badSrv.URL, "deepseek-chat", "sys", 0, 80, 0.5, nil)
	res = d2.Reply(context.Background(), "小鱼干", "nil 是什么")
	if !res.Mock || !res.Degraded || res.Err == nil || strings.TrimSpace(res.Reply) == "" {
		t.Fatalf("fallback = %+v", res)
	}
}

func TestTidyReplyStripsMarkdown(t *testing.T) {
	in := "```go\nfmt.Println(1)\n```\n**先看输入。** 再看输出。"
	got := TidyReply(in)
	if strings.Contains(got, "`") || strings.Contains(got, "*") {
		t.Fatalf("tidy = %q", got)
	}
	if !strings.Contains(got, "先看输入") {
		t.Fatalf("tidy = %q", got)
	}
}
