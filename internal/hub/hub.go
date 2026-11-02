package hub

import (
	"encoding/json"
	"sync"
)

// Hub fans one JSON event out to every connected pet page.
type Hub struct {
	mu   sync.Mutex
	subs map[chan []byte]struct{}
}

func New() *Hub {
	return &Hub{subs: map[chan []byte]struct{}{}}
}

// Publish marshals v once and delivers it to subscribers.
// A slow page drops the event instead of blocking the speaker.
func (h *Hub) Publish(v any) {
	if h == nil {
		return
	}
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- b:
		default:
		}
	}
}

// Subscribe returns a buffered stream of already-encoded events.
// Call the cancel func when the socket closes.
func (h *Hub) Subscribe() (<-chan []byte, func()) {
	ch := make(chan []byte, 32)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			if _, ok := h.subs[ch]; !ok {
				return
			}
			delete(h.subs, ch)
			close(ch)
		})
	}
	return ch, cancel
}
