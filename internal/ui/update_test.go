package ui

import (
	"errors"
	"testing"

	"betterscreen/internal/env"
	"betterscreen/internal/screen"

	tea "github.com/charmbracelet/bubbletea"
)

func key(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

func modelWith(api *fakeAPI) Model {
	m := New(api)
	m.sessions = api.sessions
	m.windows = api.windows
	return m
}

func TestNavigateSessionsDown(t *testing.T) {
	api := &fakeAPI{sessions: []screen.Session{{ID: "a"}, {ID: "b"}}}
	m := modelWith(api)
	next, _ := m.Update(key('j'))
	if next.(Model).selSession != 1 {
		t.Errorf("selSession = %d, want 1", next.(Model).selSession)
	}
}

func TestNavigateClampTop(t *testing.T) {
	api := &fakeAPI{sessions: []screen.Session{{ID: "a"}, {ID: "b"}}}
	m := modelWith(api)
	next, _ := m.Update(key('k')) // already at top
	if next.(Model).selSession != 0 {
		t.Errorf("selSession = %d, want 0 (clamp)", next.(Model).selSession)
	}
}

func TestTabSwitchesFocus(t *testing.T) {
	m := modelWith(&fakeAPI{})
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if next.(Model).focus != focusWindows {
		t.Errorf("focus = %v, want focusWindows", next.(Model).focus)
	}
}

func TestDEntersConfirmKill(t *testing.T) {
	api := &fakeAPI{sessions: []screen.Session{{ID: "a"}}}
	m := modelWith(api)
	next, _ := m.Update(key('d'))
	if next.(Model).mode != modeConfirmKill {
		t.Errorf("mode = %v, want modeConfirmKill", next.(Model).mode)
	}
}

func TestConfirmKillYesCallsAPI(t *testing.T) {
	api := &fakeAPI{sessions: []screen.Session{{ID: "victim"}}}
	m := modelWith(api)
	m.mode = modeConfirmKill
	next, _ := m.Update(key('y'))
	if api.killed != "victim" {
		t.Errorf("killed = %q, want victim", api.killed)
	}
	if next.(Model).mode != modeNormal {
		t.Errorf("mode after kill = %v, want modeNormal", next.(Model).mode)
	}
}

func TestNewSessionTypingAndCreate(t *testing.T) {
	api := &fakeAPI{}
	m := modelWith(api)
	m.mode = modeNewSession
	mi, _ := m.Update(key('w'))
	mi2, _ := mi.(Model).Update(key('k'))
	final, _ := mi2.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if api.created != "wk" {
		t.Errorf("created = %q, want wk", api.created)
	}
	if final.(Model).mode != modeNormal {
		t.Errorf("mode = %v, want modeNormal", final.(Model).mode)
	}
}

func TestSessionsMsgUpdatesState(t *testing.T) {
	m := New(&fakeAPI{})
	next, _ := m.Update(sessionsMsg{sessions: []screen.Session{{ID: "x"}}})
	if len(next.(Model).sessions) != 1 {
		t.Errorf("sessions not updated")
	}
}

func TestWindowsMsgErrClearsWindows(t *testing.T) {
	m := modelWith(&fakeAPI{windows: []screen.Window{{Num: 1}}})
	m.windows = []screen.Window{{Num: 1}}
	next, _ := m.Update(windowsMsg{err: errors.New("fail"), windows: []screen.Window{{Num: 1}}})
	if next.(Model).windows != nil {
		t.Error("expected windows cleared on windowsMsg error")
	}
}

func TestNavigateClampBottom(t *testing.T) {
	api := &fakeAPI{sessions: []screen.Session{{ID: "a"}, {ID: "b"}}}
	m := modelWith(api)
	m.selSession = 1
	next, _ := m.Update(key('j')) // already at bottom
	if next.(Model).selSession != 1 {
		t.Errorf("selSession = %d, want 1 (clamp at bottom)", next.(Model).selSession)
	}
}

func TestSessionsMsgSkipsWindowReloadWhenSameSession(t *testing.T) {
	api := &fakeAPI{sessions: []screen.Session{{ID: "a"}}}
	m := modelWith(api)
	m.loadedSessionID = "a" // already loaded — a tick refresh must not reload windows
	_, cmd := m.Update(sessionsMsg{sessions: []screen.Session{{ID: "a"}}})
	if cmd != nil {
		t.Error("expected no window reload cmd when selected session is unchanged")
	}
}

func TestSessionsMsgReloadsWindowsWhenSessionChanges(t *testing.T) {
	api := &fakeAPI{sessions: []screen.Session{{ID: "a"}}}
	m := modelWith(api)
	m.loadedSessionID = "" // nothing loaded yet
	_, cmd := m.Update(sessionsMsg{sessions: []screen.Session{{ID: "a"}}})
	if cmd == nil {
		t.Error("expected a window reload cmd when a session becomes newly selected")
	}
}

func TestEnterLauncherAttaches(t *testing.T) {
	api := &fakeAPI{sessions: []screen.Session{{ID: "A", Name: "A", State: screen.StateDetached}}}
	m := New(api) // launcher mode
	m.sessions = api.sessions
	m.windows = []screen.Window{{Num: 0}}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || api.attachedID != "A" {
		t.Errorf("launcher: expected attach A, got id=%q", api.attachedID)
	}
}

func TestEnterInSessionSelectsCurrentWindow(t *testing.T) {
	api := &fakeAPI{}
	m := New(api, WithContext(env.Context{SessionID: "A", Window: 0, InSession: true}), WithHandoff(&fakeHandoff{}))
	m.sessions = []screen.Session{{ID: "A", Name: "A", State: screen.StateAttached}}
	m.windows = []screen.Window{{Num: 0, Title: "zsh"}, {Num: 2, Title: "vim"}}
	m.selSession = 0
	m.selWindow = 1 // window Num 2
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !api.selectCalled || api.selectedID != "A" || api.selectedWindow != 2 {
		t.Errorf("expected SelectWindow(A,2), got called=%v id=%q win=%d", api.selectCalled, api.selectedID, api.selectedWindow)
	}
	if api.detachedID != "" {
		t.Error("must not detach when staying in the current session")
	}
}

func TestEnterInSessionJumpsToOtherSession(t *testing.T) {
	api := &fakeAPI{}
	ho := &fakeHandoff{}
	m := New(api, WithContext(env.Context{SessionID: "A", Window: 0, InSession: true}), WithHandoff(ho))
	m.sessions = []screen.Session{{ID: "B", Name: "B", State: screen.StateDetached}}
	m.windows = []screen.Window{{Num: 3, Title: "logs"}}
	m.selSession = 0
	m.selWindow = 0
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !ho.written || ho.wroteID != "B" || ho.wroteWin != 3 {
		t.Errorf("expected handoff(B,3), got %+v", ho)
	}
	if api.detachedID != "A" {
		t.Errorf("expected Detach(A), got %q", api.detachedID)
	}
}

func TestEnterInSessionHandoffErrorDoesNotDetach(t *testing.T) {
	api := &fakeAPI{}
	ho := &fakeHandoff{writeErr: errors.New("disk full")}
	m := New(api, WithContext(env.Context{SessionID: "A", InSession: true}), WithHandoff(ho))
	m.sessions = []screen.Session{{ID: "B", State: screen.StateDetached}}
	m.windows = []screen.Window{{Num: 0}}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if api.detachedID != "" {
		t.Error("must NOT detach if the handoff write fails")
	}
	if next.(Model).err == "" {
		t.Error("expected an error message")
	}
}

func TestAttachDoneChainsHandoff(t *testing.T) {
	api := &fakeAPI{}
	ho := &fakeHandoff{readID: "B", readWin: 1, readOK: true}
	m := New(api, WithHandoff(ho))
	_, cmd := m.Update(attachDoneMsg{})
	if cmd == nil {
		t.Fatal("expected a chained attach command")
	}
	if api.attachedID != "B" || api.attachedWin != 1 {
		t.Errorf("expected chained attach to B window 1, got id=%q win=%d", api.attachedID, api.attachedWin)
	}
}

func TestAttachDoneNoHandoffReloads(t *testing.T) {
	api := &fakeAPI{}
	m := New(api, WithHandoff(&fakeHandoff{readOK: false}))
	_, cmd := m.Update(attachDoneMsg{})
	if cmd == nil {
		t.Fatal("expected a refresh command")
	}
	if api.attachedID != "" {
		t.Errorf("must not attach without a handoff, got %q", api.attachedID)
	}
}

func TestEnterDeadSessionBlocked(t *testing.T) {
	api := &fakeAPI{}
	m := New(api) // launcher mode
	m.sessions = []screen.Session{{ID: "D", State: screen.StateDead}}
	m.windows = []screen.Window{{Num: 0}}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("dead session: enter must not trigger anything")
	}
	if api.attachedID != "" {
		t.Errorf("must not attach a dead session, got %q", api.attachedID)
	}
}

