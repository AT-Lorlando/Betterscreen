package screen

// SessionState represents the attachment state of a screen session.
type SessionState int

const (
	StateDetached SessionState = iota
	StateAttached
	StateDead
)

func (s SessionState) String() string {
	switch s {
	case StateAttached:
		return "attached"
	case StateDead:
		return "dead"
	default:
		return "detached"
	}
}

// Session is a screen session as listed by `screen -ls`.
type Session struct {
	PID   int
	Name  string
	ID    string // "<pid>.<name>", identifier passed to `screen -S`
	State SessionState
}

// Window is a window inside a screen session.
type Window struct {
	Num   int
	Title string
}

// Detail is best-effort info about a window (pwd + foreground process).
type Detail struct {
	Pwd  string
	Proc string
}
