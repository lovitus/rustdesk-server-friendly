package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyLinuxWritesUnitsWhenSystemctlSkipped(t *testing.T) {
	unitDir := filepath.Join(t.TempDir(), "systemd")
	t.Setenv("RUSTDESK_FRIENDLY_SYSTEMD_DIR", unitDir)
	t.Setenv("RUSTDESK_FRIENDLY_SKIP_SYSTEMCTL", "1")
	tmp := t.TempDir()
	logDir := filepath.Join(tmp, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := Apply(Config{
		OS:          "linux",
		ServiceName: "rustdesk",
		DataDir:     filepath.Join(tmp, "data"),
		InstallDir:  filepath.Join(tmp, "bin"),
		LogDir:      logDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.UnitPaths) != 2 {
		t.Fatalf("expected 2 unit paths, got %d", len(res.UnitPaths))
	}
	for _, path := range res.UnitPaths {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDetectWindowsManagerEnvOverride(t *testing.T) {
	t.Setenv("RUSTDESK_FRIENDLY_WINDOWS_SERVICE_MANAGER", "pm2")
	if got := detectWindowsManager(); got != "pm2" {
		t.Fatalf("expected pm2, got %s", got)
	}
}

func TestApplyLinuxVerifyModeUsesIsolatedPorts(t *testing.T) {
	unitDir := filepath.Join(t.TempDir(), "systemd")
	t.Setenv("RUSTDESK_FRIENDLY_SYSTEMD_DIR", unitDir)
	t.Setenv("RUSTDESK_FRIENDLY_SKIP_SYSTEMCTL", "1")
	tmp := t.TempDir()
	logDir := filepath.Join(tmp, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := Apply(Config{
		OS:          "linux",
		ServiceName: "rustdesk",
		DataDir:     filepath.Join(tmp, "data"),
		InstallDir:  filepath.Join(tmp, "bin"),
		LogDir:      logDir,
		RelayHost:   "127.0.0.1",
		HBBSPort:    22116,
		HBBRPort:    22117,
		VerifyMode:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.HBBSPort != 22116 || res.HBBRPort != 22117 {
		t.Fatalf("unexpected ports: %+v", res)
	}
	content, err := os.ReadFile(filepath.Join(unitDir, "rustdesk-hbbs-verify.service"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "-p 22116 -r 127.0.0.1:22117") {
		t.Fatalf("expected isolated hbbs ports in unit, got %s", string(content))
	}
}
