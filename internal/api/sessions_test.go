package api

import (
	"bytes"
	"log"
	"net/http/httptest"
	"strings"
	"testing"

	"betterscreen/internal/screen"
)

func attachedA() []screen.Session {
	return []screen.Session{{PID: 1, Name: "a", ID: "1.a", State: screen.StateAttached}}
}

func assertBody(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantBody string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Errorf("status = %d, want %d (body %q)", rec.Code, wantStatus, rec.Body.String())
	}
	if wantBody != "" {
		if got := strings.TrimSpace(rec.Body.String()); got != wantBody {
			t.Errorf("body = %s, want %s", got, wantBody)
		}
	}
}

func TestListSessions(t *testing.T) {
	h := newTestHandler(&fakeScreen{sessions: attachedA()})
	assertBody(t, do(t, h, "GET", "/api/sessions", ""), 200,
		`[{"id":"1.a","pid":1,"name":"a","state":"attached"}]`)
}

func TestListSessionsEmpty(t *testing.T) {
	h := newTestHandler(&fakeScreen{})
	assertBody(t, do(t, h, "GET", "/api/sessions", ""), 200, `[]`)
}

func TestCreateSession(t *testing.T) {
	f := &fakeScreen{}
	rec := do(t, newTestHandler(f), "POST", "/api/sessions", `{"name":"work"}`)
	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201 (body %q)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"name":"work"`) {
		t.Errorf("body = %s, want the created session", rec.Body.String())
	}
	if f.created != "work" {
		t.Errorf("created = %q, want work", f.created)
	}
}

func TestCreateSessionInvalidName(t *testing.T) {
	for _, name := range []string{"", "-X", "a b", strings.Repeat("a", 65)} {
		f := &fakeScreen{}
		rec := do(t, newTestHandler(f), "POST", "/api/sessions", `{"name":"`+name+`"}`)
		if rec.Code != 400 {
			t.Errorf("name %q: status = %d, want 400", name, rec.Code)
		}
		if f.created != "" {
			t.Errorf("name %q: session created anyway", name)
		}
	}
}

func TestCreateSessionDuplicate(t *testing.T) {
	f := &fakeScreen{sessions: []screen.Session{{PID: 1, Name: "work", ID: "1.work"}}}
	assertBody(t, do(t, newTestHandler(f), "POST", "/api/sessions", `{"name":"work"}`), 409, "")
	if f.created != "" {
		t.Error("duplicate session created")
	}
}

func TestCreateSessionMalformedJSON(t *testing.T) {
	assertBody(t, do(t, newTestHandler(&fakeScreen{}), "POST", "/api/sessions", `{`), 400, "")
}

func TestKillSession(t *testing.T) {
	f := &fakeScreen{sessions: attachedA()}
	assertBody(t, do(t, newTestHandler(f), "DELETE", "/api/sessions/1.a", ""), 204, "")
	if f.killed != "1.a" {
		t.Errorf("killed = %q, want 1.a", f.killed)
	}
}

func TestUnknownSession(t *testing.T) {
	f := &fakeScreen{sessions: attachedA()}
	assertBody(t, do(t, newTestHandler(f), "DELETE", "/api/sessions/9.nope", ""), 404, "")
}

func TestRejectsDashID(t *testing.T) {
	f := &fakeScreen{sessions: []screen.Session{{PID: 1, Name: "X", ID: "-X"}}}
	assertBody(t, do(t, newTestHandler(f), "DELETE", "/api/sessions/-X", ""), 404, "")
	if f.killed != "" {
		t.Errorf("KillSession called with %q", f.killed)
	}
}

func TestListWindows(t *testing.T) {
	f := &fakeScreen{
		sessions: attachedA(),
		windows:  []screen.Window{{Num: 0, Title: "zsh"}},
		details:  map[int]screen.Detail{0: {Pwd: "/tmp", Proc: "claude"}},
	}
	assertBody(t, do(t, newTestHandler(f), "GET", "/api/sessions/1.a/windows", ""), 200,
		`[{"num":0,"title":"zsh","pwd":"/tmp","proc":"claude"}]`)
}

func TestDetach(t *testing.T) {
	f := &fakeScreen{sessions: attachedA()}
	assertBody(t, do(t, newTestHandler(f), "POST", "/api/sessions/1.a/detach", ""), 204, "")
	if f.detached != "1.a" {
		t.Errorf("detached = %q, want 1.a", f.detached)
	}
}

func TestSelectWindow(t *testing.T) {
	f := &fakeScreen{sessions: attachedA()}
	assertBody(t, do(t, newTestHandler(f), "POST", "/api/sessions/1.a/select", `{"window":2}`), 204, "")
	if f.selectedID != "1.a" || f.selectedWin != 2 {
		t.Errorf("selected %q/%d, want 1.a/2", f.selectedID, f.selectedWin)
	}
	for _, body := range []string{`{"window":100}`, `{"window":-1}`} {
		assertBody(t, do(t, newTestHandler(f), "POST", "/api/sessions/1.a/select", body), 400, "")
	}
}

func TestScreenFailure(t *testing.T) {
	rec := do(t, newTestHandler(&fakeScreen{err: errScreen}), "GET", "/api/sessions", "")
	if rec.Code != 500 {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"error":`) {
		t.Errorf("body = %s, want an error object", rec.Body.String())
	}
}

func TestRequiresToken(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestHandler(&fakeScreen{}).ServeHTTP(rec, httptest.NewRequest("GET", "/api/sessions", nil))
	if rec.Code != 401 {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestLogNoSecrets(t *testing.T) {
	var buf bytes.Buffer
	h := New(Options{Screen: &fakeScreen{}, Token: testToken, Logger: log.New(&buf, "", 0)})
	do(t, h, "GET", "/api/sessions?x=1", "")
	logged := buf.String()
	if !strings.Contains(logged, "GET /api/sessions 200") {
		t.Errorf("log = %q, want a request line", logged)
	}
	if strings.Contains(logged, testToken) || strings.Contains(logged, "x=1") {
		t.Errorf("log leaks token or query: %q", logged)
	}
}
