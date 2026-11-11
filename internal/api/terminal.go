package api

import (
	"net/http"
	"os/exec"
	"strconv"
	"time"

	"github.com/coder/websocket"

	"betterscreen/internal/auth"
	"betterscreen/internal/screen"
	"betterscreen/internal/term"
)

// terminal upgrades to a WebSocket bridged to a screen client: `share` joins
// the session alongside its other displays, `take` detaches them first.
// Validation happens before the upgrade so errors stay plain HTTP statuses.
func (s *server) terminal(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	mode := q.Get("mode")
	if mode == "" {
		mode = "share"
	}
	if mode != "share" && mode != "take" {
		writeError(w, http.StatusBadRequest, "mode must be share or take")
		return
	}
	window := -1 // absent: keep the session's current window
	if v := q.Get("window"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > maxWindow {
			writeError(w, http.StatusBadRequest, "window must be an integer in 0..99")
			return
		}
		window = n
	}
	sess, ok := s.lookupOrFail(w, r)
	if !ok {
		return
	}

	var cmd *exec.Cmd
	if mode == "take" {
		cmd = s.screen.AttachCommand(sess, screen.Window{Num: window})
	} else {
		cmd = s.screen.ShareCommand(sess, window)
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		Subprotocols:   []string{auth.Subprotocol},
		OriginPatterns: s.originHosts,
	})
	if err != nil {
		return // Accept already wrote the error response
	}

	start := time.Now()
	s.log.Printf("ws open %s %s", sess.ID, mode)
	_ = term.Bridge(r.Context(), conn, cmd)
	s.log.Printf("ws close %s %s %s", sess.ID, mode, time.Since(start).Round(time.Millisecond))
}
