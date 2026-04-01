package restore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lovitus/rustdesk-server-friendly/internal/backup"
)

func TestRunImportZipToLinuxDir(t *testing.T) {
	tmpRoot := t.TempDir()
	t.Setenv("RUSTDESK_FRIENDLY_SKIP_DOWNLOAD", "1")
	t.Setenv("RUSTDESK_FRIENDLY_SKIP_SYSTEMCTL", "1")
	t.Setenv("RUSTDESK_FRIENDLY_SYSTEMD_DIR", filepath.Join(tmpRoot, "systemd"))
	t.Setenv("RUSTDESK_FRIENDLY_LOGROTATE_FILE", filepath.Join(tmpRoot, "logrotate", "rustdesk-server"))
	t.Setenv("RUSTDESK_FRIENDLY_JOURNALD_FILE", filepath.Join(tmpRoot, "journald", "rustdesk.conf"))
	t.Setenv("RUSTDESK_FRIENDLY_INSTALL_DIR", filepath.Join(tmpRoot, "install"))
	tmp := t.TempDir()
	source := filepath.Join(tmp, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(source, "id_ed25519"), "priv")
	mustWrite(t, filepath.Join(source, "id_ed25519.pub"), "pub")
	mustWrite(t, filepath.Join(source, "db_v2.sqlite3"), "db")

	archive := filepath.Join(tmp, "backup.zip")
	if _, err := backup.Run(backup.Options{SourceOS: "windows", SourceDataDir: source, Output: archive}); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(tmp, "target")
	res, err := Run(Options{TargetOS: "linux", Archive: archive, TargetDataDir: target, Force: true, TripleConfirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.RestoredFiles) != 5 {
		t.Fatalf("expected 5 restored files, got %d", len(res.RestoredFiles))
	}
	if _, err := os.Stat(filepath.Join(target, "id_ed25519")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, ".rustdesk-friendly-verification-report.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, ".rustdesk-friendly-bundle", "runtime", "snapshot.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, ".rustdesk-friendly-bundle", "policy", "snapshot.json")); err != nil {
		t.Fatal(err)
	}
	md, err := os.ReadFile(filepath.Join(target, "rustdesk-friendly-verification-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(md), "Verification Level") {
		t.Fatal("expected verification summary in verification report")
	}
}

func TestRunLiveVerifyWritesStateAndConfirmation(t *testing.T) {
	tmpRoot := t.TempDir()
	t.Setenv("RUSTDESK_FRIENDLY_SKIP_DOWNLOAD", "1")
	t.Setenv("RUSTDESK_FRIENDLY_SKIP_SYSTEMCTL", "1")
	t.Setenv("RUSTDESK_FRIENDLY_SYSTEMD_DIR", filepath.Join(tmpRoot, "systemd"))
	t.Setenv("RUSTDESK_FRIENDLY_LOGROTATE_FILE", filepath.Join(tmpRoot, "logrotate", "rustdesk-server"))
	t.Setenv("RUSTDESK_FRIENDLY_JOURNALD_FILE", filepath.Join(tmpRoot, "journald", "rustdesk.conf"))
	t.Setenv("RUSTDESK_FRIENDLY_INSTALL_DIR", filepath.Join(tmpRoot, "install"))
	tmp := t.TempDir()
	source := filepath.Join(tmp, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(source, "id_ed25519"), "priv")
	mustWrite(t, filepath.Join(source, "id_ed25519.pub"), "pub")
	mustWrite(t, filepath.Join(source, "db_v2.sqlite3"), "db")
	archive := filepath.Join(tmp, "backup.tgz")
	if _, err := backup.Run(backup.Options{SourceOS: "linux", SourceDataDir: source, Output: archive}); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(tmp, "target")
	res, err := Run(Options{TargetOS: "linux", Archive: archive, TargetDataDir: target, Force: true, TripleConfirmed: true, LiveVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsolatedValidationDataDir == "" {
		t.Fatal("expected isolated validation dir")
	}
	if _, err := os.Stat(filepath.Join(res.IsolatedValidationDataDir, ".rustdesk-friendly-live-verify.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(res.IsolatedValidationDataDir, ".rustdesk-friendly-verification-report.json")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"rustdesk-friendly-client-validation-windows.md",
		"rustdesk-friendly-client-validation-linux.md",
		"rustdesk-friendly-client-validation-macos.md",
	} {
		if _, err := os.Stat(filepath.Join(res.IsolatedValidationDataDir, name)); err != nil {
			t.Fatal(err)
		}
	}
	md, err := os.ReadFile(filepath.Join(res.IsolatedValidationDataDir, "rustdesk-friendly-verification-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(md), "Manual Validation Record") {
		t.Fatal("expected manual validation record in verification report")
	}
	if err := ConfirmLiveRestoreVerified(archive, res.IsolatedValidationDataDir); err != nil {
		t.Fatal(err)
	}
}

func TestRunRejectsInvalidArchive(t *testing.T) {
	tmp := t.TempDir()
	archive := filepath.Join(tmp, "invalid.zip")
	if err := os.WriteFile(archive, []byte("not-a-valid-archive"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Run(Options{TargetOS: "linux", Archive: archive, TargetDataDir: filepath.Join(tmp, "target")})
	if err == nil {
		t.Fatal("expected error")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
