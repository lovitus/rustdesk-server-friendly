package runtimeinfo

import "testing"

func TestLinuxLoadStateExists(t *testing.T) {
	cases := []struct {
		state string
		want  bool
	}{
		{"loaded", true},
		{" masked ", true},
		{"not-found", false},
		{"error", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := linuxLoadStateExists(tc.state); got != tc.want {
			t.Fatalf("state %q: want %v got %v", tc.state, tc.want, got)
		}
	}
}
