package logpolicy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyLinuxWritesPoliciesWhenActivationSkipped(t *testing.T) {
	t.Setenv("RUSTDESK_FRIENDLY_SKIP_SYSTEMCTL", "1")
	root := t.TempDir()
	logrotateFile := filepath.Join(root, "logrotate", "rustdesk-server")
	journaldFile := filepath.Join(root, "journald", "rustdesk.conf")
	t.Setenv("RUSTDESK_FRIENDLY_LOGROTATE_FILE", logrotateFile)
	t.Setenv("RUSTDESK_FRIENDLY_JOURNALD_FILE", journaldFile)
	logDir := filepath.Join(root, "logs")
	res, err := Apply(Config{OS: "linux", LogDir: logDir})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.ArtifactPaths) != 2 {
		t.Fatalf("expected 2 policy artifacts, got %d", len(res.ArtifactPaths))
	}
	for _, path := range []string{logrotateFile, journaldFile} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
}

func TestApplyWindowsWritesPolicyArtifact(t *testing.T) {
	root := t.TempDir()
	logDir := filepath.Join(root, "logs")
	res, err := Apply(Config{OS: "windows", ServiceManager: "sc", LogDir: logDir, ServiceNames: []string{"rustdesk-hbbs", "rustdesk-hbbr"}})
	if err != nil {
		t.Fatal(err)
	}
	policy := filepath.Join(logDir, "rustdesk-friendly-log-policy.json")
	if _, err := os.Stat(policy); err != nil {
		t.Fatal(err)
	}
	if len(res.ArtifactPaths) == 0 {
		t.Fatal("expected policy artifact path")
	}
}
