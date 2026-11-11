package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"betterscreen/internal/screen"
)

// hub polls the session list while at least one subscriber exists and pushes
// the list to every subscriber whenever it changes.
type hub struct {
	list     func() ([]screen.Session, error)
	interval time.Duration

	mu   sync.Mutex
	subs map[chan []byte]struct{}
	last []byte
	stop chan struct{}
}

func newHub(list func() ([]screen.Session, error), interval time.Duration) *hub {
	return &hub{list: list, interval: interval, subs: map[chan []byte]struct{}{}}
}

// subscribe registers a subscriber. Its channel receives the current list
// first, then every change. The returned func unsubscribes.
func (h *hub) subscribe() (<-chan []byte, func()) {
	ch := make(chan []byte, 1)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	if len(h.subs) == 1 {
		h.last = nil
		h.stop = make(chan struct{})
		go h.run(h.stop)
	} else if h.last != nil {
		offer(ch, h.last)
	}
	h.mu.Unlock()

	var once sync.Once
	return ch, func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			delete(h.subs, ch)
			if len(h.subs) == 0 {
				close(h.stop)
			}
		})
	}
}

func (h *hub) run(stop chan struct{}) {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()
	for {
		h.poll(stop)
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
	}
}

func (h *hub) poll(stop chan struct{}) {
	sessions, err := h.list() // outside the lock: screen -ls is slow
	if err != nil {
		return
	}
	data, err := json.Marshal(toSessionsJSON(sessions))
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	select {
	case <-stop:
		return // unsubscribed while polling
	default:
	}
	if bytes.Equal(data, h.last) {
		return
	}
	h.last = data
	for ch := range h.subs {
		offer(ch, data)
	}
}

// offer delivers v without blocking: a pending, unread value is replaced, so a
// slow subscriber only ever gets the latest list.
func offer(ch chan []byte, v []byte) {
	select {
	case ch <- v:
		return
	default:
	}
	select {
	case <-ch:
	default:
	}
	select {
	case ch <- v:
	default:
	}
}

func (s *server) events(w http.ResponseWriter, r *http.Request) {
	ch, cancel := s.hub.subscribe()
	defer cancel()

	rc := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_ = rc.Flush()

	keepAlive := time.NewTicker(s.keepAlive)
	defer keepAlive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case data := <-ch:
			if _, err := fmt.Fprintf(w, "event: sessions\ndata: %s\n\n", data); err != nil {
				return
			}
		case <-keepAlive.C:
			if _, err := io.WriteString(w, ": keep-alive\n\n"); err != nil {
				return
			}
		}
		if err := rc.Flush(); err != nil {
			return
		}
	}
}
