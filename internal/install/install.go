package install

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/lovitus/rustdesk-server-friendly/internal/acceptance"
	"github.com/lovitus/rustdesk-server-friendly/internal/logpolicy"
	"github.com/lovitus/rustdesk-server-friendly/internal/platform"
	"github.com/lovitus/rustdesk-server-friendly/internal/runtimeinfo"
	"github.com/lovitus/rustdesk-server-friendly/internal/service"
	"github.com/lovitus/rustdesk-server-friendly/internal/upstream"
)

type Options struct {
	TargetOS        string
	TripleConfirmed bool
	Out             io.Writer
}

type Result struct {
	Checks           []string
	Warnings         []string
	BlockingIssues   []string
	DetectedRuntime  runtimeinfo.Runtime
	ServiceManager   string
	InstallDir       string
	DataDir          string
	LogDir           string
	ActionsPerformed []string
}

func Run(opts Options) (Result, error) {
	targetOS := normalizeOS(opts.TargetOS)
	rt := runtimeinfo.Detect(targetOS)
	result := Result{
		DetectedRuntime: rt,
		ServiceManager:  rt.ServiceManager,
	}
	if !rt.Supported {
		return result, fmt.Errorf("unsupported runtime: %s/%s: %s", rt.OS, rt.Arch, rt.SupportReason)
	}

	if (rt.ExistingService || len(runtimeinfo.PortConflicts(rt.Ports)) > 0 || hasExistingData(rt.DataDir)) && !opts.TripleConfirmed {
		result.BlockingIssues = append(result.BlockingIssues, "existing service, data, or ports detected; triple confirmation is required")
		return result, fmt.Errorf("%s", result.BlockingIssues[0])
	}

	result.InstallDir = chooseInstallDir(rt)
	result.DataDir = chooseDataDir(rt)
	result.LogDir = chooseLogDir(rt)
	preflight := acceptance.Preflight(rt, []string{result.InstallDir, result.DataDir, result.LogDir}, []string{"rustdesk-hbbs", "rustdesk-hbbr"}, []int{21116, 21117})
	result.Checks = append(result.Checks, preflight.Checks...)
	result.Warnings = append(result.Warnings, preflight.Warnings...)
	if len(preflight.BlockingIssues) > 0 {
		result.BlockingIssues = append(result.BlockingIssues, preflight.BlockingIssues...)
		return result, fmt.Errorf("%s", result.BlockingIssues[0])
	}
	for _, dir := range []string{result.InstallDir, result.DataDir, result.LogDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return result, err
		}
	}
	result.Checks = append(result.Checks, "install directories prepared")

	if err := writeRuntimePlan(result.InstallDir, rt); err != nil {
		return result, err
	}
	result.ActionsPerformed = append(result.ActionsPerformed, "wrote runtime installation plan")

	if len(rt.BinaryPaths) == 0 && rt.OS != "darwin" {
		extracted, warnings, err := upstream.DownloadAndExtract(rt.OS, rt.Arch, result.InstallDir)
		if err != nil {
			return result, err
		}
		result.ActionsPerformed = append(result.ActionsPerformed, "downloaded upstream binaries")
		result.Warnings = append(result.Warnings, warnings...)
		for _, p := range extracted {
			result.Checks = append(result.Checks, "installed binary "+filepath.Base(p))
		}
	} else if rt.OS == "darwin" {
		result.Warnings = append(result.Warnings, "macOS local service hosting is not managed; binaries are not auto-installed")
	}

	if rt.ManagedService {
		svcResult, err := service.Apply(service.Config{
			OS:          rt.OS,
			ServiceName: "rustdesk",
			DataDir:     result.DataDir,
			InstallDir:  result.InstallDir,
			LogDir:      result.LogDir,
			RelayHost:   "127.0.0.1",
			HBBSPort:    21116,
			HBBRPort:    21117,
		})
		if err != nil {
			return result, err
		}
		result.ActionsPerformed = append(result.ActionsPerformed, "registered managed services")
		result.Checks = append(result.Checks, "managed service supported on target runtime")
		result.Checks = append(result.Checks, svcResult.Checks...)
		result.Warnings = append(result.Warnings, svcResult.Warnings...)
		logResult, err := logpolicy.Apply(logpolicy.Config{
			OS:             rt.OS,
			ServiceManager: svcResult.Manager,
			LogDir:         result.LogDir,
			ServiceNames:   svcResult.ServiceNames,
		})
		if err != nil {
			return result, err
		}
		result.ActionsPerformed = append(result.ActionsPerformed, "applied log retention policy")
		result.Checks = append(result.Checks, logResult.Checks...)
		result.Warnings = append(result.Warnings, logResult.Warnings...)
		validateRuntime := rt
		if svcResult.Manager != "" {
			validateRuntime.ServiceManager = svcResult.Manager
		}
		accept := acceptance.Validate(acceptance.Options{
			Runtime:      validateRuntime,
			InstallDir:   result.InstallDir,
			DataDir:      result.DataDir,
			LogDir:       result.LogDir,
			ServiceNames: svcResult.ServiceNames,
			Ports:        []int{svcResult.HBBSPort, svcResult.HBBRPort},
			RequireData:  false,
		})
		result.Checks = append(result.Checks, accept.Checks...)
		result.Warnings = append(result.Warnings, accept.Warnings...)
		if len(accept.BlockingIssues) > 0 {
			result.BlockingIssues = append(result.BlockingIssues, accept.BlockingIssues...)
			return result, fmt.Errorf("%s", result.BlockingIssues[0])
		}
	} else {
		result.Warnings = append(result.Warnings, "managed service is not supported on this runtime; only local validation is available")
	}
	logResult(opts.Out, result)
	return result, nil
}

