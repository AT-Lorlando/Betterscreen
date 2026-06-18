package screen

// SessionState représente l'état d'attachement d'une session screen.
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

// Session est une session screen telle que listée par `screen -ls`.
type Session struct {
	PID   int
	Name  string
	ID    string // "<pid>.<nom>", identifiant passé à `screen -S`
	State SessionState
}

// Window est une fenêtre dans une session screen.
type Window struct {
	Num   int
	Title string
}

// Detail est l'info best-effort sur une fenêtre (pwd + process avant-plan).
type Detail struct {
	Pwd  string
	Proc string
}
