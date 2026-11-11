package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"betterscreen/internal/screen"
)

// TestEndToEnd drives a real screen session through the whole API.
func TestEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("screen"); err != nil {
		t.Skip("screen not on PATH")
	}
	srv := httptest.NewServer(New(Options{Screen: screen.NewClient(), Token: testToken}))
	t.Cleanup(srv.Close)

	call := func(method, path, body string) (*http.Response, []byte) {
		t.Helper()
		req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+testToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return resp, data
	}
	stateOf := func(id string) string {
		_, data := call("GET", "/api/sessions", "")
		var list []sessionJSON
		if err := json.Unmarshal(data, &list); err != nil {
			t.Fatalf("decode %s: %v", data, err)
		}
		for _, s := range list {
			if s.ID == id {
				return s.State
			}
		}
		return ""
	}
	open := func(query string) *websocket.Conn {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		c, _, err := websocket.Dial(ctx, wsURL(srv, query), &websocket.DialOptions{Subprotocols: tokenProtocols()})
		if err != nil {
			t.Fatal(err)
		}
		c.SetReadLimit(1 << 20)
		t.Cleanup(func() { c.CloseNow() })
		return c
	}
	// typeAndWait sends a command whose output differs from its own echo, so
	// seeing want proves the shell ran it.
	typeAndWait := func(c *websocket.Conn, input, want string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := c.Write(ctx, websocket.MessageBinary, []byte(input)); err != nil {
			t.Fatal(err)
		}
		var out strings.Builder
		for !strings.Contains(out.String(), want) {
			_, data, err := c.Read(ctx)
			if err != nil {
				t.Fatalf("waiting for %q: %v", want, err)
			}
			out.Write(data)
		}
	}

	// 1. Create.
	name := fmt.Sprintf("bse2e%d", rand.Intn(1e6))
	resp, data := call("POST", "/api/sessions", `{"name":"`+name+`"}`)
	if resp.StatusCode != 201 {
		t.Fatalf("create: %d %s", resp.StatusCode, data)
	}
	var created sessionJSON
	if err := json.Unmarshal(data, &created); err != nil {
		t.Fatal(err)
	}
	id := created.ID
	t.Cleanup(func() { _ = exec.Command("screen", "-S", id, "-X", "quit").Run() })

	// 2. Share, resize, type, read back.
	c := open("/ws/sessions/" + id)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageText, []byte(`{"type":"resize","cols":100,"rows":30}`)); err != nil {
		t.Fatal(err)
	}
	typeAndWait(c, "echo e2e-$((40+2))\r", "e2e-42")

	// 3. Closing the socket detaches the display; the session survives.
	if err := c.Close(websocket.StatusNormalClosure, ""); err != nil {
		t.Logf("close: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for stateOf(id) != "detached" {
		if time.Now().After(deadline) {
			t.Fatalf("state = %q after close, want detached", stateOf(id))
		}
		time.Sleep(100 * time.Millisecond)
	}

	// 4. A take kicks a share out with close code 4000.
	a := open("/ws/sessions/" + id)
	typeAndWait(a, "echo a-$((1+1))\r", "a-2")
	b := open("/ws/sessions/" + id + "?mode=take")
	readCtx, readCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer readCancel()
	var err error
	for err == nil {
		_, _, err = a.Read(readCtx)
	}
	if got := websocket.CloseStatus(err); got != 4000 {
		t.Errorf("shared connection closed with %v (%v), want 4000", got, err)
	}
	typeAndWait(b, "echo b-$((2+1))\r", "b-3")
	b.Close(websocket.StatusNormalClosure, "")

	// 5. Kill.
	if resp, data := call("DELETE", "/api/sessions/"+id, ""); resp.StatusCode != 204 {
		t.Fatalf("delete: %d %s", resp.StatusCode, data)
	}
	deadline = time.Now().Add(3 * time.Second)
	for stateOf(id) != "" {
		if time.Now().After(deadline) {
			t.Fatal("session still listed after DELETE")
		}
		time.Sleep(100 * time.Millisecond)
	}
}
