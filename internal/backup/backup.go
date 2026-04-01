package backup

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/lovitus/rustdesk-server-friendly/internal/bundle"
	"github.com/lovitus/rustdesk-server-friendly/internal/common"
	"github.com/lovitus/rustdesk-server-friendly/internal/runtimeinfo"
)

type Options struct {
	SourceOS      string
	SourceDataDir string
	Output        string
	Force         bool
	Out           io.Writer
}

type Result struct {
	ArchivePath       string
	SHA256            string
	Files             []string
	Checks            []string
	Warnings          []string
	BlockingIssues    []string
	DetectedRuntime   runtimeinfo.Runtime
	ServiceManager    string
	PackageContents   []bundle.FileEntry
	VerificationLevel string
}

type archiveEntry struct {
	Src  string
	Dst  string
	Kind string
}

type ArchiveRewriteEntry struct {
	Src string
	Dst string
}

func Run(opts Options) (Result, error) {
	sourceOS := normalizeOS(opts.SourceOS)
	rt := runtimeinfo.Detect(sourceOS)
	if sourceOS == "" {
		sourceOS = rt.OS
	}
	if !rt.Supported {
		return Result{}, fmt.Errorf("unsupported source runtime: %s/%s: %s", rt.OS, rt.Arch, rt.SupportReason)
	}

	if strings.TrimSpace(opts.SourceDataDir) != "" {
		rt.DataDir = strings.TrimSpace(opts.SourceDataDir)
	}
	if rt.DataDir == "" {
		return Result{}, fmt.Errorf("cannot detect RustDesk source data dir")
	}
	if st, err := os.Stat(rt.DataDir); err != nil || !st.IsDir() {
		return Result{}, fmt.Errorf("source data dir not found: %s", rt.DataDir)
	}

	entries, warnings, err := collectEntries(rt)
	if err != nil {
		return Result{}, err
	}
	if len(entries) == 0 {
		return Result{}, fmt.Errorf("no RustDesk content found to back up")
	}

	archivePath := strings.TrimSpace(opts.Output)
	if archivePath == "" {
		if sourceOS == "windows" {
			archivePath = `C:\rustdesk-migration-backup\rustdesk-lifecycle-backup.zip`
		} else {
			archivePath = `/tmp/rustdesk-lifecycle-backup.tgz`
		}
	}
	archivePath = common.Abs(archivePath)
	if _, err := os.Stat(archivePath); err == nil && !opts.Force {
		return Result{}, fmt.Errorf("archive already exists: %s", archivePath)
	}
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		return Result{}, err
	}

	manifest := bundle.NewManifest(rt)
	manifest.Warnings = append(manifest.Warnings, warnings...)
	manifest.Checks = append(manifest.Checks,
		"backup is read-only and does not stop or modify the source service",
		"archive manifest generated",
	)
	manifest.RestorePlan = defaultRestorePlan(rt)

	for _, entry := range entries {
		if err := manifest.AddFile(entry.Src, entry.Kind); err != nil {
			return Result{}, err
		}
	}

	data, err := manifest.Marshal()
	if err != nil {
		return Result{}, err
	}

	if sourceOS == "windows" {
		err = writeZip(archivePath, entries, data)
	} else {
		err = writeTarGz(archivePath, entries, data)
	}
	if err != nil {
		return Result{}, err
	}

	verifiedManifest, err := VerifyArchive(archivePath)
	if err != nil {
		return Result{}, err
	}
	verifiedManifest.VerificationLevel = bundle.VerificationRestorable
	verifiedManifest.Checks = append(verifiedManifest.Checks, "archive reopened and manifest revalidated")
	if err := rewriteManifest(archivePath, sourceOS, verifiedManifest); err != nil {
		return Result{}, err
	}

	hash, err := common.FileSHA256(archivePath)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		ArchivePath:       archivePath,
		SHA256:            hash,
		Checks:            verifiedManifest.Checks,
		Warnings:          verifiedManifest.Warnings,
		DetectedRuntime:   rt,
		ServiceManager:    rt.ServiceManager,
		PackageContents:   verifiedManifest.PackageContents,
		VerificationLevel: verifiedManifest.VerificationLevel,
	}
	for _, entry := range entries {
		result.Files = append(result.Files, entry.Dst)
	}
	sort.Strings(result.Files)
	logResult(opts.Out, result)
	return result, nil
}

func VerifyArchive(path string) (bundle.Manifest, error) {
	tmp, files, manifest, err := extractToTemp(path)
	if err != nil {
		return bundle.Manifest{}, err
	}
	defer os.RemoveAll(tmp)
	if manifest.Version == "" {
		return bundle.Manifest{}, fmt.Errorf("archive missing manifest")
	}
	if len(files) == 0 {
		return bundle.Manifest{}, fmt.Errorf("archive has no restorable content")
	}
	hasPriv, hasPub := false, false
	for _, f := range files {
		base := filepath.Base(f)
		if base == "id_ed25519" {
			hasPriv = true
		}
		if base == "id_ed25519.pub" {
			hasPub = true
		}
	}
	if !hasPriv || !hasPub {
		return bundle.Manifest{}, fmt.Errorf("archive invalid: missing id_ed25519 or id_ed25519.pub")
	}
	return manifest, nil
}

