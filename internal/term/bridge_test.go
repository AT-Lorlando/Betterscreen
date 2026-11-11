package term

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// harness serves one WebSocket endpoint bridged to the command built by mk,
// dials it, and reports Bridge's return on done.
type harness struct {
	conn *websocket.Conn
	cmd  *exec.Cmd
	done chan error
}

func start(t *testing.T, name string, args ...string) *harness {
	t.Helper()
	h := &harness{done: make(chan error, 1), cmd: exec.Command(name, args...)}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			h.done <- err
			return
		}
		h.done <- Bridge(context.Background(), c, h.cmd)
	}))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	c.SetReadLimit(1 << 20)
	h.conn = c
	t.Cleanup(func() { c.CloseNow() })
	return h
}

func (h *harness) send(t *testing.T, typ websocket.MessageType, data string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.conn.Write(ctx, typ, []byte(data)); err != nil {
		t.Fatal(err)
	}
}

// waitFor reads binary frames until the accumulated output contains want.
func (h *harness) waitFor(t *testing.T, want string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var out strings.Builder
	for !strings.Contains(out.String(), want) {
		_, data, err := h.conn.Read(ctx)
		if err != nil {
			t.Fatalf("waiting for %q, got %q then %v", want, out.String(), err)
		}
		out.Write(data)
	}
}

func TestBridgeEcho(t *testing.T) {
	h := start(t, "cat")
	h.send(t, websocket.MessageBinary, "hello\n")
	h.waitFor(t, "hello")
}

func TestBridgeResize(t *testing.T) {
	h := start(t, "sh", "-c", "read x; stty size")
	h.send(t, websocket.MessageText, `{"type":"resize","cols":120,"rows":40}`)
	h.send(t, websocket.MessageBinary, "\n")
	h.waitFor(t, "40 120")
}

func TestBridgeIgnoresBadControl(t *testing.T) {
	h := start(t, "cat")
	h.send(t, websocket.MessageText, `not json`)
	h.send(t, websocket.MessageText, `{"type":"resize","cols":0,"rows":0}`)
	h.send(t, websocket.MessageText, `{"type":"other"}`)
	h.send(t, websocket.MessageBinary, "ok\n")
	h.waitFor(t, "ok")
}

func TestBridgeLargePaste(t *testing.T) {
	// READY is printed once the tty is raw: a canonical-mode tty would
	// truncate a 200 000-byte line to its 4 KiB line buffer.
	h := start(t, "sh", "-c", "stty raw -echo; echo READY; head -c 200000 | wc -c")
	h.waitFor(t, "READY")
	h.send(t, websocket.MessageBinary, strings.Repeat("x", 200000))
	h.waitFor(t, "200000")
}

func TestBridgeCommandExit(t *testing.T) {
	h := start(t, "true")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var err error
	for err == nil {
		_, _, err = h.conn.Read(ctx)
	}
	if got := websocket.CloseStatus(err); got != CloseScreenExited {
		t.Errorf("close status = %v (%v), want %v", got, err, CloseScreenExited)
	}
	select {
	case berr := <-h.done:
		if berr != nil {
			t.Errorf("Bridge returned %v", berr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Bridge did not return after the command exited")
	}
}

func TestBridgeClientClose(t *testing.T) {
	h := start(t, "sleep", "30")
	time.Sleep(200 * time.Millisecond) // let the command start
	if err := h.conn.Close(websocket.StatusNormalClosure, ""); err != nil && !errors.Is(err, context.Canceled) {
		t.Logf("close: %v", err)
	}
	select {
	case <-h.done:
	case <-time.After(3 * time.Second):
		t.Fatal("Bridge did not return within 3s of the client closing")
	}
	if h.cmd.ProcessState == nil {
		t.Fatal("command was not waited for: process may still run")
	}
	// A hangup lets screen detach its display cleanly; a kill is the fallback.
	ws := h.cmd.ProcessState.Sys().(syscall.WaitStatus)
	if !ws.Signaled() || ws.Signal() != syscall.SIGHUP {
		t.Errorf("command ended with %v, want SIGHUP", h.cmd.ProcessState)
	}
}
