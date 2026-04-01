package runtimeinfo

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/lovitus/rustdesk-server-friendly/internal/platform"
)

type Runtime struct {
	OS                 string            `json:"os"`
	Arch               string            `json:"arch"`
	Supported          bool              `json:"supported"`
	ManagedService     bool              `json:"managed_service"`
	SupportReason      string            `json:"support_reason,omitempty"`
	ServiceManager     string            `json:"service_manager,omitempty"`
	ExistingService    bool              `json:"existing_service"`
	DataDir            string            `json:"data_dir,omitempty"`
	InstallDir         string            `json:"install_dir,omitempty"`
	LogDir             string            `json:"log_dir,omitempty"`
	BinaryPaths        map[string]string `json:"binary_paths,omitempty"`
	ServiceDefinitions []string          `json:"service_definitions,omitempty"`
	Ports              []int             `json:"ports,omitempty"`
	Warnings           []string          `json:"warnings,omitempty"`
}

func Detect(osName string) Runtime {
	if strings.TrimSpace(osName) == "" {
		osName = runtime.GOOS
	}
	status := platform.Check(osName, runtime.GOARCH)
	rt := Runtime{
		OS:             status.OS,
		Arch:           status.Arch,
		Supported:      status.Supported,
		ManagedService: status.ManagedServiceSupport,
		SupportReason:  status.Reason,
		BinaryPaths:    map[string]string{},
		Ports:          []int{21115, 21116, 21117, 21118, 21119},
	}
	switch rt.OS {
	case "linux":
		detectLinux(&rt)
	case "windows":
		detectWindows(&rt)
	case "darwin":
		detectDarwin(&rt)
	}
	return rt
}

func detectLinux(rt *Runtime) {
	if hasCmd("systemctl") {
		rt.ServiceManager = "systemd"
		for _, svc := range []string{"rustdesk-hbbs", "rustdesk-hbbr", "hbbs", "hbbr"} {
			if serviceExistsLinux(svc) {
				rt.ExistingService = true
				if isFile(filepath.Join("/etc/systemd/system", svc+".service")) {
					rt.ServiceDefinitions = append(rt.ServiceDefinitions, filepath.Join("/etc/systemd/system", svc+".service"))
				}
			}
		}
	}
	for _, p := range []string{"/var/lib/rustdesk-server", "/opt/rustdesk-server/data", "/opt/rustdesk/data"} {
		if rt.DataDir == "" && hasDataFiles(p) {
			rt.DataDir = p
		}
	}
	for _, p := range []string{"/opt/rustdesk-server/bin", "/usr/local/bin", "/opt/rustdesk-server", "/opt/rustdesk"} {
		addBinariesFromDir(rt, p)
	}
	for _, p := range []string{"/var/log/rustdesk-server", "/opt/rustdesk-server/logs", "/opt/rustdesk/logs"} {
		if rt.LogDir == "" && hasLogFiles(p) {
			rt.LogDir = p
		}
	}
	if rt.InstallDir == "" {
		for _, p := range []string{"/opt/rustdesk-server", "/opt/rustdesk", "/usr/local/bin"} {
			if isDir(p) {
				rt.InstallDir = p
				break
			}
		}
	}
}

func detectDarwin(rt *Runtime) {
	for _, p := range []string{
		filepath.Join(os.Getenv("HOME"), "Library/Application Support/RustDeskServer"),
		"/usr/local/var/rustdesk-server",
		"/opt/homebrew/var/rustdesk-server",
	} {
		if rt.DataDir == "" && hasDataFiles(p) {
			rt.DataDir = p
		}
	}
	for _, p := range []string{"/usr/local/bin", "/opt/homebrew/bin"} {
		addBinariesFromDir(rt, p)
		if rt.InstallDir == "" && isDir(p) {
			rt.InstallDir = p
		}
	}
}

func detectWindows(rt *Runtime) {
	rt.ServiceManager = detectWindowsServiceManager()
	rt.ExistingService = rt.ServiceManager != ""
	rt.ServiceDefinitions = append(rt.ServiceDefinitions, detectWindowsServiceDefinitions()...)
	for _, p := range append([]string{
		`C:\RustDesk-Server`,
		`C:\rustdesk-server`,
		`C:\Program Files\RustDesk Server`,
	}, detectWindowsCandidates()...) {
		addBinariesFromDir(rt, p)
		if rt.InstallDir == "" && isDir(p) {
			rt.InstallDir = p
		}
		if rt.DataDir == "" {
			for _, d := range []string{p, filepath.Join(p, "data")} {
				if hasDataFiles(d) {
					rt.DataDir = d
					break
				}
			}
		}
		if rt.LogDir == "" {
			for _, d := range []string{filepath.Join(p, "logs"), p} {
				if hasLogFiles(d) {
					rt.LogDir = d
					break
				}
			}
		}
	}
}

func addBinariesFromDir(rt *Runtime, dir string) {
	for _, name := range []string{"hbbs", "hbbr", "hbbs.exe", "hbbr.exe"} {
		path := filepath.Join(dir, name)
		if !isFile(path) {
			continue
		}
		key := strings.TrimSuffix(strings.ToLower(name), ".exe")
		rt.BinaryPaths[key] = path
		if rt.InstallDir == "" {
			rt.InstallDir = filepath.Dir(path)
		}
	}
}

