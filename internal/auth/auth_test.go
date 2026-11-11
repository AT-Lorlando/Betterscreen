package auth

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadOrCreateGenerates(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cfg")
	token, created, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Error("created = false, want true")
	}
	if len(token) != 43 {
		t.Errorf("len(token) = %d, want 43", len(token))
	}
	fi, err := os.Stat(filepath.Join(dir, "token"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", fi.Mode().Perm())
	}
}

func TestLoadOrCreateReuses(t *testing.T) {
	dir := t.TempDir()
	first, _, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatal(err)
	}
	second, created, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Error("created = true on second call")
	}
	if first != second {
		t.Errorf("token changed: %q then %q", first, second)
	}
}

func TestLoadOrCreateRejectsShortToken(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "token"), []byte("abc\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadOrCreate(dir); err == nil {
		t.Error("expected an error for a short token")
	}
}

const tok = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFG"

func reqWith(header, value string) *http.Request {
	r := httptest.NewRequest("GET", "/api/sessions", nil)
	if header != "" {
		r.Header.Set(header, value)
	}
	return r
}

func TestCheckBearer(t *testing.T) {
	cases := []struct {
		name, header, value string
		want                bool
	}{
		{"right token", "Authorization", "Bearer " + tok, true},
		{"wrong token", "Authorization", "Bearer nope", false},
		{"missing header", "", "", false},
		{"basic scheme", "Authorization", "Basic " + tok, false},
	}
	for _, c := range cases {
		if got := Check(reqWith(c.header, c.value), tok); got != c.want {
			t.Errorf("%s: Check = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCheckSubprotocol(t *testing.T) {
	if !Check(reqWith("Sec-WebSocket-Protocol", "betterscreen, token."+tok), tok) {
		t.Error("valid token subprotocol rejected")
	}
	if Check(reqWith("Sec-WebSocket-Protocol", "betterscreen, token.wrong"), tok) {
		t.Error("wrong token subprotocol accepted")
	}
	r := httptest.NewRequest("GET", "/ws/sessions/1.a", nil)
	r.Header.Add("Sec-WebSocket-Protocol", "betterscreen")
	r.Header.Add("Sec-WebSocket-Protocol", "token."+tok)
	if !Check(r, tok) {
		t.Error("token on a second Sec-WebSocket-Protocol line rejected")
	}
}

func TestCheckEmptyConfiguredToken(t *testing.T) {
	if Check(reqWith("Authorization", "Bearer "), "") {
		t.Error("empty configured token must never authenticate")
	}
}

func TestMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	h := Middleware(tok, next)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, reqWith("", ""))
	if rec.Code != 401 {
		t.Errorf("unauthenticated status = %d, want 401", rec.Code)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `{"error":"unauthorized"}` {
		t.Errorf("body = %q", body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, reqWith("Authorization", "Bearer "+tok))
	if rec.Code != 200 {
		t.Errorf("authenticated status = %d, want 200", rec.Code)
	}
}
