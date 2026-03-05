package restore

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestRunImportZipToLinuxDir(t *testing.T) {
	tmp := t.TempDir()
	archive := filepath.Join(tmp, "backup.zip")
	makeZip(t, archive, map[string]string{
		"id_ed25519":     "priv",
		"id_ed25519.pub": "pub",
		"db_v2.sqlite3":  "db",
	})

	target := filepath.Join(tmp, "target")
	res, err := Run(Options{TargetOS: "linux", Archive: archive, TargetDataDir: target, Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.RestoredFiles) != 3 {
		t.Fatalf("expected 3 restored files, got %d", len(res.RestoredFiles))
	}
	if _, err := os.Stat(filepath.Join(target, "id_ed25519")); err != nil {
		t.Fatal(err)
	}
}

func TestRunRejectsInvalidArchive(t *testing.T) {
	tmp := t.TempDir()
	archive := filepath.Join(tmp, "invalid.zip")
	makeZip(t, archive, map[string]string{"random.txt": "nope"})
	_, err := Run(Options{TargetOS: "linux", Archive: archive, TargetDataDir: filepath.Join(tmp, "target")})
	if err == nil {
		t.Fatal("expected error")
	}
}

func makeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for n, c := range files {
		w, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(c)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}
