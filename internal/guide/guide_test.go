package guide

import (
	"strings"
	"testing"
)

func TestLinuxAllIncludesSections(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Target = "linux"
	cfg.Topic = "all"
	cfg.Host = "example.com"

	out, err := Render(cfg)
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, out, "Linux CLI Deploy (Binary, Idempotent)")
	mustContain(t, out, "Linux Service Install (systemd, Idempotent)")
	mustContain(t, out, "Linux Log Limits (Idempotent)")
	mustContain(t, out, "example.com:21117")
	mustContain(t, out, "DOWNLOAD_TOOL")
	mustContain(t, out, "wget")
	mustContain(t, out, "RUSTDESK_ZIP_SHA256")
}

func TestWindowsAllIncludesSections(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Target = "windows"
	cfg.Topic = "all"

	out, err := Render(cfg)
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, out, "Windows CLI Deploy (PowerShell, Idempotent)")
	mustContain(t, out, "Windows Service Install (PM2, Idempotent)")
	mustContain(t, out, "Windows Log Limits (Idempotent)")
	mustContain(t, out, "winget install GitHub.cli")
	mustContain(t, out, "rustdesk-server-windows-x86_64-unsigned.zip")
}

func TestMigrationPairs(t *testing.T) {
	pairs := [][3]string{
		{"windows", "linux", "Migration: Windows -> Linux"},
		{"linux", "windows", "Migration: Linux -> Windows"},
		{"linux", "linux", "Migration: Linux -> Linux"},
		{"windows", "windows", "Migration: Windows -> Windows"},
	}

	for _, p := range pairs {
		cfg := DefaultConfig()
		cfg.Target = "cross"
		cfg.Topic = "migrate"
		cfg.MigrationSourceOS = p[0]
		cfg.MigrationTargetOS = p[1]
		out, err := Render(cfg)
		if err != nil {
			t.Fatalf("%v: %v", p, err)
		}
		mustContain(t, out, p[2])
		mustContain(t, out, "id_ed25519")
		mustContain(t, out, "RUSTDESK_SOURCE_DATA_DIR")
		mustContain(t, out, "RUSTDESK_TARGET_DATA_DIR")
	}
}

func TestInvalidTarget(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Target = "macos"
	if _, err := Render(cfg); err == nil {
		t.Fatal("expected error")
	}
}

func mustContain(t *testing.T, text, want string) {
	t.Helper()
	if !strings.Contains(text, want) {
		t.Fatalf("expected output to contain %q", want)
	}
}
