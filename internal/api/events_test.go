package api

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"betterscreen/internal/screen"
)

func sessionNamed(name string) []screen.Session {
	return []screen.Session{{PID: 1, Name: name, ID: "1." + name}}
}

// openEvents starts the handler on a real server and streams the SSE lines.
func openEvents(t *testing.T, f *fakeScreen) <-chan string {
	t.Helper()
	h := New(Options{Screen: f, Token: testToken, PollInterval: 20 * time.Millisecond, KeepAlive: 50 * time.Millisecond})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	req, _ := http.NewRequest("GET", srv.URL+"/api/events", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q", ct)
	}
	lines := make(chan string, 100)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	return lines
}

// nextData waits for the next `data:` line following an `event: sessions` line.
func nextData(t *testing.T, lines <-chan string, timeout time.Duration) (string, bool) {
	t.Helper()
	deadline := time.After(timeout)
	sawEvent := false
	for {
		select {
		case l, ok := <-lines:
			if !ok {
				return "", false
			}
			if l == "event: sessions" {
				sawEvent = true
			} else if d, ok := strings.CutPrefix(l, "data: "); ok && sawEvent {
				return d, true
			}
		case <-deadline:
			return "", false
		}
	}
}

func TestEventsInitial(t *testing.T) {
	lines := openEvents(t, &fakeScreen{sessions: sessionNamed("a")})
	d, ok := nextData(t, lines, 500*time.Millisecond)
	if !ok || d != `[{"id":"1.a","pid":1,"name":"a","state":"detached"}]` {
		t.Errorf("first event = %q (ok=%v)", d, ok)
	}
}

func TestEventsOnChange(t *testing.T) {
	f := &fakeScreen{sessions: sessionNamed("a")}
	lines := openEvents(t, f)
	if _, ok := nextData(t, lines, 500*time.Millisecond); !ok {
		t.Fatal("no initial event")
	}
	f.setSessions(sessionNamed("b"))
	d, ok := nextData(t, lines, 500*time.Millisecond)
	if !ok || !strings.Contains(d, `"name":"b"`) {
		t.Errorf("change event = %q (ok=%v)", d, ok)
	}
}

func TestEventsNoDuplicate(t *testing.T) {
	lines := openEvents(t, &fakeScreen{sessions: sessionNamed("a")})
	if _, ok := nextData(t, lines, 500*time.Millisecond); !ok {
		t.Fatal("no initial event")
	}
	if d, ok := nextData(t, lines, 200*time.Millisecond); ok {
		t.Errorf("unexpected second event %q for an unchanged list", d)
	}
}

func TestEventsKeepAlive(t *testing.T) {
	lines := openEvents(t, &fakeScreen{})
	deadline := time.After(500 * time.Millisecond)
	for {
		select {
		case l := <-lines:
			if l == ": keep-alive" {
				return
			}
		case <-deadline:
			t.Fatal("no keep-alive comment")
		}
	}
}

func TestHubSlowSubscriber(t *testing.T) {
	f := &fakeScreen{sessions: sessionNamed("s0")}
	h := newHub(f.ListSessions, 10*time.Millisecond)
	_, cancelStuck := h.subscribe() // never read
	defer cancelStuck()
	live, cancel := h.subscribe()
	defer cancel()
	for i := 1; i <= 5; i++ {
		f.setSessions(sessionNamed("s" + string(rune('0'+i))))
		time.Sleep(25 * time.Millisecond)
	}
	deadline := time.After(500 * time.Millisecond)
	for {
		select {
		case data := <-live:
			if strings.Contains(string(data), `"name":"s5"`) {
				return
			}
		case <-deadline:
			t.Fatal("live subscriber never received the latest list")
		}
	}
}

func TestHubStopsWithoutSubscribers(t *testing.T) {
	f := &fakeScreen{}
	h := newHub(f.ListSessions, 10*time.Millisecond)
	_, cancel := h.subscribe()
	time.Sleep(50 * time.Millisecond)
	cancel()
	time.Sleep(30 * time.Millisecond) // let an in-flight poll finish
	f.mu.Lock()
	before := f.listCalls
	f.mu.Unlock()
	time.Sleep(100 * time.Millisecond)
	f.mu.Lock()
	after := f.listCalls
	f.mu.Unlock()
	if before == 0 || after != before {
		t.Errorf("listCalls before=%d after=%d: poller must run then stop", before, after)
	}
}