func ExtractArchiveForRestore(path string) (string, []string, bundle.Manifest, error) {
	return extractToTemp(path)
}

func extractToTemp(path string) (string, []string, bundle.Manifest, error) {
	tmp, err := os.MkdirTemp("", "rustdesk-friendly-backup-verify-")
	if err != nil {
		return "", nil, bundle.Manifest{}, err
	}
	lower := strings.ToLower(path)
	files := []string{}
	var manifest bundle.Manifest
	save := func(name string, r io.Reader) error {
		base := filepath.Base(name)
		outPath := filepath.Join(tmp, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		if base == bundle.ManifestName {
			manifest, err = bundle.Parse(data)
			return err
		}
		if err := os.WriteFile(outPath, data, 0o644); err != nil {
			return err
		}
		files = append(files, outPath)
		return nil
	}
	if strings.HasSuffix(lower, ".zip") {
		zr, err := zip.OpenReader(path)
		if err != nil {
			return "", nil, bundle.Manifest{}, err
		}
		defer zr.Close()
		for _, f := range zr.File {
			if f.FileInfo().IsDir() {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				return "", nil, bundle.Manifest{}, err
			}
			if err := save(f.Name, rc); err != nil {
				rc.Close()
				return "", nil, bundle.Manifest{}, err
			}
			rc.Close()
		}
		return tmp, files, manifest, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return "", nil, bundle.Manifest{}, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", nil, bundle.Manifest{}, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", nil, bundle.Manifest{}, err
		}
		if h.FileInfo().IsDir() {
			continue
		}
		if err := save(h.Name, tr); err != nil {
			return "", nil, bundle.Manifest{}, err
		}
	}
	return tmp, files, manifest, nil
}

func collectEntries(rt runtimeinfo.Runtime) ([]archiveEntry, []string, error) {
	entries := []archiveEntry{}
	warnings := []string{}
	addFile := func(src, dst, kind string) {
		if src == "" || !isFile(src) {
			return
		}
		entries = append(entries, archiveEntry{Src: src, Dst: dst, Kind: kind})
	}

	for _, name := range []string{"id_ed25519", "id_ed25519.pub"} {
		addFile(filepath.Join(rt.DataDir, name), filepath.ToSlash(filepath.Join("data", name)), "data")
	}
	patterns := []string{"db_v2.sqlite3*", "db.sqlite3*"}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(rt.DataDir, pattern))
		if err != nil {
			return nil, nil, err
		}
		for _, match := range matches {
			addFile(match, filepath.ToSlash(filepath.Join("data", filepath.Base(match))), "data")
		}
	}

	for name, path := range rt.BinaryPaths {
		ext := filepath.Ext(path)
		dst := filepath.ToSlash(filepath.Join("app", name+ext))
		addFile(path, dst, "app")
	}
	if len(rt.BinaryPaths) == 0 {
		warnings = append(warnings, "running binaries were not detected; restore will rely on target-side download")
	}

	for _, path := range rt.ServiceDefinitions {
		addFile(path, filepath.ToSlash(filepath.Join("service", sanitizeName(path))), "service")
	}
	if rt.LogDir != "" && isDir(rt.LogDir) {
		entries = append(entries, archiveEntry{
			Src:  rt.LogDir,
			Dst:  filepath.ToSlash(filepath.Join("logs", "snapshot.json")),
			Kind: "logs",
		})
	}

	if len(entries) == 0 {
		return nil, warnings, nil
	}
	return entries, warnings, nil
}

func chooseBestDataDir(candidates []string) string {
	firstExisting := ""
	seen := map[string]bool{}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		for _, dir := range []string{candidate, filepath.Join(candidate, "data")} {
			if dir == "" || seen[dir] {
				continue
			}
			seen[dir] = true
			if !isDir(dir) {
				continue
			}
			if firstExisting == "" {
				firstExisting = dir
			}
			for _, name := range []string{"id_ed25519", "id_ed25519.pub", "db_v2.sqlite3", "db.sqlite3"} {
				if isFile(filepath.Join(dir, name)) {
					return dir
				}
			}
		}
	}
	return firstExisting
}

func defaultRestorePlan(rt runtimeinfo.Runtime) []string {
	plan := []string{
		"validate archive manifest and required files",
		"prepare target staging directory",
		"map or download target binaries for the destination OS/arch",
		"restore data into staging area",
		"create or repair managed service definitions",
		"run health checks before cutover",
	}
	if rt.ExistingService {
		plan = append(plan, "backup current target state and enable rollback before any cutover")
	}
	return plan
}

