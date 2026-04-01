package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunCreatesRuntimePlanAndServiceArtifacts(t *testing.T) {
	t.Setenv("RUSTDESK_FRIENDLY_SKIP_DOWNLOAD", "1")
	t.Setenv("RUSTDESK_FRIENDLY_SKIP_SYSTEMCTL", "1")
	unitDir := filepath.Join(t.TempDir(), "systemd")
	t.Setenv("RUSTDESK_FRIENDLY_SYSTEMD_DIR", unitDir)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("RUSTDESK_FRIENDLY_INSTALL_DIR", filepath.Join(tmp, "install"))
	t.Setenv("RUSTDESK_FRIENDLY_DATA_DIR", filepath.Join(tmp, "data"))
	t.Setenv("RUSTDESK_FRIENDLY_LOG_DIR", filepath.Join(tmp, "logs"))

	res, err := Run(Options{TargetOS: "linux", TripleConfirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(res.InstallDir, "rustdesk-friendly-install.plan.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(unitDir, "rustdesk-hbbs.service")); err != nil {
		t.Fatal(err)
	}
}
