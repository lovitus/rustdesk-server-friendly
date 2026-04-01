package service

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type Config struct {
	OS          string
	ServiceName string
	DataDir     string
	InstallDir  string
	LogDir      string
	RelayHost   string
	VerifyMode  bool
}

type Result struct {
	UnitPaths []string
	Checks    []string
	Warnings  []string
}

func Apply(cfg Config) (Result, error) {
	switch strings.ToLower(cfg.OS) {
	case "linux":
		return applyLinux(cfg)
	case "windows":
		return applyWindows(cfg)
	default:
		return Result{}, nil
	}
}

func applyLinux(cfg Config) (Result, error) {
	unitDir := os.Getenv("RUSTDESK_FRIENDLY_SYSTEMD_DIR")
	if strings.TrimSpace(unitDir) == "" {
		unitDir = "/etc/systemd/system"
	}
	if err := os.MkdirAll(unitDir, 0o755); err != nil {
		return Result{}, err
	}
	suffix := ""
	portHost := cfg.RelayHost
	if portHost == "" {
		portHost = "127.0.0.1"
	}
	if cfg.VerifyMode {
		suffix = "-verify"
	}
	hbbsUnit := filepath.Join(unitDir, cfg.ServiceName+"-hbbs"+suffix+".service")
	hbbrUnit := filepath.Join(unitDir, cfg.ServiceName+"-hbbr"+suffix+".service")
	hbbsContent := fmt.Sprintf(`[Unit]
Description=RustDesk HBBS%s
After=network.target

[Service]
Type=simple
WorkingDirectory=%s
ExecStart=%s -r %s:21117
Restart=always
RestartSec=5
LimitNOFILE=1048576
StandardOutput=append:%s/hbbs%s.log
StandardError=append:%s/hbbs%s.error.log

[Install]
WantedBy=multi-user.target
`, suffix, cfg.DataDir, filepath.Join(cfg.InstallDir, "hbbs"), portHost, cfg.LogDir, suffix, cfg.LogDir, suffix)
	hbbrContent := fmt.Sprintf(`[Unit]
Description=RustDesk HBBR%s
After=network.target

[Service]
Type=simple
WorkingDirectory=%s
ExecStart=%s
Restart=always
RestartSec=5
LimitNOFILE=1048576
StandardOutput=append:%s/hbbr%s.log
StandardError=append:%s/hbbr%s.error.log

[Install]
WantedBy=multi-user.target
`, suffix, cfg.DataDir, filepath.Join(cfg.InstallDir, "hbbr"), cfg.LogDir, suffix, cfg.LogDir, suffix)
	if err := os.WriteFile(hbbsUnit, []byte(hbbsContent), 0o644); err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(hbbrUnit, []byte(hbbrContent), 0o644); err != nil {
		return Result{}, err
	}
	res := Result{UnitPaths: []string{hbbsUnit, hbbrUnit}}
	if os.Getenv("RUSTDESK_FRIENDLY_SKIP_SYSTEMCTL") == "1" || runtime.GOOS != "linux" {
		res.Warnings = append(res.Warnings, "systemctl execution skipped")
		return res, nil
	}
	for _, args := range [][]string{
		{"daemon-reload"},
		{"enable", "--now", filepath.Base(hbbsUnit)},
		{"enable", "--now", filepath.Base(hbbrUnit)},
		{"is-active", strings.TrimSuffix(filepath.Base(hbbsUnit), ".service")},
		{"is-active", strings.TrimSuffix(filepath.Base(hbbrUnit), ".service")},
	} {
		cmd := exec.Command("systemctl", args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return res, fmt.Errorf("systemctl %s failed: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
		}
		res.Checks = append(res.Checks, fmt.Sprintf("systemctl %s", strings.Join(args, " ")))
	}
	return res, nil
}

func applyWindows(cfg Config) (Result, error) {
	planPath := filepath.Join(cfg.DataDir, ".managed-service", "windows-service-plan.json")
	if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
		return Result{}, err
	}
	suffix := ""
	if cfg.VerifyMode {
		suffix = "-verify"
	}
	servicePayload := map[string]string{
		"service_name_hbbs": cfg.ServiceName + "-hbbs" + suffix,
		"service_name_hbbr": cfg.ServiceName + "-hbbr" + suffix,
		"hbbs":              filepath.Join(cfg.InstallDir, "hbbs.exe"),
		"hbbr":              filepath.Join(cfg.InstallDir, "hbbr.exe"),
		"data_dir":          cfg.DataDir,
		"log_dir":           cfg.LogDir,
	}
	data, err := json.MarshalIndent(servicePayload, "", "  ")
	if err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(planPath, data, 0o644); err != nil {
		return Result{}, err
	}
	res := Result{UnitPaths: []string{planPath}}
	if runtime.GOOS != "windows" || os.Getenv("RUSTDESK_FRIENDLY_SKIP_SC") == "1" {
		res.Warnings = append(res.Warnings, "windows service execution skipped")
		return res, nil
	}
	manager := detectWindowsManager()
	res.Checks = append(res.Checks, "windows service manager "+manager)
	switch manager {
	case "nssm":
		return applyWindowsNSSM(cfg, servicePayload, res)
	case "pm2":
		return applyWindowsPM2(cfg, servicePayload, res)
	default:
		return applyWindowsSC(servicePayload, res)
	}
}

