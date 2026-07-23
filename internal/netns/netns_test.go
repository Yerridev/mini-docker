package netns

import (
	"testing"
)

func TestParseMode(t *testing.T) {
	cases := []struct {
		input string
		want  Mode
	}{
		{"", ModeLoopback},
		{"loopback", ModeLoopback},
		{"LOOPBACK", ModeLoopback},
		{"none", ModeNone},
		{"None", ModeNone},
		{"veth", ModeVeth},
		{"VETH", ModeVeth},
	}
	for _, tc := range cases {
		got, err := ParseMode(tc.input)
		if err != nil {
			t.Fatalf("ParseMode(%q) error inesperado: %v", tc.input, err)
		}
		if got != tc.want {
			t.Errorf("ParseMode(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestParseModeInvalid(t *testing.T) {
	_, err := ParseMode("bridge")
	if err == nil {
		t.Fatal("ParseMode(bridge) debería fallar")
	}
}

func TestModeString(t *testing.T) {
	cases := []struct {
		mode Mode
		want string
	}{
		{ModeNone, "none"},
		{ModeLoopback, "loopback"},
		{ModeVeth, "veth"},
		{Mode(99), "Mode(99)"},
	}
	for _, tc := range cases {
		if got := tc.mode.String(); got != tc.want {
			t.Errorf("Mode(%d).String() = %q, want %q", tc.mode, got, tc.want)
		}
	}
}
