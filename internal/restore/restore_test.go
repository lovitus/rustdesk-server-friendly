package restore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lovitus/rustdesk-server-friendly/internal/backup"
)

func TestRunImportZipToLinuxDir(t *testing.T) {
	tmpRoot := t.TempDir()
	t.Setenv("RUSTDESK_FRIENDLY_SKIP_DOWNLOAD", "1")
	t.Setenv("RUSTDESK_FRIENDLY_SKIP_SYSTEMCTL", "1")
	t.Setenv("RUSTDESK_FRIENDLY_SYSTEMD_DIR", filepath.Join(tmpRoot, "systemd"))
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
	if len(res.RestoredFiles) != 3 {
		t.Fatalf("expected 3 restored files, got %d", len(res.RestoredFiles))
	}
	if _, err := os.Stat(filepath.Join(target, "id_ed25519")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, ".rustdesk-friendly-verification-report.json")); err != nil {
		t.Fatal(err)
	}
}

func TestRunLiveVerifyWritesStateAndConfirmation(t *testing.T) {
	tmpRoot := t.TempDir()
	t.Setenv("RUSTDESK_FRIENDLY_SKIP_DOWNLOAD", "1")
	t.Setenv("RUSTDESK_FRIENDLY_SKIP_SYSTEMCTL", "1")
	t.Setenv("RUSTDESK_FRIENDLY_SYSTEMD_DIR", filepath.Join(tmpRoot, "systemd"))
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
	if _, err := Run(Options{TargetOS: "linux", Archive: archive, TargetDataDir: target, ValidateOnly: true, UserConfirmedLive: true, TripleConfirmed: true}); err != nil {
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
