package restore

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Options struct {
	TargetOS      string
	Archive       string
	TargetDataDir string
	Force         bool
	Out           io.Writer
}

type Result struct {
	TargetDataDir string
	RestoredFiles []string
	PreBackupDir  string
}

func Run(opts Options) (Result, error) {
	targetOS := strings.ToLower(strings.TrimSpace(opts.TargetOS))
	if targetOS == "" {
		if runtime.GOOS == "windows" {
			targetOS = "windows"
		} else {
			targetOS = "linux"
		}
	}
	if targetOS != "windows" && targetOS != "linux" {
		return Result{}, fmt.Errorf("unsupported target OS: %s", targetOS)
	}

	archivePath := strings.TrimSpace(opts.Archive)
	if archivePath == "" {
		return Result{}, errors.New("archive path is required")
	}
	absArchive, err := filepath.Abs(archivePath)
	if err == nil {
		archivePath = absArchive
	}
	st, err := os.Stat(archivePath)
	if err != nil || st.IsDir() {
		return Result{}, fmt.Errorf("archive not found: %s", archivePath)
	}

	extractDir, files, err := extractArchive(archivePath)
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(extractDir)

	if err := validateFileSet(files); err != nil {
		return Result{}, err
	}

	targetDataDir := strings.TrimSpace(opts.TargetDataDir)
	if targetDataDir == "" {
		targetDataDir = detectTargetDataDir(targetOS)
	}
	if targetDataDir == "" {
		return Result{}, errors.New("cannot detect target data dir; set --target-data-dir or RUSTDESK_TARGET_DATA_DIR")
	}
	absTarget, err := filepath.Abs(targetDataDir)
	if err == nil {
		targetDataDir = absTarget
	}

	if err := os.MkdirAll(targetDataDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("cannot create target data dir: %w", err)
	}

	conflicts := existingConflicts(targetDataDir, files)
	if len(conflicts) > 0 && !opts.Force {
		return Result{}, fmt.Errorf("target contains existing migration files (%d). Use --force to overwrite", len(conflicts))
	}

	preBackupDir := ""
	if len(conflicts) > 0 {
		preBackupDir = filepath.Join(targetDataDir, ".rustdesk-friendly-preimport-"+time.Now().Format("20060102-150405"))
		if err := os.MkdirAll(preBackupDir, 0o755); err != nil {
			return Result{}, err
		}
		for _, c := range conflicts {
			dst := filepath.Join(preBackupDir, filepath.Base(c))
			if err := copyFile(c, dst); err != nil {
				return Result{}, err
			}
		}
	}

	logf(opts.Out, "[INFO] Archive: %s", archivePath)
	logf(opts.Out, "[INFO] Target data dir: %s", targetDataDir)
	if preBackupDir != "" {
		logf(opts.Out, "[INFO] Existing files backed up: %s", preBackupDir)
	}

	stopServices(targetOS, opts.Out)

	restored := []string{}
	for _, f := range files {
		src := filepath.Join(extractDir, filepath.Base(f))
		dst := filepath.Join(targetDataDir, filepath.Base(f))
		if err := copyFile(src, dst); err != nil {
			return Result{}, err
		}
		restored = append(restored, dst)
	}

	if targetOS == "linux" {
		_ = os.Chmod(filepath.Join(targetDataDir, "id_ed25519"), 0o600)
		_ = os.Chmod(filepath.Join(targetDataDir, "id_ed25519.pub"), 0o644)
	}

	startServices(targetOS, opts.Out)
	verifyServices(targetOS, opts.Out)

	for _, r := range restored {
		logf(opts.Out, "[OK] Restored: %s", r)
	}
	logf(opts.Out, "[OK] Import completed")

	return Result{TargetDataDir: targetDataDir, RestoredFiles: restored, PreBackupDir: preBackupDir}, nil
}

func extractArchive(path string) (string, []string, error) {
	tmp, err := os.MkdirTemp("", "rustdesk-friendly-import-")
	if err != nil {
		return "", nil, err
	}

	lower := strings.ToLower(path)
	saved := []string{}

	save := func(name string, r io.Reader) error {
		base := filepath.Base(name)
		if !isAllowedFile(base) {
			return nil
		}
		outPath := filepath.Join(tmp, base)
		out, err := os.Create(outPath)
		if err != nil {
			return err
		}
		defer out.Close()
		if _, err := io.Copy(out, r); err != nil {
			return err
		}
		saved = append(saved, outPath)
		return nil
	}

	if strings.HasSuffix(lower, ".zip") {
		zr, err := zip.OpenReader(path)
		if err != nil {
			return "", nil, err
		}
		defer zr.Close()
		for _, f := range zr.File {
			if f.FileInfo().IsDir() {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				return "", nil, err
			}
			if err := save(f.Name, rc); err != nil {
				rc.Close()
				return "", nil, err
			}
			rc.Close()
		}
		return tmp, saved, nil
	}

	if strings.HasSuffix(lower, ".tgz") || strings.HasSuffix(lower, ".tar.gz") {
		f, err := os.Open(path)
		if err != nil {
			return "", nil, err
		}
		defer f.Close()
		gz, err := gzip.NewReader(f)
		if err != nil {
			return "", nil, err
		}
		defer gz.Close()
		tr := tar.NewReader(gz)
		for {
			h, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", nil, err
			}
			if h.FileInfo().IsDir() {
				continue
			}
			if err := save(h.Name, tr); err != nil {
				return "", nil, err
			}
		}
		return tmp, saved, nil
	}

	return "", nil, fmt.Errorf("unsupported archive format: %s", path)
}

