package ui

import (
	"os/exec"
	"testing"

	"betterscreen/internal/env"
	"betterscreen/internal/screen"
)

// fakeAPI implements ScreenAPI for tests.
type fakeAPI struct {
	sessions []screen.Session
	windows  []screen.Window
	details  map[int]screen.Detail
	created  string
	killed   string

	selectedID     string
	selectedWindow int
	selectCalled   bool
	detachedID     string
	attachedID     string
	attachedWin    int
}

func (f *fakeAPI) ListSessions() ([]screen.Session, error) { return f.sessions, nil }
func (f *fakeAPI) ListWindows(screen.Session) ([]screen.Window, error) {
	return f.windows, nil
}
func (f *fakeAPI) InspectAll(screen.Session, []screen.Window) (map[int]screen.Detail, error) {
	return f.details, nil
}
func (f *fakeAPI) CreateSession(name string) error    { f.created = name; return nil }
func (f *fakeAPI) KillSession(s screen.Session) error { f.killed = s.ID; return nil }
func (f *fakeAPI) AttachCommand(s screen.Session, w screen.Window) *exec.Cmd {
	f.attachedID = s.ID
	f.attachedWin = w.Num
	return exec.Command("true")
}
func (f *fakeAPI) SelectWindow(id string, n int) error {
	f.selectedID = id
	f.selectedWindow = n
	f.selectCalled = true
	return nil
}
func (f *fakeAPI) Detach(id string) error { f.detachedID = id; return nil }

// fakeHandoff implements Handoff for tests.
type fakeHandoff struct {
	wroteID  string
	wroteWin int
	written  bool
	writeErr error
	readID   string
	readWin  int
	readOK   bool
}

func (h *fakeHandoff) Write(id string, win int) error {
	h.wroteID, h.wroteWin, h.written = id, win, true
	return h.writeErr
}
func (h *fakeHandoff) ReadAndClear() (string, int, bool) { return h.readID, h.readWin, h.readOK }

func TestNewModelDefaults(t *testing.T) {
	m := New(&fakeAPI{})
	if m.focus != focusSessions {
		t.Errorf("initial focus = %v, want focusSessions", m.focus)
	}
	if m.mode != modeNormal {
		t.Errorf("initial mode = %v, want modeNormal", m.mode)
	}
}

func TestInitReturnsCommand(t *testing.T) {
	m := New(&fakeAPI{})
	if m.Init() == nil {
		t.Error("Init() must return a command (initial load)")
	}
}

func TestNewDefaultsLauncher(t *testing.T) {
	m := New(&fakeAPI{})
	if m.inSession {
		t.Error("default = launcher mode (inSession=false)")
	}
	if m.currentWindow != -1 {
		t.Errorf("default currentWindow = %d, want -1", m.currentWindow)
	}
}

func TestWithContextSetsInSession(t *testing.T) {
	m := New(&fakeAPI{}, WithContext(env.Context{SessionID: "9.work", Window: 2, InSession: true}))
	if !m.inSession || m.currentSessionID != "9.work" || m.currentWindow != 2 {
		t.Errorf("context not applied: inSession=%v id=%q win=%d", m.inSession, m.currentSessionID, m.currentWindow)
	}
}

func TestWithHandoffInjects(t *testing.T) {
	ho := &fakeHandoff{}
	m := New(&fakeAPI{}, WithHandoff(ho))
	if m.handoff == nil {
		t.Error("handoff not injected")
	}
}

func TestFilterWindowRemovesEphemeral(t *testing.T) {
	ws := []screen.Window{{Num: 0}, {Num: 1}, {Num: 2}}
	got := filterWindow(ws, 1)
	if len(got) != 2 || got[0].Num != 0 || got[1].Num != 2 {
		t.Errorf("got %+v", got)
	}
}

func TestLoadWindowsHidesEphemeralInCurrentSession(t *testing.T) {
	api := &fakeAPI{windows: []screen.Window{{Num: 0, Title: "zsh"}, {Num: 1, Title: "betterscreen"}}}
	m := New(api, WithContext(env.Context{SessionID: "A", Window: 1, InSession: true}))
	msg := m.loadWindows(screen.Session{ID: "A"})().(windowsMsg)
	if len(msg.windows) != 1 || msg.windows[0].Num != 0 {
		t.Errorf("expected ephemeral window 1 to be filtered, got %+v", msg.windows)
	}
}

func TestLoadWindowsKeepsAllForOtherSession(t *testing.T) {
	api := &fakeAPI{windows: []screen.Window{{Num: 0}, {Num: 1}}}
	m := New(api, WithContext(env.Context{SessionID: "A", Window: 1, InSession: true}))
	msg := m.loadWindows(screen.Session{ID: "B"})().(windowsMsg) // other session
	if len(msg.windows) != 2 {
		t.Errorf("must not filter anything for another session, got %+v", msg.windows)
	}
}
