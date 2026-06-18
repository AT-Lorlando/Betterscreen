package screen

import (
	"os"
	"sort"
	"strconv"
	"strings"
)

const unknown = "—"

// ProcFS abstracts access to /proc for testability.
type ProcFS interface {
	ReadDir(name string) ([]string, error)
	ReadFile(name string) ([]byte, error)
	Readlink(name string) (string, error)
}

type osProcFS struct{}

func (osProcFS) ReadDir(name string) ([]string, error) {
	entries, err := os.ReadDir(name)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names, nil
}
func (osProcFS) ReadFile(name string) ([]byte, error) { return os.ReadFile(name) }
func (osProcFS) Readlink(name string) (string, error) { return os.Readlink(name) }

// OSProcFS returns the real /proc-based implementation.
func OSProcFS() ProcFS { return osProcFS{} }

type proc struct {
	pid       int
	comm      string
	ppid      int
	starttime int
}

// statFields parses /proc/<pid>/stat. comm may contain spaces/parentheses,
// so we isolate the last ")" before splitting the rest.
func statFields(data []byte) (proc, bool) {
	s := string(data)
	open := strings.IndexByte(s, '(')
	close := strings.LastIndexByte(s, ')')
	if open < 0 || close < 0 || close < open {
		return proc{}, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(s[:open]))
	if err != nil {
		return proc{}, false
	}
	comm := s[open+1 : close]
	rest := strings.Fields(s[close+1:])
	// rest[0]=state, rest[1]=ppid, ... rest[19]=starttime (global field 22).
	if len(rest) < 20 {
		return proc{}, false
	}
	ppid, _ := strconv.Atoi(rest[1])
	start, _ := strconv.Atoi(rest[19])
	return proc{pid: pid, comm: comm, ppid: ppid, starttime: start}, true
}

func readProc(fs ProcFS, pid int) (proc, bool) {
	data, err := fs.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return proc{}, false
	}
	return statFields(data)
}

func allProcs(fs ProcFS) []proc {
	names, err := fs.ReadDir("/proc")
	if err != nil {
		return nil
	}
	var out []proc
	for _, n := range names {
		pid, err := strconv.Atoi(n)
		if err != nil {
			continue // ignore non-numeric entries
		}
		if p, ok := readProc(fs, pid); ok {
			out = append(out, p)
		}
	}
	return out
}

// InspectAll best-effort maps each window to a pwd + process.
func InspectAll(fs ProcFS, s Session, windows []Window) map[int]Detail {
	out := make(map[int]Detail, len(windows))
	for _, w := range windows {
		out[w.Num] = Detail{Pwd: unknown, Proc: unknown}
	}

	procs := allProcs(fs)
	var children []proc
	for _, p := range procs {
		if p.ppid == s.PID {
			children = append(children, p)
		}
	}

	// Reliable mapping only if there are as many children as windows.
	if len(children) != len(windows) || len(windows) == 0 {
		return out
	}

	sort.Slice(children, func(i, j int) bool { return children[i].starttime < children[j].starttime })
	sorted := append([]Window(nil), windows...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Num < sorted[j].Num })

	for i, w := range sorted {
		shell := children[i]
		pwd, err := fs.Readlink("/proc/" + strconv.Itoa(shell.pid) + "/cwd")
		if err != nil {
			pwd = unknown
		}
		out[w.Num] = Detail{Pwd: pwd, Proc: foreground(procs, shell)}
	}
	return out
}

// foreground walks down to the most recently started descendant of the shell.
func foreground(procs []proc, shell proc) string {
	cur := shell
	for {
		var kids []proc
		for _, p := range procs {
			if p.ppid == cur.pid {
				kids = append(kids, p)
			}
		}
		if len(kids) == 0 {
			return cur.comm
		}
		sort.Slice(kids, func(i, j int) bool { return kids[i].starttime < kids[j].starttime })
		cur = kids[len(kids)-1]
	}
}
