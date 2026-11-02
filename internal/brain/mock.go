package brain

import (
	"context"
	"strings"

	"whale-live/internal/textutil"
)

// Local is the offline persona used when no API key is configured,
// and as a fallback when DeepSeek cannot answer.
type Local struct{}

func (Local) Mode() string { return "mock" }

func (Local) Reply(_ context.Context, user, text string) Result {
	return Result{Reply: MockReply(user, text), Mock: true}
}

// Fallback is the last-resort line if every other path returns empty text.
func Fallback(user string) string {
	name := textutil.Clip(strings.TrimSpace(user), 8)
	if name == "" {
		name = "朋友"
	}
	return name + "，我在的。我们先看输入从哪来、最后想要什么，中间只抓一个点。"
}

// MockReply writes a short in-character answer without calling the network.
func MockReply(user, text string) string {
	name := textutil.Clip(strings.TrimSpace(user), 8)
	if name == "" {
		name = "朋友"
	}
	s := strings.TrimSpace(text)
	var reply string
	switch {
	case containsAny(s, "这段代码", "讲讲这段", "帮我讲"):
		reply = "行，我帮你看。先别通读整屏，找到这段的输入和它最后交出去的结果，中间只盯住一层循环或者一次调用。"
	case containsAny(s, "nil", "空指针", "空指标"):
		reply = "空指针就是手里还没有东西，就急着去用。顺着调用往上找，看是谁该把对象交出来，结果交了个空。"
	case containsAny(s, "递归"):
		reply = "递归就是同一件事换小一号再做一遍。先写清楚什么时候停，再看这一层怎么把问题变小。"
	case containsAny(s, "channel", "通道", "阻塞"):
		reply = "通道像一根小管子，一边放一边取。堵住了，多半是有人在等对面。看看是不是忘了送，或者没人来取。"
	case containsAny(s, "goroutine", "协程", "并发"):
		reply = "协程就是多请了几个帮手一起干。先问他们会不会抢同一件东西，会的话就交给通道或者锁。"
	case containsAny(s, "for", "range", "循环"):
		reply = "循环就是同一件事做很多遍。手里已经有一串东西就用 range，要自己数到几就用普通 for。"
	case containsAny(s, "slice", "切片"):
		reply = "切片像一张会变长的清单。追加时有时会换一张更大的新纸，别让别人一直攥着旧纸的一角。"
	case containsAny(s, "map", "字典"):
		reply = "map 像贴了标签的抽屉。拿之前先看这个标签在不在，不在就别直接拆开用。"
	case containsAny(s, "interface", "接口"):
		reply = "接口只约定会做什么，不约定是谁来做。手里拿着接口时，先确认里面真有人，再喊它干活。"
	case containsAny(s, "指针", "pointer"):
		reply = "指针是一张写着地址的小纸条。纸条是空的就别顺着找。有值的时候，改它会影响原来的那一份。"
	case containsAny(s, "panic", "报错", "error", "错误", "异常"):
		reply = "先别慌。看错误最后那句人话，再对上你刚改过的几行，原因通常就在附近。"
	case containsAny(s, "你好", "hello", "嗨", "在吗", "早上好", "下午好", "晚上好", "可爱", "喜欢"):
		reply = name + "，我在呢。把看不懂的那一行丢过来，我用大白话帮你拆开。"
	default:
		reply = name + "，我听到啦。我们先只抓一个点：输入是什么，你希望它变成什么，中间差的那一步就是今天要讲的。"
	}
	return textutil.ClipSentence(reply, 110)
}

func containsAny(text string, keys ...string) bool {
	low := strings.ToLower(text)
	for _, k := range keys {
		if k == "" {
			continue
		}
		if strings.Contains(text, k) || strings.Contains(low, strings.ToLower(k)) {
			return true
		}
	}
	return false
}
