package restore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lovitus/rustdesk-server-friendly/internal/backup"
	"github.com/lovitus/rustdesk-server-friendly/internal/bundle"
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
	reportJSON, err := os.ReadFile(filepath.Join(res.IsolatedValidationDataDir, ".rustdesk-friendly-verification-report.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(reportJSON), `"verification_level": "live_restore_verified"`) ||
		!strings.Contains(string(reportJSON), `"user_confirmed_live_restore": true`) {
		t.Fatalf("expected refreshed verification report json, got %s", string(reportJSON))
	}
	reportMD, err := os.ReadFile(filepath.Join(res.IsolatedValidationDataDir, "rustdesk-friendly-verification-report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(reportMD), "Verification Level: `live_restore_verified`") ||
		!strings.Contains(string(reportMD), "User Confirmed Live Restore: `true`") {
		t.Fatalf("expected refreshed verification report markdown, got %s", string(reportMD))
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

func TestRunRejectsDirectUserConfirmedLive(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(source, "id_ed25519"), "priv")
	mustWrite(t, filepath.Join(source, "id_ed25519.pub"), "pub")
	archive := filepath.Join(tmp, "backup.tgz")
	if _, err := backup.Run(backup.Options{SourceOS: "linux", SourceDataDir: source, Output: archive}); err != nil {
		t.Fatal(err)
	}
	_, err := Run(Options{
		TargetOS:          "linux",
		Archive:           archive,
		TargetDataDir:     filepath.Join(tmp, "target"),
		UserConfirmedLive: true,
	})
	if err == nil || !strings.Contains(err.Error(), "confirm-live-verify") {
		t.Fatalf("expected confirm-live-verify guidance error, got %v", err)
	}
}

func TestConfirmLiveRestoreRequiresVerificationState(t *testing.T) {
	tmp := t.TempDir()
	archive := filepath.Join(tmp, "backup.tgz")
	source := filepath.Join(tmp, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(source, "id_ed25519"), "priv")
	mustWrite(t, filepath.Join(source, "id_ed25519.pub"), "pub")
	if _, err := backup.Run(backup.Options{SourceOS: "linux", SourceDataDir: source, Output: archive}); err != nil {
		t.Fatal(err)
	}
	err := ConfirmLiveRestoreVerified(archive, filepath.Join(tmp, "missing-state"))
	if err == nil || !strings.Contains(err.Error(), "verification state not found") {
		t.Fatalf("expected missing verification state error, got %v", err)
	}
}

func TestConfirmLiveRestoreRequiresNonEmptyVerificationDir(t *testing.T) {
	tmp := t.TempDir()
	archive := filepath.Join(tmp, "backup.tgz")
	source := filepath.Join(tmp, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(source, "id_ed25519"), "priv")
	mustWrite(t, filepath.Join(source, "id_ed25519.pub"), "pub")
	if _, err := backup.Run(backup.Options{SourceOS: "linux", SourceDataDir: source, Output: archive}); err != nil {
		t.Fatal(err)
	}
	err := ConfirmLiveRestoreVerified(archive, "   ")
	if err == nil || !strings.Contains(err.Error(), "verification directory is required") {
		t.Fatalf("expected empty verification dir error, got %v", err)
	}
}

func TestConfirmLiveRestorePreservesManualMarkdownNotes(t *testing.T) {
	tmpRoot := t.TempDir()
	t.Setenv("RUSTDESK_FRIENDLY_SKIP_DOWNLOAD", "1")
	t.Setenv("RUSTDESK_FRIENDLY_SKIP_SYSTEMCTL", "1")
	t.Setenv("RUSTDESK_FRIENDLY_SYSTEMD_DIR", filepath.Join(tmpRoot, "systemd"))
	t.Setenv("RUSTDESK_FRIENDLY_LOGROTATE_FILE", filepath.Join(tmpRoot, "logrotate", "rustdesk-server"))
	t.Setenv("RUSTDESK_FRIENDLY_JOURNALD_FILE", filepath.Join(tmpRoot, "journald", "rustdesk.conf"))
	t.Setenv("RUSTDESK_FRIENDLY_INSTALL_DIR", filepath.Join(tmpRoot, "install"))
	tmp := t.TempDir()
	archive := filepath.Join(tmp, "backup.tgz")
	source := filepath.Join(tmp, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(source, "id_ed25519"), "priv")
	mustWrite(t, filepath.Join(source, "id_ed25519.pub"), "pub")
	mustWrite(t, filepath.Join(source, "db_v2.sqlite3"), "db")
	if _, err := backup.Run(backup.Options{SourceOS: "linux", SourceDataDir: source, Output: archive}); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(tmp, "target")
	res, err := Run(Options{TargetOS: "linux", Archive: archive, TargetDataDir: target, Force: true, TripleConfirmed: true, LiveVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	mdPath := filepath.Join(res.IsolatedValidationDataDir, "rustdesk-friendly-verification-report.md")
	custom := "# RustDesk Friendly Verification Report\n\n## Manual Validation Record\n- Operator Name: Alice\n- Final Notes: verified with production observers\n"
	if err := os.WriteFile(mdPath, []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ConfirmLiveRestoreVerified(archive, res.IsolatedValidationDataDir); err != nil {
		t.Fatal(err)
	}
	updated, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), "- Operator Name: Alice") || !strings.Contains(string(updated), "- Final Notes: verified with production observers") {
		t.Fatalf("expected manual notes to be preserved, got %s", string(updated))
	}
	if !strings.Contains(string(updated), "Verification Level: `live_restore_verified`") {
		t.Fatalf("expected refreshed verification level, got %s", string(updated))
	}
}

func TestConfirmLiveRestoreMissingReportDoesNotPromoteArchive(t *testing.T) {
	tmpRoot := t.TempDir()
	t.Setenv("RUSTDESK_FRIENDLY_SKIP_DOWNLOAD", "1")
	t.Setenv("RUSTDESK_FRIENDLY_SKIP_SYSTEMCTL", "1")
	t.Setenv("RUSTDESK_FRIENDLY_SYSTEMD_DIR", filepath.Join(tmpRoot, "systemd"))
	t.Setenv("RUSTDESK_FRIENDLY_LOGROTATE_FILE", filepath.Join(tmpRoot, "logrotate", "rustdesk-server"))
	t.Setenv("RUSTDESK_FRIENDLY_JOURNALD_FILE", filepath.Join(tmpRoot, "journald", "rustdesk.conf"))
	t.Setenv("RUSTDESK_FRIENDLY_INSTALL_DIR", filepath.Join(tmpRoot, "install"))
	tmp := t.TempDir()
	archive := filepath.Join(tmp, "backup.tgz")
	source := filepath.Join(tmp, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(source, "id_ed25519"), "priv")
	mustWrite(t, filepath.Join(source, "id_ed25519.pub"), "pub")
	mustWrite(t, filepath.Join(source, "db_v2.sqlite3"), "db")
	if _, err := backup.Run(backup.Options{SourceOS: "linux", SourceDataDir: source, Output: archive}); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(tmp, "target")
	res, err := Run(Options{TargetOS: "linux", Archive: archive, TargetDataDir: target, Force: true, TripleConfirmed: true, LiveVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(res.IsolatedValidationDataDir, ".rustdesk-friendly-verification-report.json")); err != nil {
		t.Fatal(err)
	}
	err = ConfirmLiveRestoreVerified(archive, res.IsolatedValidationDataDir)
	if err == nil || !strings.Contains(err.Error(), "verification report not found") {
		t.Fatalf("expected missing report error, got %v", err)
	}
	manifest, err := backup.VerifyArchive(archive)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.VerificationLevel != bundle.VerificationRestorable || manifest.UserConfirmedLiveRestore {
		t.Fatalf("expected archive to remain restorable_verified/false, got level=%s confirmed=%v", manifest.VerificationLevel, manifest.UserConfirmedLiveRestore)
	}
	state, err := os.ReadFile(filepath.Join(res.IsolatedValidationDataDir, ".rustdesk-friendly-live-verify.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(state), `"user_confirmed_live_restore": true`) {
		t.Fatalf("expected live verify state to remain unconfirmed, got %s", string(state))
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
