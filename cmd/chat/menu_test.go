package main

import (
	"testing"

	pb "github.com/NurPech/hannah-proto-go/v4"
)

func TestParseChoice(t *testing.T) {
	cases := []struct {
		in     string
		wantN  int
		wantOK bool
	}{
		{"0", 0, true},
		{"3", 3, true},
		{"  2  ", 2, true},
		{"", 0, false},
		{"-1", 0, false},
		{"abc", 0, false},
		{"1.5", 0, false},
	}
	for _, c := range cases {
		n, ok := parseChoice(c.in)
		if ok != c.wantOK || (ok && n != c.wantN) {
			t.Errorf("parseChoice(%q) = (%d, %v), want (%d, %v)", c.in, n, ok, c.wantN, c.wantOK)
		}
	}
}

func TestWritableActionsSortedAndFiltered(t *testing.T) {
	dev := &pb.DeviceInfo{
		States:        []string{"on", "level", "color"},
		StateWritable: map[string]bool{"on": true, "level": true, "color": false},
	}
	got := writableActions(dev)
	want := []string{"level", "on"}
	if len(got) != len(want) {
		t.Fatalf("writableActions = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("writableActions = %v, want %v", got, want)
		}
	}
}

func TestActionLabelBooleanToggle(t *testing.T) {
	dev := &pb.DeviceInfo{
		StateTypes: map[string]pb.StateType{"on": pb.StateType_BOOLEAN},
		Current:    map[string]string{"on": "True"},
	}
	if got := actionLabel(dev, "on"); got != "Turn off" {
		t.Errorf("actionLabel(on, True) = %q, want %q", got, "Turn off")
	}

	dev.Current["on"] = "False"
	if got := actionLabel(dev, "on"); got != "Turn on" {
		t.Errorf("actionLabel(on, False) = %q, want %q", got, "Turn on")
	}
}

func TestActionLabelNonBoolean(t *testing.T) {
	dev := &pb.DeviceInfo{
		StateTypes: map[string]pb.StateType{"level": pb.StateType_NUMERIC},
		Current:    map[string]string{"level": "75"},
	}
	want := "Set level (current: 75)"
	if got := actionLabel(dev, "level"); got != want {
		t.Errorf("actionLabel(level) = %q, want %q", got, want)
	}

	dev2 := &pb.DeviceInfo{
		StateTypes: map[string]pb.StateType{"color": pb.StateType_COLOR},
		Current:    map[string]string{},
	}
	if got := actionLabel(dev2, "color"); got != "Set color" {
		t.Errorf("actionLabel(color, no current) = %q, want %q", got, "Set color")
	}
}

func TestStatusDot(t *testing.T) {
	cases := []struct {
		current string
		want    string
	}{
		{"True", "🟢"},
		{"False", "🔴"},
		{"", "⚫"},
		{"true", "⚫"}, // server serialises Python bools as "True"/"False", not lowercase
	}
	for _, c := range cases {
		dev := &pb.DeviceInfo{Current: map[string]string{"on": c.current}}
		if got := statusDot(dev); got != c.want {
			t.Errorf("statusDot(on=%q) = %q, want %q", c.current, got, c.want)
		}
	}
}

func TestCategoryIconFallback(t *testing.T) {
	if got := categoryIcon("Licht"); got != "💡" {
		t.Errorf("categoryIcon(Licht) = %q, want 💡", got)
	}
	if got := categoryIcon("Irgendwas"); got != "⚙️" {
		t.Errorf("categoryIcon(Irgendwas) = %q, want ⚙️", got)
	}
}