func validateFileSet(files []string) error {
	if len(files) == 0 {
		return errors.New("archive has no valid RustDesk migration files")
	}
	hasPriv, hasPub := false, false
	for _, f := range files {
		b := filepath.Base(f)
		if b == "id_ed25519" {
			hasPriv = true
		}
		if b == "id_ed25519.pub" {
			hasPub = true
		}
		if !isAllowedFile(b) {
			return fmt.Errorf("archive contains unsupported file: %s", b)
		}
	}
	if !hasPriv || !hasPub {
		return errors.New("archive invalid: missing id_ed25519 or id_ed25519.pub")
	}
	return nil
}

func isAllowedFile(name string) bool {
	if name == "id_ed25519" || name == "id_ed25519.pub" {
		return true
	}
	if strings.HasPrefix(name, "db_v2.sqlite3") || strings.HasPrefix(name, "db.sqlite3") {
		return true
	}
	return false
}

func detectTargetDataDir(targetOS string) string {
	if env := strings.TrimSpace(os.Getenv("RUSTDESK_TARGET_DATA_DIR")); env != "" {
		return env
	}
	if targetOS == "windows" {
		for _, p := range []string{`C:\RustDesk-Server\data`, `C:\rustdesk-server\data`, `C:\Program Files\RustDesk Server\data`} {
			if isDir(p) {
				return p
			}
		}
		return `C:\RustDesk-Server\data`
	}
	for _, p := range []string{"/var/lib/rustdesk-server", "/opt/rustdesk-server", "/opt/rustdesk"} {
		if isDir(p) {
			return p
		}
	}
	return "/var/lib/rustdesk-server"
}

func isDir(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

func existingConflicts(targetDir string, extracted []string) []string {
	out := []string{}
	for _, f := range extracted {
		p := filepath.Join(targetDir, filepath.Base(f))
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			out = append(out, p)
		}
	}
	return out
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func stopServices(targetOS string, out io.Writer) {
	logf(out, "[INFO] Stopping RustDesk services (best effort)...")
	if targetOS == "linux" {
		for _, svc := range []string{"rustdesk-hbbs", "rustdesk-hbbr", "hbbs", "hbbr"} {
			runCmd(out, "systemctl", "stop", svc)
		}
		return
	}
	if hasCmd("pm2") {
		runCmd(out, "pm2", "stop", "rustdesk-hbbs")
		runCmd(out, "pm2", "stop", "rustdesk-hbbr")
	}
	for _, svc := range []string{"rustdesk-hbbs", "rustdesk-hbbr", "rustdesksignal", "rustdeskrelay", "hbbs", "hbbr"} {
		runCmd(out, "sc", "stop", svc)
	}
}

func startServices(targetOS string, out io.Writer) {
	logf(out, "[INFO] Starting RustDesk services (best effort)...")
	if targetOS == "linux" {
		for _, svc := range []string{"rustdesk-hbbs", "rustdesk-hbbr", "hbbs", "hbbr"} {
			runCmd(out, "systemctl", "start", svc)
		}
		return
	}
	if hasCmd("pm2") {
		runCmd(out, "pm2", "start", "rustdesk-hbbs")
		runCmd(out, "pm2", "start", "rustdesk-hbbr")
	}
	for _, svc := range []string{"rustdesk-hbbs", "rustdesk-hbbr", "rustdesksignal", "rustdeskrelay", "hbbs", "hbbr"} {
		runCmd(out, "sc", "start", svc)
	}
}

func verifyServices(targetOS string, out io.Writer) {
	logf(out, "[INFO] Verifying service state (best effort)...")
	if targetOS == "linux" {
		for _, svc := range []string{"rustdesk-hbbs", "rustdesk-hbbr"} {
			runCmd(out, "systemctl", "is-active", svc)
		}
		return
	}
	for _, svc := range []string{"rustdesk-hbbs", "rustdesk-hbbr"} {
		runCmd(out, "sc", "query", svc)
	}
}

func hasCmd(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func runCmd(out io.Writer, name string, args ...string) {
	if !hasCmd(name) {
		return
	}
	cmd := exec.Command(name, args...)
	b, err := cmd.CombinedOutput()
	if err != nil {
		if len(strings.TrimSpace(string(b))) > 0 {
			logf(out, "[WARN] %s %s -> %s", name, strings.Join(args, " "), strings.TrimSpace(string(b)))
		}
		return
	}
	if len(strings.TrimSpace(string(b))) > 0 {
		logf(out, "[INFO] %s %s -> %s", name, strings.Join(args, " "), strings.TrimSpace(string(b)))
	}
}

func logf(out io.Writer, format string, args ...any) {
	if out == nil {
		return
	}
	fmt.Fprintf(out, format+"\n", args...)
}