func rewriteManifest(path, sourceOS string, manifest bundle.Manifest) error {
	tmpDir, err := os.MkdirTemp("", "rustdesk-friendly-manifest-rewrite-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	verifyDir, files, _, err := extractToTemp(path)
	if err != nil {
		return err
	}
	defer os.RemoveAll(verifyDir)
	entries := []archiveEntry{}
	for _, f := range files {
		rel, err := filepath.Rel(verifyDir, f)
		if err != nil {
			return err
		}
		entries = append(entries, archiveEntry{Src: f, Dst: filepath.ToSlash(rel)})
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	newPath := filepath.Join(tmpDir, filepath.Base(path))
	if sourceOS == "windows" {
		if err := writeZip(newPath, entries, data); err != nil {
			return err
		}
	} else {
		if err := writeTarGz(newPath, entries, data); err != nil {
			return err
		}
	}
	return os.Rename(newPath, path)
}

func RewriteArchiveManifest(path string, entries []ArchiveRewriteEntry, manifest bundle.Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	internalEntries := make([]archiveEntry, 0, len(entries))
	for _, entry := range entries {
		internalEntries = append(internalEntries, archiveEntry{Src: entry.Src, Dst: entry.Dst})
	}
	tmpDir, err := os.MkdirTemp("", "rustdesk-friendly-archive-rewrite-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	newPath := filepath.Join(tmpDir, filepath.Base(path))
	if strings.HasSuffix(strings.ToLower(path), ".zip") {
		if err := writeZip(newPath, internalEntries, data); err != nil {
			return err
		}
	} else {
		if err := writeTarGz(newPath, internalEntries, data); err != nil {
			return err
		}
	}
	return os.Rename(newPath, path)
}

func writeZip(out string, entries []archiveEntry, manifest []byte) error {
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	defer zw.Close()
	for _, entry := range entries {
		if entry.Kind == "logs" && isDir(entry.Src) {
			if err := writeLogSnapshotZip(zw, entry); err != nil {
				return err
			}
			continue
		}
		if err := addZipFile(zw, entry.Src, entry.Dst); err != nil {
			return err
		}
	}
	w, err := zw.Create(bundle.ManifestName)
	if err != nil {
		return err
	}
	_, err = w.Write(manifest)
	return err
}

func writeTarGz(out string, entries []archiveEntry, manifest []byte) error {
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()
	for _, entry := range entries {
		if entry.Kind == "logs" && isDir(entry.Src) {
			if err := writeLogSnapshotTar(tw, entry); err != nil {
				return err
			}
			continue
		}
		if err := addTarFile(tw, entry.Src, entry.Dst); err != nil {
			return err
		}
	}
	return addTarBytes(tw, bundle.ManifestName, manifest)
}

func writeLogSnapshotZip(zw *zip.Writer, entry archiveEntry) error {
	data, err := json.MarshalIndent(map[string]any{
		"log_dir": entry.Src,
	}, "", "  ")
	if err != nil {
		return err
	}
	w, err := zw.Create(entry.Dst)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func writeLogSnapshotTar(tw *tar.Writer, entry archiveEntry) error {
	data, err := json.MarshalIndent(map[string]any{
		"log_dir": entry.Src,
	}, "", "  ")
	if err != nil {
		return err
	}
	return addTarBytes(tw, entry.Dst, data)
}

func addZipFile(zw *zip.Writer, src, name string) error {
	st, err := os.Stat(src)
	if err != nil {
		return err
	}
	h, err := zip.FileInfoHeader(st)
	if err != nil {
		return err
	}
	h.Name = name
	h.Method = zip.Deflate
	w, err := zw.CreateHeader(h)
	if err != nil {
		return err
	}
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}

func addTarFile(tw *tar.Writer, src, name string) error {
	st, err := os.Stat(src)
	if err != nil {
		return err
	}
	h := &tar.Header{Name: name, Mode: int64(st.Mode().Perm()), Size: st.Size(), ModTime: st.ModTime()}
	if err := tw.WriteHeader(h); err != nil {
		return err
	}
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(tw, f)
	return err
}

func addTarBytes(tw *tar.Writer, name string, data []byte) error {
	h := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(data))}
	if err := tw.WriteHeader(h); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

func normalizeOS(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		if runtime.GOOS == "windows" {
			return "windows"
		}
		return runtime.GOOS
	}
	return v
}

func sanitizeName(path string) string {
	path = strings.ReplaceAll(path, ":", "")
	path = strings.ReplaceAll(path, `\`, "_")
	path = strings.ReplaceAll(path, "/", "_")
	return path
}

func isFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func isDir(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

func logResult(out io.Writer, result Result) {
	if out == nil {
		return
	}
	fmt.Fprintln(out, "[SAFE] Backup is read-only: no service stop, no source file modification, no deletion.")
	fmt.Fprintf(out, "[OK] Source runtime: %s/%s\n", result.DetectedRuntime.OS, result.DetectedRuntime.Arch)
	fmt.Fprintf(out, "[OK] Service manager: %s\n", emptyOr(result.ServiceManager, "not detected"))
	fmt.Fprintf(out, "[OK] Verification level: %s\n", result.VerificationLevel)
	fmt.Fprintf(out, "[OK] Archive: %s\n", result.ArchivePath)
	fmt.Fprintf(out, "[OK] SHA256: %s\n", result.SHA256)
	for _, check := range result.Checks {
		fmt.Fprintf(out, "[CHECK] %s\n", check)
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(out, "[WARN] %s\n", warning)
	}
}

func emptyOr(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
