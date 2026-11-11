// Package api serves screen sessions over HTTP: REST for management, SSE for
// change events and WebSocket for the terminal stream.
package api

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os/exec"
	"time"

	"betterscreen/internal/auth"
	"betterscreen/internal/screen"
)

// Screen is what the API needs from the screen domain; screen.Client
// satisfies it.
type Screen interface {
	ListSessions() ([]screen.Session, error)
	ListWindows(screen.Session) ([]screen.Window, error)
	InspectAll(screen.Session, []screen.Window) (map[int]screen.Detail, error)
	CreateSession(name string) error
	KillSession(screen.Session) error
	SelectWindow(sessionID string, window int) error
	Detach(sessionID string) error
	AttachCommand(screen.Session, screen.Window) *exec.Cmd
	ShareCommand(screen.Session, int) *exec.Cmd
}

// Options configures the handler built by New.
type Options struct {
	Screen         Screen
	Token          string
	AllowedOrigins []string
	PollInterval   time.Duration // SSE poll, 0 → 2s
	KeepAlive      time.Duration // SSE keep-alive, 0 → 30s
	Logger         *log.Logger   // nil → discard
}

type server struct {
	screen    Screen
	origins   map[string]bool
	log       *log.Logger
	hub       *hub
	keepAlive time.Duration
}

// New builds the service handler: logging → CORS → auth → routes.
func New(opts Options) http.Handler {
	s := &server{screen: opts.Screen, origins: map[string]bool{}, log: opts.Logger}
	if s.log == nil {
		s.log = log.New(io.Discard, "", 0)
	}
	poll := opts.PollInterval
	if poll == 0 {
		poll = 2 * time.Second
	}
	s.hub = newHub(opts.Screen.ListSessions, poll)
	s.keepAlive = opts.KeepAlive
	if s.keepAlive == 0 {
		s.keepAlive = 30 * time.Second
	}
	for _, o := range opts.AllowedOrigins {
		s.origins[o] = true
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/sessions", s.listSessions)
	mux.HandleFunc("POST /api/sessions", s.createSession)
	mux.HandleFunc("DELETE /api/sessions/{id}", s.killSession)
	mux.HandleFunc("GET /api/sessions/{id}/windows", s.listWindows)
	mux.HandleFunc("POST /api/sessions/{id}/detach", s.detach)
	mux.HandleFunc("POST /api/sessions/{id}/select", s.selectWindow)
	mux.HandleFunc("GET /api/events", s.events)

	return s.logging(s.cors(auth.Middleware(opts.Token, mux)))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// logging writes one line per request: method, path, status, duration. The
// query string and headers are never logged: they may carry secrets.
func (s *server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		s.log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Microsecond))
	})
}

// statusWriter records the status code. It forwards Flush (SSE) and Hijack
// (WebSocket upgrade) so streaming routes work behind the logger.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.status = http.StatusSwitchingProtocols
	return http.NewResponseController(w.ResponseWriter).Hijack()
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