func writeRuntimePlan(installDir string, rt runtimeinfo.Runtime) error {
	payload := map[string]string{
		"runtime":         fmt.Sprintf("%s/%s", rt.OS, rt.Arch),
		"service_manager": rt.ServiceManager,
		"binary_strategy": "download-or-reuse",
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(installDir, "rustdesk-friendly-install.plan.json"), data, 0o644)
}

func chooseInstallDir(rt runtimeinfo.Runtime) string {
	if env := strings.TrimSpace(os.Getenv("RUSTDESK_FRIENDLY_INSTALL_DIR")); env != "" {
		return env
	}
	if rt.InstallDir != "" {
		return rt.InstallDir
	}
	switch rt.OS {
	case "windows":
		return `C:\RustDesk-Server`
	case "darwin":
		return "/usr/local/bin"
	default:
		return "/opt/rustdesk-server"
	}
}

func chooseDataDir(rt runtimeinfo.Runtime) string {
	if env := strings.TrimSpace(os.Getenv("RUSTDESK_FRIENDLY_DATA_DIR")); env != "" {
		return env
	}
	if rt.DataDir != "" {
		return rt.DataDir
	}
	switch rt.OS {
	case "windows":
		return `C:\RustDesk-Server\data`
	case "darwin":
		return filepath.Join(os.Getenv("HOME"), "Library/Application Support/RustDeskServer")
	default:
		return "/var/lib/rustdesk-server"
	}
}

func chooseLogDir(rt runtimeinfo.Runtime) string {
	if env := strings.TrimSpace(os.Getenv("RUSTDESK_FRIENDLY_LOG_DIR")); env != "" {
		return env
	}
	if rt.LogDir != "" {
		return rt.LogDir
	}
	switch rt.OS {
	case "windows":
		return `C:\RustDesk-Server\logs`
	case "darwin":
		return filepath.Join(os.Getenv("HOME"), "Library/Logs/RustDeskServer")
	default:
		return "/var/log/rustdesk-server"
	}
}

func hasExistingData(dir string) bool {
	if strings.TrimSpace(dir) == "" {
		return false
	}
	for _, name := range []string{"id_ed25519", "id_ed25519.pub", "db_v2.sqlite3"} {
		if st, err := os.Stat(filepath.Join(dir, name)); err == nil && !st.IsDir() {
			return true
		}
	}
	return false
}

func normalizeOS(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return platform.NormalizeOS(runtime.GOOS)
	}
	return platform.NormalizeOS(v)
}

func logResult(out io.Writer, result Result) {
	if out == nil {
		return
	}
	fmt.Fprintf(out, "[OK] Runtime: %s/%s\n", result.DetectedRuntime.OS, result.DetectedRuntime.Arch)
	fmt.Fprintf(out, "[OK] Install dir: %s\n", result.InstallDir)
	fmt.Fprintf(out, "[OK] Data dir: %s\n", result.DataDir)
	fmt.Fprintf(out, "[OK] Log dir: %s\n", result.LogDir)
	for _, check := range result.Checks {
		fmt.Fprintf(out, "[CHECK] %s\n", check)
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(out, "[WARN] %s\n", warning)
	}
}
