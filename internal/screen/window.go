package screen

import (
	"regexp"
	"strconv"
)

// Capture: a number, optional non-space flags, then the title.
// E.g. "0 zsh", "1*$ vim". Multi-word titles are not supported (default
// screen titles = a single token).
var windowRe = regexp.MustCompile(`(\d+)\S*\s+(\S+)`)

// ParseWindows turns the output of `screen -Q windows` into windows.
func ParseWindows(raw string) []Window {
	var out []Window
	for _, m := range windowRe.FindAllStringSubmatch(raw, -1) {
		num, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		out = append(out, Window{Num: num, Title: m[2]})
	}
	return out
}
