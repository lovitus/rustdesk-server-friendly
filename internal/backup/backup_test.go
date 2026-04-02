package backup

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/lovitus/rustdesk-server-friendly/internal/bundle"
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

func TestVerifyArchiveRejectsUndeclaredFile(t *testing.T) {
	tmp := t.TempDir()
	data := filepath.Join(tmp, "data")
	mustMkdir(t, data)
	mustWrite(t, filepath.Join(data, "id_ed25519"), "priv")
	mustWrite(t, filepath.Join(data, "id_ed25519.pub"), "pub")
	mustWrite(t, filepath.Join(data, "db_v2.sqlite3"), "db")
	baseArchive := filepath.Join(tmp, "base.zip")
	if _, err := Run(Options{SourceOS: "windows", SourceDataDir: data, Output: baseArchive}); err != nil {
		t.Fatal(err)
	}

	tmpDir, files, manifest, err := ExtractArchiveForRestore(baseArchive)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	extraPath := filepath.Join(tmpDir, "evil.txt")
	mustWrite(t, extraPath, "evil")

	out := filepath.Join(tmp, "tampered.zip")
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for _, file := range files {
		rel, err := filepath.Rel(tmpDir, file)
		if err != nil {
			t.Fatal(err)
		}
		w, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			t.Fatal(err)
		}
		in, err := os.Open(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(w, in); err != nil {
			in.Close()
			t.Fatal(err)
		}
		in.Close()
	}
	w, err := zw.Create("evil.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("evil")); err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	w, err = zw.Create(bundle.ManifestName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(manifestBytes); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := VerifyArchive(out); err == nil {
		t.Fatal("expected undeclared-file validation failure")
	}
}

func TestVerifyArchiveRejectsHashMismatch(t *testing.T) {
	tmp := t.TempDir()
	data := filepath.Join(tmp, "data")
	mustMkdir(t, data)
	mustWrite(t, filepath.Join(data, "id_ed25519"), "priv")
	mustWrite(t, filepath.Join(data, "id_ed25519.pub"), "pub")
	mustWrite(t, filepath.Join(data, "db_v2.sqlite3"), "db")
	baseArchive := filepath.Join(tmp, "base.zip")
	if _, err := Run(Options{SourceOS: "windows", SourceDataDir: data, Output: baseArchive}); err != nil {
		t.Fatal(err)
	}

	tmpDir, files, manifest, err := ExtractArchiveForRestore(baseArchive)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	for _, file := range files {
		if filepath.Base(file) == "db_v2.sqlite3" {
			mustWrite(t, file, "tampered-db")
			break
		}
	}
	rewriteEntries := make([]ArchiveRewriteEntry, 0, len(files))
	for _, file := range files {
		rel, err := filepath.Rel(tmpDir, file)
		if err != nil {
			t.Fatal(err)
		}
		rewriteEntries = append(rewriteEntries, ArchiveRewriteEntry{Src: file, Dst: filepath.ToSlash(rel)})
	}
	tampered := filepath.Join(tmp, "tampered.zip")
	if err := RewriteArchiveManifest(tampered, rewriteEntries, manifest); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyArchive(tampered); err == nil {
		t.Fatal("expected hash mismatch failure")
	}
}

func TestVerifyArchiveRejectsPathTraversal(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "traversal.zip")
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("../escape.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("escape")); err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := json.MarshalIndent(bundle.Manifest{Version: "1"}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	w, err = zw.Create(bundle.ManifestName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(manifestBytes); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyArchive(out); err == nil {
		t.Fatal("expected path traversal validation failure")
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
