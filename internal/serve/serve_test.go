package serve

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestParseFlagsDefaults(t *testing.T) {
	cfg, err := parseFlags(nil, "/h")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != "" {
		t.Errorf("Addr = %q, want empty (resolved later from config.json)", cfg.Addr)
	}
	if cfg.ConfigDir != "/h/.betterscreen" {
		t.Errorf("ConfigDir = %q, want /h/.betterscreen", cfg.ConfigDir)
	}
}

func TestParseFlagsOrigins(t *testing.T) {
	cfg, err := parseFlags([]string{"--allow-origin", "https://a.lan", "--allow-origin", "http://b.lan:3000"}, "/h")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"https://a.lan", "http://b.lan:3000"}; !slices.Equal(cfg.AllowedOrigins, want) {
		t.Errorf("AllowedOrigins = %v, want %v", cfg.AllowedOrigins, want)
	}
	for _, bad := range []string{"front.lan", "https://front.lan/app", "ftp://front.lan"} {
		if _, err := parseFlags([]string{"--allow-origin", bad}, "/h"); err == nil {
			t.Errorf("origin %q accepted", bad)
		}
	}
}

// syncBuffer is a bytes.Buffer safe for the concurrent writes of Run's logger.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// runUntilListening starts Run and waits for its "listening on" line.
func runUntilListening(t *testing.T, dir string, port int) (out *syncBuffer, stop func() error) {
	t.Helper()
	out = &syncBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, []string{"--addr", fmt.Sprintf("127.0.0.1:%d", port), "--config-dir", dir}, out)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(out.String(), "listening on") {
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("server never listened; output:\n%s", out.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	return out, func() error {
		cancel()
		select {
		case err := <-done:
			return err
		case <-time.After(6 * time.Second):
			return fmt.Errorf("Run did not return within 6s")
		}
	}
}

func TestRunServes(t *testing.T) {
	dir := t.TempDir()
	port := freePort(t)
	out, stop := runUntilListening(t, dir, port)
	if _, err := os.Stat(filepath.Join(dir, "config.json")); err != nil {
		t.Errorf("config.json not created: %v", err)
	}
	for _, want := range []string{"token:", "TLS certificate SHA-256:"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	resp, err := client.Get(fmt.Sprintf("https://127.0.0.1:%d/api/sessions", port))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
	if err := stop(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out, stop = runUntilListening(t, dir, freePort(t))
	if strings.Contains(out.String(), "token:") {
		t.Errorf("token printed again on second start:\n%s", out.String())
	}
	if err := stop(); err != nil {
		t.Fatalf("second Run: %v", err)
	}
}

// An open SSE stream must not stall shutdown or make it fail.
func TestRunStopsWithOpenStream(t *testing.T) {
	dir := t.TempDir()
	port := freePort(t)
	_, stop := runUntilListening(t, dir, port)
	fc, _, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	req, _ := http.NewRequest("GET", fmt.Sprintf("https://127.0.0.1:%d/api/events", port), nil)
	req.Header.Set("Authorization", "Bearer "+fc.Token)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	start := time.Now()
	if err := stop(); err != nil {
		t.Errorf("Run returned %v with a stream open, want nil", err)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("shutdown took %v with a stream open, want < 2s", d)
	}
}

// Idle keep-alive connections are closed, so clients without the token
// cannot pile up connections.
func TestRunClosesIdleConnections(t *testing.T) {
	old := idleTimeout
	idleTimeout = 200 * time.Millisecond
	defer func() { idleTimeout = old }()
	port := freePort(t)
	_, stop := runUntilListening(t, t.TempDir(), port)
	defer stop()

	conn, err := tls.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port), &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	fmt.Fprintf(conn, "GET /api/sessions HTTP/1.1\r\nHost: x\r\n\r\n")
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := br.ReadByte(); err != io.EOF {
		t.Errorf("idle connection not closed by the server: read returned %v, want EOF", err)
	}
}

// Without --addr, the listen address comes from config.json.
func TestRunUsesConfigAddr(t *testing.T) {
	dir := t.TempDir()
	port := freePort(t)
	writeConfig(t, dir, fmt.Sprintf(`{"token":"%s","addr":"127.0.0.1:%d"}`, goodToken, port))
	out := &syncBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, []string{"--config-dir", dir}, out) }()
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(out.String(), "listening on") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run: %v; output:\n%s", err, out.String())
	}
	if want := fmt.Sprintf("listening on https://127.0.0.1:%d", port); !strings.Contains(out.String(), want) {
		t.Errorf("output lacks %q:\n%s", want, out.String())
	}
}