func hasDataFiles(dir string) bool {
	if !isDir(dir) {
		return false
	}
	for _, name := range []string{"id_ed25519", "id_ed25519.pub", "db_v2.sqlite3", "db.sqlite3"} {
		if isFile(filepath.Join(dir, name)) {
			return true
		}
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "db_v2.sqlite3*"))
	return len(matches) > 0
}

func hasLogFiles(dir string) bool {
	if !isDir(dir) {
		return false
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if strings.HasSuffix(name, ".log") || strings.Contains(name, "hbbs") || strings.Contains(name, "hbbr") {
			return true
		}
	}
	return false
}

func PortConflicts(ports []int) []string {
	issues := []string{}
	for _, port := range ports {
		if port > 0 && portListening(port) {
			issues = append(issues, fmt.Sprintf("port %d is already in use", port))
		}
	}
	return issues
}

func serviceExistsLinux(name string) bool {
	if !hasCmd("systemctl") {
		return false
	}
	return exec.Command("systemctl", "status", name).Run() == nil
}

func detectWindowsServiceManager() string {
	if hasCmd("pm2") && len(detectWindowsPM2Candidates()) > 0 {
		return "pm2"
	}
	if hasCmd("powershell") && len(detectWindowsServiceDefinitions()) > 0 {
		return "sc"
	}
	return ""
}

func detectWindowsServiceDefinitions() []string {
	if !hasCmd("powershell") {
		return nil
	}
	script := `$names=@('rustdesk-hbbs','rustdesk-hbbr','rustdesksignal','rustdeskrelay','hbbs','hbbr');
foreach($n in $names){
  $p1="HKLM:\SYSTEM\CurrentControlSet\Services\$n\Parameters"
  if(Test-Path $p1){$p1}
  $p2="HKLM:\SYSTEM\CurrentControlSet\Services\$n"
  if(Test-Path $p2){$p2}
}`
	out, err := exec.Command("powershell", "-NoProfile", "-Command", script).Output()
	if err != nil {
		return nil
	}
	return splitLines(string(out))
}

func detectWindowsCandidates() []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(items []string) {
		for _, item := range items {
			item = strings.TrimSpace(item)
			if item == "" || seen[item] {
				continue
			}
			seen[item] = true
			out = append(out, item)
		}
	}
	add(detectWindowsPM2Candidates())
	add(detectWindowsServiceCandidates())
	add(detectWindowsProcessCandidates())
	return out
}

func detectWindowsPM2Candidates() []string {
	if !hasCmd("pm2") {
		return nil
	}
	out, err := exec.Command("pm2", "jlist").Output()
	if err != nil || len(out) == 0 {
		return nil
	}
	type env struct {
		CWD string `json:"pm_cwd"`
	}
	type item struct {
		Name string `json:"name"`
		Env  env    `json:"pm2_env"`
	}
	var items []item
	if err := json.Unmarshal(out, &items); err != nil {
		return nil
	}
	candidates := []string{}
	for _, item := range items {
		name := strings.ToLower(strings.TrimSpace(item.Name))
		if !strings.Contains(name, "rustdesk") && name != "hbbs" && name != "hbbr" {
			continue
		}
		if item.Env.CWD != "" {
			candidates = append(candidates, item.Env.CWD)
		}
	}
	return candidates
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
      $m=[regex]::Match($expanded,'^"?([^" ]+\.exe)')
      if($m.Success){Split-Path -Parent $m.Groups[1].Value}
    }
  }catch{}
}`
	out, err := exec.Command("powershell", "-NoProfile", "-Command", script).Output()
	if err != nil {
		return nil
	}
	return splitLines(string(out))
}

func detectWindowsProcessCandidates() []string {
	if !hasCmd("powershell") {
		return nil
	}
	script := `Get-Process -Name hbbs,hbbr,rustdesksignal,rustdeskrelay -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Path`
	out, err := exec.Command("powershell", "-NoProfile", "-Command", script).Output()
	if err != nil {
		return nil
	}
	lines := splitLines(string(out))
	candidates := make([]string, 0, len(lines))
	for _, line := range lines {
		candidates = append(candidates, filepath.Dir(line))
	}
	return candidates
}

func splitLines(s string) []string {
	out := []string{}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func portListening(port int) bool {
	if runtime.GOOS == "windows" && hasCmd("powershell") {
		script := fmt.Sprintf(`$x=Get-NetTCPConnection -State Listen -LocalPort %d -ErrorAction SilentlyContinue; if($x){'1'}`, port)
		out, _ := exec.Command("powershell", "-NoProfile", "-Command", script).Output()
		return strings.TrimSpace(string(out)) == "1"
	}
	if hasCmd("lsof") {
		return exec.Command("lsof", "-i", ":"+strconv.Itoa(port)).Run() == nil
	}
	if hasCmd("ss") {
		cmd := exec.Command("sh", "-c", fmt.Sprintf("ss -ltn '( sport = :%d )' | tail -n +2 | grep -q .", port))
		return cmd.Run() == nil
	}
	return false
}

func hasCmd(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func isDir(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

func isFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}
