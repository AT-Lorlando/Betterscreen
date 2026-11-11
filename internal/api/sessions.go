package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"

	"betterscreen/internal/screen"
)

var (
	// Ids and names reach exec as arguments: these patterns keep anything
	// starting with "-" from being parsed by screen as an option.
	sessionIDRe   = regexp.MustCompile(`^[0-9]+\.[A-Za-z0-9_.-]+$`)
	sessionNameRe = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_-]{0,63}$`)

	errNotFound = errors.New("session not found")
)

const (
	maxWindow   = 99
	maxBodySize = 4 << 10
)

type sessionJSON struct {
	ID    string `json:"id"`
	PID   int    `json:"pid"`
	Name  string `json:"name"`
	State string `json:"state"`
}

type windowJSON struct {
	Num   int    `json:"num"`
	Title string `json:"title"`
	Pwd   string `json:"pwd"`
	Proc  string `json:"proc"`
}

func toSessionJSON(s screen.Session) sessionJSON {
	return sessionJSON{ID: s.ID, PID: s.PID, Name: s.Name, State: s.State.String()}
}

func toSessionsJSON(ss []screen.Session) []sessionJSON {
	out := make([]sessionJSON, 0, len(ss))
	for _, s := range ss {
		out = append(out, toSessionJSON(s))
	}
	return out
}

// lookup validates id and resolves it against a fresh `screen -ls`.
func (s *server) lookup(id string) (screen.Session, error) {
	if !sessionIDRe.MatchString(id) {
		return screen.Session{}, errNotFound
	}
	sessions, err := s.screen.ListSessions()
	if err != nil {
		return screen.Session{}, err
	}
	for _, sess := range sessions {
		if sess.ID == id {
			return sess, nil
		}
	}
	return screen.Session{}, errNotFound
}

// lookupOrFail resolves {id} and writes the error response when it fails.
func (s *server) lookupOrFail(w http.ResponseWriter, r *http.Request) (screen.Session, bool) {
	sess, err := s.lookup(r.PathValue("id"))
	switch {
	case errors.Is(err, errNotFound):
		writeError(w, http.StatusNotFound, err.Error())
		return sess, false
	case err != nil:
		writeError(w, http.StatusInternalServerError, err.Error())
		return sess, false
	}
	return sess, true
}

func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodySize)).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

func (s *server) listSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := s.screen.ListSessions()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toSessionsJSON(sessions))
}

func (s *server) createSession(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if !sessionNameRe.MatchString(body.Name) {
		writeError(w, http.StatusBadRequest, "invalid session name")
		return
	}
	existing, err := s.screen.ListSessions()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, ok := findByName(existing, body.Name); ok {
		writeError(w, http.StatusConflict, "session name already in use")
		return
	}
	if err := s.screen.CreateSession(body.Name); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sessions, err := s.screen.ListSessions()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	created, ok := findByName(sessions, body.Name)
	if !ok {
		writeError(w, http.StatusInternalServerError, "session not found after creation")
		return
	}
	writeJSON(w, http.StatusCreated, toSessionJSON(created))
}

func findByName(ss []screen.Session, name string) (screen.Session, bool) {
	for _, s := range ss {
		if s.Name == name {
			return s, true
		}
	}
	return screen.Session{}, false
}

func (s *server) killSession(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.lookupOrFail(w, r)
	if !ok {
		return
	}
	if err := s.screen.KillSession(sess); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) listWindows(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.lookupOrFail(w, r)
	if !ok {
		return
	}
	windows, err := s.screen.ListWindows(sess)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	details, _ := s.screen.InspectAll(sess, windows) // best effort, like the TUI
	out := make([]windowJSON, 0, len(windows))
	for _, win := range windows {
		d := details[win.Num]
		out = append(out, windowJSON{Num: win.Num, Title: win.Title, Pwd: d.Pwd, Proc: d.Proc})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) detach(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.lookupOrFail(w, r)
	if !ok {
		return
	}
	if err := s.screen.Detach(sess.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) selectWindow(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Window *int `json:"window"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if body.Window == nil || *body.Window < 0 || *body.Window > maxWindow {
		writeError(w, http.StatusBadRequest, "window must be an integer in 0..99")
		return
	}
	sess, ok := s.lookupOrFail(w, r)
	if !ok {
		return
	}
	if err := s.screen.SelectWindow(sess.ID, *body.Window); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
