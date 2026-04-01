package backup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunWindowsBackupFromExplicitDir(t *testing.T) {
	tmp := t.TempDir()
	data := filepath.Join(tmp, "data")
	if err := os.MkdirAll(data, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(data, "id_ed25519"), "priv")
	mustWrite(t, filepath.Join(data, "id_ed25519.pub"), "pub")
	mustWrite(t, filepath.Join(data, "db_v2.sqlite3"), "db")

	out := filepath.Join(tmp, "bundle.zip")
	res, err := Run(Options{SourceOS: "windows", SourceDataDir: data, Output: out})
	if err != nil {
		t.Fatal(err)
	}
	if res.ArchivePath == "" || res.SHA256 == "" {
		t.Fatal("expected archive path and sha")
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("archive missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "rustdesk-friendly-backup-report.json")); err != nil {
		t.Fatalf("backup report missing: %v", err)
	}
}

func TestRunLinuxBackupNoFiles(t *testing.T) {
	tmp := t.TempDir()
	_, err := Run(Options{SourceOS: "linux", SourceDataDir: tmp, Output: filepath.Join(tmp, "x.tgz")})
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

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