func applyWindowsSC(servicePayload map[string]string, res Result) (Result, error) {
	for name, bin := range map[string]string{
		servicePayload["service_name_hbbs"]: servicePayload["hbbs"],
		servicePayload["service_name_hbbr"]: servicePayload["hbbr"],
	} {
		create := exec.Command("sc", "create", name, "binPath=", bin, "start=", "auto")
		if out, err := create.CombinedOutput(); err != nil && !strings.Contains(strings.ToLower(string(out)), "already exists") {
			return res, fmt.Errorf("sc create %s failed: %s", name, strings.TrimSpace(string(out)))
		}
		start := exec.Command("sc", "start", name)
		if out, err := start.CombinedOutput(); err != nil && !strings.Contains(strings.ToLower(string(out)), "already running") {
			return res, fmt.Errorf("sc start %s failed: %s", name, strings.TrimSpace(string(out)))
		}
		res.Checks = append(res.Checks, "sc create/start "+name)
	}
	return res, nil
}

func applyWindowsNSSM(cfg Config, servicePayload map[string]string, res Result) (Result, error) {
	logDir := cfg.LogDir
	for name, bin := range map[string]string{
		servicePayload["service_name_hbbs"]: servicePayload["hbbs"],
		servicePayload["service_name_hbbr"]: servicePayload["hbbr"],
	} {
		if out, err := exec.Command("nssm", "install", name, bin).CombinedOutput(); err != nil && !strings.Contains(strings.ToLower(string(out)), "already exists") {
			return res, fmt.Errorf("nssm install %s failed: %s", name, strings.TrimSpace(string(out)))
		}
		for _, kv := range [][]string{
			{"AppDirectory", cfg.DataDir},
			{"AppStdout", filepath.Join(logDir, name+".log")},
			{"AppStderr", filepath.Join(logDir, name+".error.log")},
		} {
			if out, err := exec.Command("nssm", "set", name, kv[0], kv[1]).CombinedOutput(); err != nil {
				return res, fmt.Errorf("nssm set %s %s failed: %s", name, kv[0], strings.TrimSpace(string(out)))
			}
		}
		if out, err := exec.Command("nssm", "start", name).CombinedOutput(); err != nil && !strings.Contains(strings.ToLower(string(out)), "already running") {
			return res, fmt.Errorf("nssm start %s failed: %s", name, strings.TrimSpace(string(out)))
		}
		res.Checks = append(res.Checks, "nssm install/start "+name)
	}
	return res, nil
}

func applyWindowsPM2(cfg Config, servicePayload map[string]string, res Result) (Result, error) {
	for name, bin := range map[string]string{
		servicePayload["service_name_hbbs"]: servicePayload["hbbs"],
		servicePayload["service_name_hbbr"]: servicePayload["hbbr"],
	} {
		if out, err := exec.Command("pm2", "describe", name).CombinedOutput(); err != nil || !strings.Contains(strings.ToLower(string(out)), "status") {
			args := []string{"start", bin, "--name", name}
			if strings.Contains(strings.ToLower(name), "hbbs") {
				args = append(args, "--", "-r", cfg.RelayHost+":21117")
			}
			if out, err := exec.Command("pm2", args...).CombinedOutput(); err != nil {
				return res, fmt.Errorf("pm2 start %s failed: %s", name, strings.TrimSpace(string(out)))
			}
		}
		if out, err := exec.Command("pm2", "save").CombinedOutput(); err != nil {
			return res, fmt.Errorf("pm2 save failed: %s", strings.TrimSpace(string(out)))
		}
		res.Checks = append(res.Checks, "pm2 start/save "+name)
	}
	return res, nil
}

func detectWindowsManager() string {
	if forced := strings.ToLower(strings.TrimSpace(os.Getenv("RUSTDESK_FRIENDLY_WINDOWS_SERVICE_MANAGER"))); forced != "" {
		return forced
	}
	if hasCmd("nssm") {
		return "nssm"
	}
	if hasCmd("pm2") {
		return "pm2"
	}
	return "sc"
}

func hasCmd(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
