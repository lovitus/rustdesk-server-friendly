package backup

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
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
		fmt.Fprintf(opts.Out, "[SAFE] Backup is read-only: no service stop, no source file modification, no deletion.\n")
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
		if d := chooseBestDataDir([]string{env}); d != "" {
			return d
		}
	}

	candidates := []string{}
	if sourceOS == "windows" {
		candidates = []string{
			`C:\RustDesk-Server\data`,
			`C:\rustdesk-server\data`,
			`C:\Program Files\RustDesk Server\data`,
		}
		candidates = append(candidates, detectWindowsCandidates()...)
	} else {
		candidates = []string{
			`/var/lib/rustdesk-server`,
			`/opt/rustdesk-server`,
			`/opt/rustdesk`,
		}
	}
	return chooseBestDataDir(candidates)
}

func chooseBestDataDir(candidates []string) string {
	firstExisting := ""
	seen := map[string]bool{}
	for _, c := range candidates {
		for _, d := range []string{strings.TrimSpace(c), filepath.Join(strings.TrimSpace(c), "data")} {
			if d == "" || seen[d] {
				continue
			}
			seen[d] = true
			if !isDir(d) {
				continue
			}
			if firstExisting == "" {
				firstExisting = d
			}
			if hasExpectedFiles(d) {
				return d
			}
		}
	}
	return firstExisting
}

func hasExpectedFiles(dir string) bool {
	for _, name := range []string{"id_ed25519", "id_ed25519.pub", "db_v2.sqlite3", "db.sqlite3"} {
		if st, err := os.Stat(filepath.Join(dir, name)); err == nil && !st.IsDir() {
			return true
		}
	}
	for _, p := range []string{"db_v2.sqlite3*", "db.sqlite3*"} {
		matched, _ := filepath.Glob(filepath.Join(dir, p))
		for _, m := range matched {
			if st, err := os.Stat(m); err == nil && !st.IsDir() {
				return true
			}
		}
	}
	return false
}

func detectWindowsCandidates() []string {
	out := []string{}
	out = append(out, detectWindowsPM2Candidates()...)
	out = append(out, detectWindowsServiceCandidates()...)
	out = append(out, detectWindowsProcessCandidates()...)
	return dedupe(out)
}

func detectWindowsPM2Candidates() []string {
	if !hasCmd("pm2") {
		return nil
	}
	b, err := exec.Command("pm2", "jlist").Output()
	if err != nil || len(b) == 0 {
		return nil
	}
	type pm2Env struct {
		PMCwd string `json:"pm_cwd"`
	}
	type pm2Entry struct {
		Name   string `json:"name"`
		PM2Env pm2Env `json:"pm2_env"`
	}
	var entries []pm2Entry
	if err := json.Unmarshal(b, &entries); err != nil {
		return nil
	}
	out := []string{}
	for _, e := range entries {
		n := strings.ToLower(strings.TrimSpace(e.Name))
		if !strings.Contains(n, "rustdesk") && n != "hbbs" && n != "hbbr" {
			continue
		}
		if strings.TrimSpace(e.PM2Env.PMCwd) != "" {
			out = append(out, e.PM2Env.PMCwd)
		}
	}
	return dedupe(out)
}

func detectWindowsServiceCandidates() []string {
	if !hasCmd("powershell") {
		return nil
	}
	script := `$names=@('rustdesk-hbbs','rustdesk-hbbr','rustdesksignal','rustdeskrelay','hbbs','hbbr');
foreach($n in $names){
  $p1="HKLM:\SYSTEM\CurrentControlSet\Services\$n\Parameters"
  try{$app=(Get-ItemProperty -Path $p1 -ErrorAction Stop).AppDirectory; if($app){$app}}catch{}
  $p2="HKLM:\SYSTEM\CurrentControlSet\Services\$n"
  try{
    $img=(Get-ItemProperty -Path $p2 -ErrorAction Stop).ImagePath
    if($img){
      $expanded=[Environment]::ExpandEnvironmentVariables($img)
      $m=[regex]::Match($expanded,'^\"?([^\" ]+\.exe)')
      if($m.Success){Split-Path -Parent $m.Groups[1].Value}
    }
  }catch{}
}`
	b, err := exec.Command("powershell", "-NoProfile", "-Command", script).Output()
	if err != nil || len(b) == 0 {
		return nil
	}
	out := []string{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return dedupe(out)
}

func detectWindowsProcessCandidates() []string {
	if !hasCmd("powershell") {
		return nil
	}
	script := `Get-Process -Name hbbs,hbbr,rustdesksignal,rustdeskrelay -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Path`
	b, err := exec.Command("powershell", "-NoProfile", "-Command", script).Output()
	if err != nil || len(b) == 0 {
		return nil
	}
	out := []string{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, filepath.Dir(line))
	}
	return dedupe(out)
}

func hasCmd(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
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
