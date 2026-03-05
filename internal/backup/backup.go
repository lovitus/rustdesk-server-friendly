package backup

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Options struct {
	SourceOS      string
	SourceDataDir string
	Output        string
	Force         bool
	Out           io.Writer
}

type Result struct {
	ArchivePath string
	SHA256      string
	Files       []string
}

func Run(opts Options) (Result, error) {
	sourceOS := strings.ToLower(strings.TrimSpace(opts.SourceOS))
	if sourceOS == "" {
		if runtime.GOOS == "windows" {
			sourceOS = "windows"
		} else {
			sourceOS = "linux"
		}
	}
	if sourceOS != "windows" && sourceOS != "linux" {
		return Result{}, fmt.Errorf("unsupported source OS: %s", sourceOS)
	}

	dataDir := strings.TrimSpace(opts.SourceDataDir)
	if dataDir == "" {
		dataDir = detectDataDir(sourceOS)
	}
	if dataDir == "" {
		return Result{}, fmt.Errorf("cannot detect RustDesk source data dir; set --source-data-dir or RUSTDESK_SOURCE_DATA_DIR")
	}
	if st, err := os.Stat(dataDir); err != nil || !st.IsDir() {
		return Result{}, fmt.Errorf("source data dir not found: %s", dataDir)
	}

	files, err := collectFiles(dataDir)
	if err != nil {
		return Result{}, err
	}
	if len(files) == 0 {
		return Result{}, fmt.Errorf("no migration files found in %s", dataDir)
	}

	archivePath := strings.TrimSpace(opts.Output)
	if archivePath == "" {
		if sourceOS == "windows" {
			archivePath = `C:\rustdesk-migration-backup\rustdesk-migration-backup.zip`
		} else {
			archivePath = `/tmp/rustdesk-migration-backup.tgz`
		}
	}
	absArchive, err := filepath.Abs(archivePath)
	if err == nil {
		archivePath = absArchive
	}

	if _, err := os.Stat(archivePath); err == nil && !opts.Force {
		return Result{}, fmt.Errorf("archive already exists: %s (use --force to overwrite)", archivePath)
	}
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		return Result{}, err
	}

	if sourceOS == "windows" {
		err = writeZip(archivePath, files)
	} else {
		err = writeTarGz(archivePath, files)
	}
	if err != nil {
		return Result{}, err
	}

	hash, err := fileSHA256(archivePath)
	if err != nil {
		return Result{}, err
	}

	if opts.Out != nil {
		fmt.Fprintf(opts.Out, "[OK] Source data dir: %s\n", dataDir)
		fmt.Fprintf(opts.Out, "[OK] Files packed: %d\n", len(files))
		for _, f := range files {
			fmt.Fprintf(opts.Out, "  - %s\n", filepath.Base(f))
		}
		fmt.Fprintf(opts.Out, "[OK] Archive: %s\n", archivePath)
		fmt.Fprintf(opts.Out, "[OK] SHA256: %s\n", hash)
	}

	return Result{ArchivePath: archivePath, SHA256: hash, Files: files}, nil
}

func detectDataDir(sourceOS string) string {
	if env := strings.TrimSpace(os.Getenv("RUSTDESK_SOURCE_DATA_DIR")); env != "" {
		if isDir(env) {
			return env
		}
	}

	candidates := []string{}
	if sourceOS == "windows" {
		candidates = []string{
			`C:\RustDesk-Server\data`,
			`C:\rustdesk-server\data`,
			`C:\Program Files\RustDesk Server\data`,
		}
	} else {
		candidates = []string{
			`/var/lib/rustdesk-server`,
			`/opt/rustdesk-server`,
			`/opt/rustdesk`,
		}
	}

	for _, d := range candidates {
		if isDir(d) {
			return d
		}
	}
	return ""
}

func isDir(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

func collectFiles(dataDir string) ([]string, error) {
	files := []string{}
	for _, name := range []string{"id_ed25519", "id_ed25519.pub"} {
		p := filepath.Join(dataDir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			files = append(files, p)
		}
	}

	patterns := []string{"db_v2.sqlite3*", "db.sqlite3*"}
	for _, p := range patterns {
		matched, err := filepath.Glob(filepath.Join(dataDir, p))
		if err != nil {
			return nil, err
		}
		for _, m := range matched {
			if st, err := os.Stat(m); err == nil && !st.IsDir() {
				files = append(files, m)
			}
		}
	}

	uniq := map[string]bool{}
	out := []string{}
	for _, f := range files {
		if !uniq[f] {
			uniq[f] = true
			out = append(out, f)
		}
	}
	return out, nil
}

func writeZip(out string, files []string) error {
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	for _, src := range files {
		if err := addZipFile(zw, src, filepath.Base(src)); err != nil {
			return err
		}
	}
	return nil
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

func writeTarGz(out string, files []string) error {
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	for _, src := range files {
		if err := addTarFile(tw, src, filepath.Base(src)); err != nil {
			return err
		}
	}
	return nil
}

func addTarFile(tw *tar.Writer, src, name string) error {
	st, err := os.Stat(src)
	if err != nil {
		return err
	}
	h := &tar.Header{
		Name:    name,
		Mode:    int64(st.Mode().Perm()),
		Size:    st.Size(),
		ModTime: st.ModTime(),
	}
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

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
