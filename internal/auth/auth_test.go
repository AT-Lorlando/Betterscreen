package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewToken(t *testing.T) {
	a, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 43 {
		t.Errorf("len(token) = %d, want 43", len(a))
	}
	if len(a) < MinTokenLen {
		t.Errorf("len(token) = %d, below MinTokenLen %d", len(a), MinTokenLen)
	}
	if a == b {
		t.Error("two tokens are equal")
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
