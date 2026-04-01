package bundle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lovitus/rustdesk-server-friendly/internal/runtimeinfo"
)

func TestManifestAddFileAllowsDirectoryEntries(t *testing.T) {
	tmp := t.TempDir()
	logDir := filepath.Join(tmp, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}

	m := NewManifest(runtimeinfo.Runtime{OS: "windows", Arch: "amd64"})
	if err := m.AddFile(logDir, "logs"); err != nil {
		t.Fatal(err)
	}
	if len(m.PackageContents) != 1 {
		t.Fatalf("expected 1 manifest entry, got %d", len(m.PackageContents))
	}
	if m.PackageContents[0].SHA256 != "" {
		t.Fatalf("expected empty hash for directory entry, got %q", m.PackageContents[0].SHA256)
	}
	if m.PackageContents[0].Size != 0 {
		t.Fatalf("expected size 0 for directory entry, got %d", m.PackageContents[0].Size)
	}
}
