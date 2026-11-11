package api

import (
	"net/http/httptest"
	"testing"
)

func preflight(origin string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("OPTIONS", "/api/sessions", nil)
	r.Header.Set("Origin", origin)
	r.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	newTestHandler(&fakeScreen{}).ServeHTTP(rec, r)
	return rec
}

func TestPreflightAllowedOrigin(t *testing.T) {
	rec := preflight("https://front.lan")
	if rec.Code != 204 {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	want := map[string]string{
		"Access-Control-Allow-Origin":  "https://front.lan",
		"Access-Control-Allow-Headers": "Authorization, Content-Type",
		"Access-Control-Allow-Methods": "GET, POST, DELETE",
	}
	for k, v := range want {
		if got := rec.Header().Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestPreflightUnknownOrigin(t *testing.T) {
	rec := preflight("https://evil.lan")
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want none", got)
	}
}
