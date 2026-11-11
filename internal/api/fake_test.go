package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"betterscreen/internal/screen"
)

const testToken = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFG"

// fakeScreen implements Screen for tests.
type fakeScreen struct {
	mu       sync.Mutex
	sessions []screen.Session
	windows  []screen.Window
	details  map[int]screen.Detail
	err      error // when set, every method fails

	listCalls   int
	created     string
	killed      string
	selectedID  string
	selectedWin int
	detached    string

	shareSession  string
	shareWindow   int
	attachSession string
	attachWindow  int
	commandsBuilt int
}

var errScreen = errors.New("screen exploded")

func (f *fakeScreen) setSessions(s []screen.Session) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessions = s
}

func (f *fakeScreen) ListSessions() ([]screen.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listCalls++
	if f.err != nil {
		return nil, f.err
	}
	return append([]screen.Session(nil), f.sessions...), nil
}

func (f *fakeScreen) ListWindows(screen.Session) ([]screen.Window, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.windows, nil
}

func (f *fakeScreen) InspectAll(screen.Session, []screen.Window) (map[int]screen.Detail, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.details, nil
}

func (f *fakeScreen) CreateSession(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.created = name
	pid := 100 + len(f.sessions)
	f.sessions = append(f.sessions, screen.Session{
		PID: pid, Name: name, ID: fmt.Sprintf("%d.%s", pid, name), State: screen.StateDetached,
	})
	return nil
}

func (f *fakeScreen) KillSession(s screen.Session) error {
	if f.err != nil {
		return f.err
	}
	f.killed = s.ID
	return nil
}

func (f *fakeScreen) SelectWindow(id string, n int) error {
	if f.err != nil {
		return f.err
	}
	f.selectedID, f.selectedWin = id, n
	return nil
}

func (f *fakeScreen) Detach(id string) error {
	if f.err != nil {
		return f.err
	}
	f.detached = id
	return nil
}

func (f *fakeScreen) AttachCommand(s screen.Session, w screen.Window) *exec.Cmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.attachSession, f.attachWindow = s.ID, w.Num
	f.commandsBuilt++
	return exec.Command("cat")
}

func (f *fakeScreen) ShareCommand(s screen.Session, window int) *exec.Cmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.shareSession, f.shareWindow = s.ID, window
	f.commandsBuilt++
	return exec.Command("cat")
}

// do sends an authenticated request to h and returns the recorder.
func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+testToken)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func newTestHandler(f *fakeScreen) http.Handler {
	return New(Options{Screen: f, Token: testToken, AllowedOrigins: []string{"https://front.lan"}})
}
