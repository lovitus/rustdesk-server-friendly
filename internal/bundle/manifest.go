package bundle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lovitus/rustdesk-server-friendly/internal/common"
	"github.com/lovitus/rustdesk-server-friendly/internal/runtimeinfo"
)

const (
	VerificationArchiveValid = "archive_valid"
	VerificationRestorable   = "restorable_verified"
	VerificationLiveRestore  = "live_restore_verified"
	ManifestName             = "manifest.json"
)

type FileEntry struct {
	Path       string `json:"path"`
	SourcePath string `json:"source_path,omitempty"`
	SHA256     string `json:"sha256,omitempty"`
	Size       int64  `json:"size,omitempty"`
	Kind       string `json:"kind,omitempty"`
}

type Manifest struct {
	Version                  string              `json:"version"`
	CreatedAt                string              `json:"created_at"`
	SourceRuntime            runtimeinfo.Runtime `json:"source_runtime"`
	VerificationLevel        string              `json:"verification_level"`
	UserConfirmedLiveRestore bool                `json:"user_confirmed_live_restore"`
	Checks                   []string            `json:"checks,omitempty"`
	Warnings                 []string            `json:"warnings,omitempty"`
	BlockingIssues           []string            `json:"blocking_issues,omitempty"`
	PackageContents          []FileEntry         `json:"package_contents,omitempty"`
	RestorePlan              []string            `json:"restore_plan,omitempty"`
	RollbackState            []string            `json:"rollback_state,omitempty"`
}

func NewManifest(rt runtimeinfo.Runtime) Manifest {
	return Manifest{
		Version:           "2.6",
		CreatedAt:         time.Now().UTC().Format(time.RFC3339),
		SourceRuntime:     rt,
		VerificationLevel: VerificationArchiveValid,
	}
}

func (m *Manifest) AddFile(path, kind string) error {
	return m.AddArchiveFile(path, filepath.ToSlash(path), kind)
}

func (m *Manifest) AddArchiveFile(sourcePath, archivePath, kind string) error {
	sourcePath = strings.TrimSpace(sourcePath)
	archivePath = filepath.ToSlash(strings.TrimSpace(archivePath))
	if sourcePath == "" {
		return fmt.Errorf("source path missing")
	}
	if archivePath == "" {
		return fmt.Errorf("archive path missing")
	}
	st, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}
	hash := ""
	size := st.Size()
	if st.IsDir() {
		size = 0
	} else {
		hash, err = common.FileSHA256(sourcePath)
		if err != nil {
			return err
		}
	}
	entry := FileEntry{
		Path:       archivePath,
		SourcePath: filepath.ToSlash(sourcePath),
		SHA256:     hash,
		Size:       size,
		Kind:       kind,
	}
	m.PackageContents = append(m.PackageContents, entry)
	return nil
}

func (m *Manifest) AddVirtualFile(archivePath string, data []byte, kind string) {
	entry := FileEntry{
		Path:   filepath.ToSlash(strings.TrimSpace(archivePath)),
		SHA256: common.SHA256Bytes(data),
		Size:   int64(len(data)),
		Kind:   kind,
	}
	m.PackageContents = append(m.PackageContents, entry)
}

func (m Manifest) Marshal() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

func Parse(data []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, err
	}
	if strings.TrimSpace(m.Version) == "" {
		return Manifest{}, fmt.Errorf("manifest version missing")
	}
	return m, nil
}
