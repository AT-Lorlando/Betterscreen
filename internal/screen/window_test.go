package screen

import "testing"

func TestParseWindows(t *testing.T) {
	// format vérifié: "<num>[flags] <titre>", séparé par espaces, sans newline.
	got := ParseWindows("0 zsh  1*$ vim  2 logs")
	if len(got) != 3 {
		t.Fatalf("want 3 windows, got %d: %+v", len(got), got)
	}
	want := []Window{{0, "zsh"}, {1, "vim"}, {2, "logs"}}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("window %d = %+v, want %+v", i, got[i], w)
		}
	}
}

func TestParseWindowsSingle(t *testing.T) {
	got := ParseWindows("0 claude")
	if len(got) != 1 || got[0] != (Window{0, "claude"}) {
		t.Errorf("got %+v", got)
	}
}

func TestParseWindowsEmpty(t *testing.T) {
	if got := ParseWindows(""); len(got) != 0 {
		t.Errorf("want 0, got %+v", got)
	}
}
