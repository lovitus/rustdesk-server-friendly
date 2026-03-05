package backup

import (
	"path/filepath"
	"testing"
)

func TestChooseBestDataDirPrefersExpectedFiles(t *testing.T) {
	tmp := t.TempDir()
	a := filepath.Join(tmp, "a")
	b := filepath.Join(tmp, "b")
	mustMkdir(t, a)
	mustMkdir(t, b)
	mustWrite(t, filepath.Join(b, "id_ed25519"), "x")

	got := chooseBestDataDir([]string{a, b})
	if got != b {
		t.Fatalf("expected %s, got %s", b, got)
	}
}

func TestChooseBestDataDirFallbackFirstExisting(t *testing.T) {
	tmp := t.TempDir()
	a := filepath.Join(tmp, "a")
	mustMkdir(t, a)

	got := chooseBestDataDir([]string{a})
	if got != a {
		t.Fatalf("expected %s, got %s", a, got)
	}
}
