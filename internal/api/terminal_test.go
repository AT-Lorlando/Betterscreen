package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func wsURL(srv *httptest.Server, path string) string {
	return "ws" + strings.TrimPrefix(srv.URL, "http") + path
}

func tokenProtocols() []string { return []string{"betterscreen", "token." + testToken} }

// dial opens a WebSocket on h; it returns the conn (nil on failure), the
// handshake response and the error.
func dial(t *testing.T, h http.Handler, path string, opts *websocket.DialOptions) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, resp, err := websocket.Dial(ctx, wsURL(srv, path), opts)
	if c != nil {
		t.Cleanup(func() { c.CloseNow() })
	}
	return c, resp, err
}

func echoes(t *testing.T, c *websocket.Conn, msg string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageBinary, []byte(msg+"\n")); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	for !strings.Contains(out.String(), msg) {
		_, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("no echo of %q: got %q, %v", msg, out.String(), err)
		}
		out.Write(data)
	}
}

func status(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}

func TestTerminalShareDefault(t *testing.T) {
	f := &fakeScreen{sessions: attachedA()}
	c, _, err := dial(t, newTestHandler(f), "/ws/sessions/1.a", &websocket.DialOptions{Subprotocols: tokenProtocols()})
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Subprotocol(); got != "betterscreen" {
		t.Errorf("subprotocol = %q, want betterscreen", got)
	}
	echoes(t, c, "ping")
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.shareSession != "1.a" || f.shareWindow != 0 {
		t.Errorf("share %q/%d, want 1.a/0", f.shareSession, f.shareWindow)
	}
}

func TestTerminalTakeWithWindow(t *testing.T) {
	f := &fakeScreen{sessions: attachedA()}
	c, _, err := dial(t, newTestHandler(f), "/ws/sessions/1.a?mode=take&window=3", &websocket.DialOptions{Subprotocols: tokenProtocols()})
	if err != nil {
		t.Fatal(err)
	}
	echoes(t, c, "ping")
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.attachSession != "1.a" || f.attachWindow != 3 {
		t.Errorf("attach %q/%d, want 1.a/3", f.attachSession, f.attachWindow)
	}
}

func TestTerminalBadMode(t *testing.T) {
	f := &fakeScreen{sessions: attachedA()}
	_, resp, err := dial(t, newTestHandler(f), "/ws/sessions/1.a?mode=steal", &websocket.DialOptions{Subprotocols: tokenProtocols()})
	if err == nil || status(resp) != 400 {
		t.Errorf("status = %d (err %v), want 400", status(resp), err)
	}
	if f.commandsBuilt != 0 {
		t.Error("a command was built for an invalid mode")
	}
}

func TestTerminalBadWindow(t *testing.T) {
	for _, q := range []string{"window=abc", "window=100", "window=-1"} {
		f := &fakeScreen{sessions: attachedA()}
		_, resp, err := dial(t, newTestHandler(f), "/ws/sessions/1.a?"+q, &websocket.DialOptions{Subprotocols: tokenProtocols()})
		if err == nil || status(resp) != 400 {
			t.Errorf("%s: status = %d (err %v), want 400", q, status(resp), err)
		}
	}
}

func TestTerminalUnknownSession(t *testing.T) {
	f := &fakeScreen{sessions: attachedA()}
	_, resp, err := dial(t, newTestHandler(f), "/ws/sessions/9.nope", &websocket.DialOptions{Subprotocols: tokenProtocols()})
	if err == nil || status(resp) != 404 {
		t.Errorf("status = %d (err %v), want 404", status(resp), err)
	}
}

func TestTerminalNoToken(t *testing.T) {
	f := &fakeScreen{sessions: attachedA()}
	_, resp, err := dial(t, newTestHandler(f), "/ws/sessions/1.a", &websocket.DialOptions{Subprotocols: []string{"betterscreen"}})
	if err == nil || status(resp) != 401 {
		t.Errorf("status = %d (err %v), want 401", status(resp), err)
	}
}

func TestTerminalTokenInQueryRefused(t *testing.T) {
	f := &fakeScreen{sessions: attachedA()}
	_, resp, err := dial(t, newTestHandler(f), "/ws/sessions/1.a?token="+testToken, nil)
	if err == nil || status(resp) != 401 {
		t.Errorf("status = %d (err %v), want 401", status(resp), err)
	}
}

func TestTerminalForeignOrigin(t *testing.T) {
	opts := func() *websocket.DialOptions {
		return &websocket.DialOptions{
			Subprotocols: tokenProtocols(),
			HTTPHeader:   http.Header{"Origin": {"https://evil.lan"}},
		}
	}
	f := &fakeScreen{sessions: attachedA()}
	_, resp, err := dial(t, newTestHandler(f), "/ws/sessions/1.a", opts())
	if err == nil || status(resp) != 403 {
		t.Errorf("foreign origin: status = %d (err %v), want 403", status(resp), err)
	}
	h := New(Options{Screen: f, Token: testToken, AllowedOrigins: []string{"https://evil.lan"}})
	if _, _, err := dial(t, h, "/ws/sessions/1.a", opts()); err != nil {
		t.Errorf("allowed origin rejected: %v", err)
	}
}
