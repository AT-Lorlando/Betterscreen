// Package term relays a screen client running in a pty over a WebSocket.
package term

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/coder/websocket"
	"github.com/creack/pty"
)

// CloseScreenExited is the close code sent when the screen client exits:
// the session was killed, or another connection took it over.
const CloseScreenExited websocket.StatusCode = 4000

// ReadLimit bounds one incoming frame. The library default (32 KiB) would
// drop the connection on a large paste.
const ReadLimit = 1 << 20

// killGrace is how long the command gets to exit on SIGHUP before a kill.
const killGrace = 2 * time.Second

type control struct {
	Type string `json:"type"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

// Bridge runs cmd in an 80x24 pty and relays it over conn: binary frames carry
// terminal bytes both ways, text frames carry control messages from the
// client. It returns once the connection, both relays and cmd are all done.
func Bridge(ctx context.Context, conn *websocket.Conn, cmd *exec.Cmd) error {
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 80, Rows: 24})
	if err != nil {
		conn.Close(websocket.StatusInternalError, "cannot start screen client")
		return err
	}
	defer f.Close()
	conn.SetReadLimit(ReadLimit)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	outDone := make(chan struct{})
	go func() { // pty -> client
		defer close(outDone)
		buf := make([]byte, 32<<10)
		for {
			n, err := f.Read(buf)
			if n > 0 {
				if conn.Write(ctx, websocket.MessageBinary, buf[:n]) != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	inDone := make(chan struct{})
	go func() { // client -> pty
		defer close(inDone)
		for {
			typ, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			if typ == websocket.MessageText {
				applyControl(f, data)
				continue
			}
			if _, err := f.Write(data); err != nil {
				return
			}
		}
	}()

	exited := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(exited)
	}()

	select {
	case <-exited:
		// Deliver the last output, then tell the client why the stream ends.
		select {
		case <-outDone:
		case <-time.After(time.Second):
		}
		conn.Close(CloseScreenExited, "screen client exited")
	case <-inDone:
		// Hang up like a closed terminal would: the screen display detaches
		// and the session survives. Closing the pty is not enough while a
		// Read is blocked on it, so the signal is sent explicitly.
		_ = cmd.Process.Signal(syscall.SIGHUP)
		select {
		case <-exited:
		case <-time.After(killGrace):
			_ = cmd.Process.Kill()
			<-exited
		}
		conn.Close(websocket.StatusNormalClosure, "")
	}
	cancel()
	f.Close()
	<-inDone
	<-outDone
	return nil
}

// applyControl handles a text frame. Anything but a well-formed resize with
// non-zero dimensions is ignored.
func applyControl(f *os.File, data []byte) {
	var c control
	if json.Unmarshal(data, &c) != nil || c.Type != "resize" || c.Cols == 0 || c.Rows == 0 {
		return
	}
	_ = pty.Setsize(f, &pty.Winsize{Cols: c.Cols, Rows: c.Rows})
}
