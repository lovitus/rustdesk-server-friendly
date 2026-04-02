package doctor

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/lovitus/rustdesk-server-friendly/internal/acceptance"
	"github.com/lovitus/rustdesk-server-friendly/internal/logpolicy"
	"github.com/lovitus/rustdesk-server-friendly/internal/runtimeinfo"
	"github.com/lovitus/rustdesk-server-friendly/internal/service"
	"github.com/lovitus/rustdesk-server-friendly/internal/upstream"
)

type Result struct {
	Checks         []string
	Warnings       []string
	BlockingIssues []string
	Runtime        runtimeinfo.Runtime
	Actions        []string
}

func Run(out io.Writer) Result {
	rt := runtimeinfo.Detect("")
	res := Result{Runtime: rt}
	validateRuntime := rt
	validateServiceNames := []string{}
	validatePorts := []int{}
	if rt.Supported {
		res.Checks = append(res.Checks, fmt.Sprintf("runtime %s/%s is within the support matrix", rt.OS, rt.Arch))
	} else {
		res.BlockingIssues = append(res.BlockingIssues, rt.SupportReason)
	}
	if rt.DataDir != "" {
		res.Checks = append(res.Checks, "data directory detected")
	} else {
		res.Warnings = append(res.Warnings, "data directory was not detected automatically")
	}
	if rt.ServiceManager != "" {
		res.Checks = append(res.Checks, fmt.Sprintf("service manager detected: %s", rt.ServiceManager))
	} else {
		res.Warnings = append(res.Warnings, "service manager was not detected automatically")
	}
	if len(runtimeinfo.PortConflicts(rt.Ports)) > 0 {
		res.Warnings = append(res.Warnings, "standard RustDesk ports are already in use")
	}
	installDir := chooseInstallDir(rt)
	dataDir := chooseDataDir(rt)
	logDir := chooseLogDir(rt)
	preflight := acceptance.Preflight(
		rt,
		[]string{installDir, dataDir, logDir},
		[]string{"rustdesk-hbbs", "rustdesk-hbbr"},
		[]int{21116, 21117},
		true,
	)
	res.Checks = append(res.Checks, preflight.Checks...)
	res.Warnings = append(res.Warnings, preflight.Warnings...)
	res.BlockingIssues = append(res.BlockingIssues, preflight.BlockingIssues...)
	if rt.Supported && len(res.BlockingIssues) == 0 {
		if len(rt.BinaryPaths) == 0 && rt.OS != "darwin" {
			if _, warnings, err := upstream.DownloadAndExtract(rt.OS, rt.Arch, installDir); err != nil {
				res.BlockingIssues = append(res.BlockingIssues, "binary repair failed: "+err.Error())
			} else {
				res.Actions = append(res.Actions, "downloaded missing upstream binaries")
				res.Warnings = append(res.Warnings, warnings...)
			}
		}
		if rt.ManagedService && rt.OS != "darwin" {
			if shouldSkipManagedServiceRepair(rt) {
				res.Warnings = append(res.Warnings, "managed service repair skipped on Linux because existing RustDesk units were detected; diagnose remains non-disruptive")
				validateServiceNames, validatePorts = defaultManagedServiceValidationTargets()
			} else {
				svcRes, err := service.Apply(service.Config{
					OS:          rt.OS,
					ServiceName: "rustdesk",
					DataDir:     dataDir,
					InstallDir:  installDir,
					LogDir:      logDir,
					RelayHost:   "127.0.0.1",
					HBBSPort:    21116,
					HBBRPort:    21117,
				})
				if err != nil {
					res.BlockingIssues = append(res.BlockingIssues, "service repair failed: "+err.Error())
				} else {
					res.Actions = append(res.Actions, "repaired managed service definitions")
					res.Checks = append(res.Checks, svcRes.Checks...)
					res.Warnings = append(res.Warnings, svcRes.Warnings...)
					if svcRes.Manager != "" {
						validateRuntime.ServiceManager = svcRes.Manager
					}
					validateServiceNames = svcRes.ServiceNames
					validatePorts = []int{svcRes.HBBSPort, svcRes.HBBRPort}
					logRes, err := logpolicy.Apply(logpolicy.Config{
						OS:             rt.OS,
						ServiceManager: svcRes.Manager,
						LogDir:         logDir,
						ServiceNames:   svcRes.ServiceNames,
					})
					if err != nil {
						res.BlockingIssues = append(res.BlockingIssues, "log policy repair failed: "+err.Error())
					} else {
						res.Actions = append(res.Actions, "applied log retention policy")
						res.Checks = append(res.Checks, logRes.Checks...)
						res.Warnings = append(res.Warnings, logRes.Warnings...)
					}
				}
			}
		}
		if len(validateServiceNames) > 0 || len(validatePorts) > 0 {
			validate := acceptance.Validate(acceptance.Options{
				Runtime:      validateRuntime,
				InstallDir:   installDir,
				DataDir:      dataDir,
				LogDir:       logDir,
				ServiceNames: validateServiceNames,
				Ports:        validatePorts,
				RequireData:  dataDir != "",
			})
			res.Checks = append(res.Checks, validate.Checks...)
			res.Warnings = append(res.Warnings, validate.Warnings...)
			res.BlockingIssues = append(res.BlockingIssues, validate.BlockingIssues...)
		}
	}
	if out != nil {
		fmt.Fprintf(out, "[OK] Runtime: %s/%s\n", rt.OS, rt.Arch)
		for _, check := range res.Checks {
			fmt.Fprintf(out, "[CHECK] %s\n", check)
		}
		for _, action := range res.Actions {
			fmt.Fprintf(out, "[OK] %s\n", action)
		}
		for _, warning := range res.Warnings {
			fmt.Fprintf(out, "[WARN] %s\n", warning)
		}
		for _, issue := range res.BlockingIssues {
			fmt.Fprintf(out, "[STOP] %s\n", issue)
		}
	}
	return res
}

func shouldSkipManagedServiceRepair(rt runtimeinfo.Runtime) bool {
	return rt.OS == "linux" && rt.ExistingService
}

func defaultManagedServiceValidationTargets() ([]string, []int) {
	return []string{"rustdesk-hbbs", "rustdesk-hbbr"}, []int{21116, 21117}
}

func chooseInstallDir(rt runtimeinfo.Runtime) string {
	if rt.InstallDir != "" {
		return rt.InstallDir
	}
	if env := os.Getenv("RUSTDESK_FRIENDLY_INSTALL_DIR"); env != "" {
		return env
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
	if rt.DataDir != "" {
		return rt.DataDir
	}
	if env := os.Getenv("RUSTDESK_FRIENDLY_DATA_DIR"); env != "" {
		return env
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
	if rt.LogDir != "" {
		return rt.LogDir
	}
	if env := os.Getenv("RUSTDESK_FRIENDLY_LOG_DIR"); env != "" {
		return env
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