func TestAttachDoneErrorSurfaced(t *testing.T) {
	api := &fakeAPI{}
	m := New(api, WithHandoff(&fakeHandoff{readOK: false}))
	next, _ := m.Update(attachDoneMsg{err: errors.New("boom")})
	if next.(Model).err == "" {
		t.Error("an attach error must be shown to the user")
	}
}

func TestEnterInSessionCurrentNoWindowNoop(t *testing.T) {
	api := &fakeAPI{}
	m := New(api, WithContext(env.Context{SessionID: "A", InSession: true}), WithHandoff(&fakeHandoff{}))
	m.sessions = []screen.Session{{ID: "A", State: screen.StateAttached}}
	m.windows = nil // no selectable window
	m.selSession = 0
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if api.selectCalled {
		t.Error("must not SelectWindow without a selected window")
	}
	if cmd != nil {
		t.Error("expected a no-op (no Quit) on the current session without a window")
	}
}

func TestEnterInSessionNilHandoffDoesNotDetach(t *testing.T) {
	api := &fakeAPI{}
	// in-session but WITHOUT WithHandoff -> m.handoff is nil
	m := New(api, WithContext(env.Context{SessionID: "A", InSession: true}))
	m.sessions = []screen.Session{{ID: "B", State: screen.StateDetached}}
	m.windows = []screen.Window{{Num: 0}}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if api.detachedID != "" {
		t.Error("nil handoff: must NOT detach (otherwise session lost with no target)")
	}
	if next.(Model).err == "" {
		t.Error("expected an error message when the handoff is unavailable")
	}
}
