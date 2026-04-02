package acceptance

import "testing"

func TestLinuxServiceLoadStateExists(t *testing.T) {
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
		if got := linuxServiceLoadStateExists(tc.state); got != tc.want {
			t.Fatalf("state %q: want %v got %v", tc.state, tc.want, got)
		}
	}
}

func TestAllowServiceConflictWarning(t *testing.T) {
	cases := []struct {
		name  string
		os    string
		allow bool
		want  bool
	}{
		{name: "linux allowed", os: "linux", allow: true, want: true},
		{name: "linux blocked", os: "linux", allow: false, want: false},
		{name: "windows stays blocking", os: "windows", allow: true, want: false},
		{name: "darwin stays blocking", os: "darwin", allow: true, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := allowServiceConflictWarning(tc.os, tc.allow); got != tc.want {
				t.Fatalf("os=%s allow=%v: want %v got %v", tc.os, tc.allow, tc.want, got)
			}
		})
	}
}
