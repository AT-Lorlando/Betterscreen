package screen

import (
	"reflect"
	"testing"
)

func TestAttachCommand(t *testing.T) {
	s := Session{ID: "3378954.Astronix"}
	cmd := AttachCommand(s, Window{Num: 2})
	want := []string{"screen", "-r", "3378954.Astronix", "-p", "2"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Errorf("args = %v, want %v", cmd.Args, want)
	}
}

func TestKillArgs(t *testing.T) {
	got := killArgs(Session{ID: "3378954.Astronix"})
	want := []string{"-S", "3378954.Astronix", "-X", "quit"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCreateArgs(t *testing.T) {
	got := createArgs("work")
	want := []string{"-dmS", "work"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSelectArgs(t *testing.T) {
	got := selectArgs("3378954.Astronix", 2)
	want := []string{"-S", "3378954.Astronix", "-X", "select", "2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDetachArgs(t *testing.T) {
	got := detachArgs("3378954.Astronix")
	want := []string{"-S", "3378954.Astronix", "-X", "detach"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
