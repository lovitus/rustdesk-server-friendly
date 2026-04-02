package acceptance

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/lovitus/rustdesk-server-friendly/internal/runtimeinfo"
)

type Result struct {
	Checks         []string
	Warnings       []string
	BlockingIssues []string
}

type Options struct {
	Runtime               runtimeinfo.Runtime
	InstallDir            string
	DataDir               string
	LogDir                string
	ServiceNames          []string
	Ports                 []int
	RequireData           bool
	AllowServiceConflicts bool
}

func Preflight(rt runtimeinfo.Runtime, dirs []string, serviceNames []string, ports []int, allowServiceConflicts bool) Result {
	res := Result{}
	for _, dir := range dirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		if err := ensureWritable(dir); err != nil {
			res.BlockingIssues = append(res.BlockingIssues, fmt.Sprintf("directory is not writable: %s (%v)", dir, err))
			continue
		}
		res.Checks = append(res.Checks, "directory writable: "+dir)
		if free, err := freeSpaceBytes(dir); err == nil {
			if free < 128*1024*1024 {
				res.BlockingIssues = append(res.BlockingIssues, fmt.Sprintf("free space below 128 MiB at %s", dir))
			} else {
				res.Checks = append(res.Checks, fmt.Sprintf("free space check passed for %s", dir))
			}
		} else {
			res.Warnings = append(res.Warnings, fmt.Sprintf("free space check unavailable for %s: %v", dir, err))
		}
	}
	for _, name := range serviceNames {
		if serviceNameConflict(rt.OS, name) {
			if allowServiceConflictWarning(rt.OS, allowServiceConflicts) {
				res.Warnings = append(res.Warnings, "service already exists and will be taken over: "+name)
			} else {
				res.BlockingIssues = append(res.BlockingIssues, "service already exists: "+name)
			}
		}
	}
	for _, issue := range runtimeinfo.PortConflicts(ports) {
		res.Warnings = append(res.Warnings, issue)
	}
	return res
}

func allowServiceConflictWarning(osName string, allowServiceConflicts bool) bool {
	return allowServiceConflicts && strings.EqualFold(strings.TrimSpace(osName), "linux")
}

func Validate(opts Options) Result {
	res := Result{}
	for _, binary := range expectedBinaries(opts.Runtime.OS, opts.InstallDir) {
		if st, err := os.Stat(binary); err != nil || st.IsDir() {
			res.BlockingIssues = append(res.BlockingIssues, "missing binary: "+binary)
		} else {
			res.Checks = append(res.Checks, "binary present: "+binary)
		}
	}
	if opts.RequireData {
		for _, name := range []string{"id_ed25519", "id_ed25519.pub"} {
			path := filepath.Join(opts.DataDir, name)
			if st, err := os.Stat(path); err != nil || st.IsDir() {
				res.BlockingIssues = append(res.BlockingIssues, "missing required data file: "+path)
			} else {
				res.Checks = append(res.Checks, "required data file present: "+path)
			}
		}
	}
	if strings.TrimSpace(opts.LogDir) != "" {
		if st, err := os.Stat(opts.LogDir); err != nil || !st.IsDir() {
			res.BlockingIssues = append(res.BlockingIssues, "log dir missing: "+opts.LogDir)
		} else {
			res.Checks = append(res.Checks, "log dir present: "+opts.LogDir)
		}
	}
	if runtime.GOOS == opts.Runtime.OS {
		serviceChecks, serviceWarnings, serviceIssues := validateServices(opts.Runtime.OS, opts.Runtime.ServiceManager, opts.ServiceNames)
		res.Checks = append(res.Checks, serviceChecks...)
		res.Warnings = append(res.Warnings, serviceWarnings...)
		res.BlockingIssues = append(res.BlockingIssues, serviceIssues...)
		for _, port := range opts.Ports {
			if port <= 0 {
				continue
			}
			if len(runtimeinfo.PortConflicts([]int{port})) == 0 {
				res.BlockingIssues = append(res.BlockingIssues, fmt.Sprintf("expected listening port not detected: %d", port))
			} else {
				res.Checks = append(res.Checks, fmt.Sprintf("port listening: %d", port))
			}
		}
	} else {
		res.Warnings = append(res.Warnings, "runtime validation skipped because target OS differs from the current host")
	}
	return res
}

