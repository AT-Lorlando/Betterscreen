package screen

import (
	"regexp"
	"strconv"
	"strings"
)

// lines like: "\t3378954.Astronix\t(18/06/2026 07:46:57)\t(Detached)"
var sessionLineRe = regexp.MustCompile(`^\s*(\d+)\.(\S+)\s+\(.*?\)\s+\((.*?)\)\s*$`)

// ParseSessions turns the output of `screen -ls` into sessions.
func ParseSessions(raw string) []Session {
	var out []Session
	for _, line := range strings.Split(raw, "\n") {
		m := sessionLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		pid, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		out = append(out, Session{
			PID:   pid,
			Name:  m[2],
			ID:    m[1] + "." + m[2],
			State: parseState(m[3]),
		})
	}
	return out
}

func parseState(s string) SessionState {
	switch {
	case strings.HasPrefix(s, "Attached"):
		return StateAttached
	case strings.HasPrefix(s, "Dead"):
		return StateDead
	default:
		return StateDetached
	}
}
