package logpolicy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type Config struct {
	OS             string
	ServiceManager string
	LogDir         string
	ServiceNames   []string
}

type Result struct {
	ArtifactPaths []string
	Checks        []string
	Warnings      []string
}

func Apply(cfg Config) (Result, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.OS)) {
	case "linux":
		return applyLinux(cfg)
	case "windows":
		return applyWindows(cfg)
	default:
		return Result{Warnings: []string{"log policy management is not supported on this runtime"}}, nil
	}
}

func applyLinux(cfg Config) (Result, error) {
	if err := os.MkdirAll(cfg.LogDir, 0o755); err != nil {
		return Result{}, err
	}
	logrotateFile := strings.TrimSpace(os.Getenv("RUSTDESK_FRIENDLY_LOGROTATE_FILE"))
	if logrotateFile == "" {
		logrotateFile = "/etc/logrotate.d/rustdesk-server"
	}
	journaldFile := strings.TrimSpace(os.Getenv("RUSTDESK_FRIENDLY_JOURNALD_FILE"))
	if journaldFile == "" {
		journaldFile = "/etc/systemd/journald.conf.d/rustdesk.conf"
	}
	if err := os.MkdirAll(filepath.Dir(logrotateFile), 0o755); err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(filepath.Dir(journaldFile), 0o755); err != nil {
		return Result{}, err
	}
	logrotateContent := fmt.Sprintf(`%s/*.log {
    daily
    rotate 14
    size 50M
    missingok
    notifempty
    compress
    delaycompress
    copytruncate
}
`, cfg.LogDir)
	journaldContent := `[Journal]
SystemMaxUse=500M
RuntimeMaxUse=200M
MaxRetentionSec=14day
`
	if err := os.WriteFile(logrotateFile, []byte(logrotateContent), 0o644); err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(journaldFile, []byte(journaldContent), 0o644); err != nil {
		return Result{}, err
	}
	res := Result{ArtifactPaths: []string{logrotateFile, journaldFile}}
	res.Checks = append(res.Checks, "linux logrotate policy written")
	res.Checks = append(res.Checks, "linux journald quota policy written")
	if runtime.GOOS != "linux" || os.Getenv("RUSTDESK_FRIENDLY_SKIP_SYSTEMCTL") == "1" {
		res.Warnings = append(res.Warnings, "linux log policy activation skipped on this runtime")
		return res, nil
	}
	if out, err := exec.Command("systemctl", "restart", "systemd-journald").CombinedOutput(); err != nil {
		return res, fmt.Errorf("systemctl restart systemd-journald failed: %s", strings.TrimSpace(string(out)))
	}
	res.Checks = append(res.Checks, "systemd-journald restarted")
	if _, err := exec.LookPath("logrotate"); err == nil {
		if out, err := exec.Command("logrotate", "-f", logrotateFile).CombinedOutput(); err != nil {
			return res, fmt.Errorf("logrotate activation failed: %s", strings.TrimSpace(string(out)))
		}
		res.Checks = append(res.Checks, "logrotate policy activated")
	} else {
		res.Warnings = append(res.Warnings, "logrotate command not found; policy file was written but not forced immediately")
	}
	return res, nil
}

func applyWindows(cfg Config) (Result, error) {
	if err := os.MkdirAll(cfg.LogDir, 0o755); err != nil {
		return Result{}, err
	}
	manager := strings.ToLower(strings.TrimSpace(cfg.ServiceManager))
	if manager == "" {
		manager = "sc"
	}
	res := Result{}
	policyPath := filepath.Join(cfg.LogDir, "rustdesk-friendly-log-policy.json")
	policyData := fmt.Sprintf(`{
  "service_manager": %q,
  "max_size": "50M",
  "retain": 14,
  "compress": true
}
`, manager)
	if err := os.WriteFile(policyPath, []byte(policyData), 0o644); err != nil {
		return Result{}, err
	}
	res.ArtifactPaths = append(res.ArtifactPaths, policyPath)
	res.Checks = append(res.Checks, "windows log policy artifact written")
	if runtime.GOOS != "windows" || os.Getenv("RUSTDESK_FRIENDLY_SKIP_SC") == "1" {
		res.Warnings = append(res.Warnings, "windows log policy activation skipped on this runtime")
		return res, nil
	}
	switch manager {
	case "pm2":
		for _, args := range [][]string{{"install", "pm2-logrotate"}, {"set", "pm2-logrotate:max_size", "50M"}, {"set", "pm2-logrotate:retain", "14"}, {"set", "pm2-logrotate:compress", "true"}, {"save"}} {
			if out, err := exec.Command("pm2", args...).CombinedOutput(); err != nil && !strings.Contains(strings.ToLower(string(out)), "already") {
				return res, fmt.Errorf("pm2 %s failed: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
			}
			res.Checks = append(res.Checks, "pm2 "+strings.Join(args, " "))
		}
	case "nssm":
		for _, name := range cfg.ServiceNames {
			for _, args := range [][]string{{"set", name, "AppRotateFiles", "1"}, {"set", name, "AppRotateOnline", "1"}, {"set", name, "AppRotateBytes", "52428800"}} {
				if out, err := exec.Command("nssm", args...).CombinedOutput(); err != nil {
					return res, fmt.Errorf("nssm %s failed: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
				}
				res.Checks = append(res.Checks, "nssm "+strings.Join(args, " "))
			}
		}
	default:
		res.Warnings = append(res.Warnings, "native sc services do not provide bounded stdout/stderr rotation; install NSSM or pm2 for enforced log rotation")
	}
	return res, nil
}