func expectedBinaries(osName, installDir string) []string {
	if strings.TrimSpace(installDir) == "" {
		return nil
	}
	if osName == "windows" {
		return []string{filepath.Join(installDir, "hbbs.exe"), filepath.Join(installDir, "hbbr.exe")}
	}
	return []string{filepath.Join(installDir, "hbbs"), filepath.Join(installDir, "hbbr")}
}

func ensureWritable(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	probe := filepath.Join(dir, ".rustdesk-friendly-write-check")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		return err
	}
	return os.Remove(probe)
}

func freeSpaceBytes(dir string) (uint64, error) {
	if runtime.GOOS == "windows" {
		root := filepath.VolumeName(dir)
		if root == "" {
			root = dir
		}
		script := fmt.Sprintf(`$p=Get-PSDrive -Name '%s'; if($p){$p.Free}`, strings.TrimSuffix(root, ":"))
		out, err := exec.Command("powershell", "-NoProfile", "-Command", script).Output()
		if err != nil {
			return 0, err
		}
		value := strings.TrimSpace(string(out))
		if value == "" {
			return 0, fmt.Errorf("empty powershell free-space result")
		}
		var free uint64
		_, err = fmt.Sscanf(value, "%d", &free)
		return free, err
	}
	out, err := exec.Command("df", "-Pk", dir).Output()
	if err != nil {
		return 0, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return 0, fmt.Errorf("unexpected df output")
	}
	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) < 4 {
		return 0, fmt.Errorf("unexpected df output fields")
	}
	var freeKB uint64
	_, err = fmt.Sscanf(fields[3], "%d", &freeKB)
	if err != nil {
		return 0, err
	}
	return freeKB * 1024, nil
}

func serviceNameConflict(osName, name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	switch osName {
	case "linux":
		if _, err := exec.LookPath("systemctl"); err != nil {
			return false
		}
		out, err := exec.Command("systemctl", "show", name, "--property=LoadState", "--value").Output()
		if err != nil {
			return false
		}
		return linuxServiceLoadStateExists(string(out))
	case "windows":
		if _, err := exec.LookPath("sc"); err != nil {
			return false
		}
		return exec.Command("sc", "query", name).Run() == nil
	default:
		return false
	}
}

func linuxServiceLoadStateExists(v string) bool {
	state := strings.ToLower(strings.TrimSpace(v))
	return state != "" && state != "not-found" && state != "error"
}

func validateServices(osName, manager string, names []string) ([]string, []string, []string) {
	checks := []string{}
	warnings := []string{}
	issues := []string{}
	if len(names) == 0 {
		return checks, warnings, issues
	}
	switch osName {
	case "linux":
		if os.Getenv("RUSTDESK_FRIENDLY_SKIP_SYSTEMCTL") == "1" {
			warnings = append(warnings, "systemctl validation skipped by RUSTDESK_FRIENDLY_SKIP_SYSTEMCTL")
			return checks, warnings, issues
		}
		for _, name := range names {
			out, err := exec.Command("systemctl", "is-active", name).CombinedOutput()
			if err != nil || !strings.Contains(strings.TrimSpace(string(out)), "active") {
				issues = append(issues, "service not active: "+name)
				continue
			}
			checks = append(checks, "service active: "+name)
		}
	case "windows":
		if os.Getenv("RUSTDESK_FRIENDLY_SKIP_SC") == "1" {
			warnings = append(warnings, "windows service validation skipped by RUSTDESK_FRIENDLY_SKIP_SC")
			return checks, warnings, issues
		}
		if strings.EqualFold(manager, "pm2") {
			for _, name := range names {
				out, err := exec.Command("pm2", "describe", name).CombinedOutput()
				if err != nil || !strings.Contains(strings.ToLower(string(out)), "status") {
					issues = append(issues, "pm2 process not healthy: "+name)
					continue
				}
				checks = append(checks, "pm2 process healthy: "+name)
			}
			return checks, warnings, issues
		}
		for _, name := range names {
			out, err := exec.Command("sc", "query", name).CombinedOutput()
			lower := strings.ToLower(string(out))
			if err != nil || (!strings.Contains(lower, "running") && !strings.Contains(lower, "state")) {
				issues = append(issues, "service not running: "+name)
				continue
			}
			checks = append(checks, "service query returned running state: "+name)
		}
	}
	return checks, warnings, issues
}
