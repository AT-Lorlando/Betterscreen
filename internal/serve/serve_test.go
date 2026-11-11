package serve

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func env(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func TestParseFlagsDefaults(t *testing.T) {
	cfg, err := parseFlags(nil, env(map[string]string{"XDG_CONFIG_HOME": ""}), "/h")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":8443" {
		t.Errorf("Addr = %q, want :8443", cfg.Addr)
	}
	if cfg.ConfigDir != "/h/.config/betterscreen" {
		t.Errorf("ConfigDir = %q", cfg.ConfigDir)
	}
}

func TestParseFlagsXDG(t *testing.T) {
	cfg, err := parseFlags(nil, env(map[string]string{"XDG_CONFIG_HOME": "/x"}), "/h")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConfigDir != "/x/betterscreen" {
		t.Errorf("ConfigDir = %q, want /x/betterscreen", cfg.ConfigDir)
	}
}

func TestParseFlagsOrigins(t *testing.T) {
	cfg, err := parseFlags([]string{"--allow-origin", "https://a.lan", "--allow-origin", "http://b.lan:3000"}, env(nil), "/h")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"https://a.lan", "http://b.lan:3000"}; !slices.Equal(cfg.AllowedOrigins, want) {
		t.Errorf("AllowedOrigins = %v, want %v", cfg.AllowedOrigins, want)
	}
	for _, bad := range []string{"front.lan", "https://front.lan/app", "ftp://front.lan"} {
		if _, err := parseFlags([]string{"--allow-origin", bad}, env(nil), "/h"); err == nil {
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
