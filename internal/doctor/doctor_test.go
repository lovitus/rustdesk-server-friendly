package doctor

import (
	"testing"

	"github.com/lovitus/rustdesk-server-friendly/internal/runtimeinfo"
)

func TestShouldSkipManagedServiceRepair(t *testing.T) {
	cases := []struct {
		name string
		rt   runtimeinfo.Runtime
		want bool
	}{
		{name: "linux existing service", rt: runtimeinfo.Runtime{OS: "linux", ExistingService: true}, want: true},
		{name: "linux without service", rt: runtimeinfo.Runtime{OS: "linux", ExistingService: false}, want: false},
		{name: "windows existing service", rt: runtimeinfo.Runtime{OS: "windows", ExistingService: true}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldSkipManagedServiceRepair(tc.rt); got != tc.want {
				t.Fatalf("want %v got %v for %+v", tc.want, got, tc.rt)
			}
		})
	}
}

func TestDefaultManagedServiceValidationTargets(t *testing.T) {
	names, ports := defaultManagedServiceValidationTargets()
	if len(names) != 2 || names[0] != "rustdesk-hbbs" || names[1] != "rustdesk-hbbr" {
		t.Fatalf("unexpected service names: %+v", names)
	}
	if len(ports) != 2 || ports[0] != 21116 || ports[1] != 21117 {
		t.Fatalf("unexpected ports: %+v", ports)
	}
}
