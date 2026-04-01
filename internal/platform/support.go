package platform

import (
	"fmt"
	"runtime"
	"strings"
)

type Capability string

const (
	CapabilityManagedService Capability = "managed_service"
	CapabilityBackup         Capability = "backup"
	CapabilityRestore        Capability = "restore"
)

type SupportStatus struct {
	OS                    string
	Arch                  string
	Supported             bool
	ManagedServiceSupport bool
	Reason                string
}

func NormalizeOS(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "darwin", "macos", "mac":
		return "darwin"
	case "windows", "linux":
		return v
	default:
		return v
	}
}

func NormalizeArch(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "x86_64", "amd64":
		return "amd64"
	case "aarch64", "arm64":
		return "arm64"
	case "armv7l", "armv7":
		return "armv7"
	default:
		return v
	}
}

func Current() SupportStatus {
	return Check(runtime.GOOS, runtime.GOARCH)
}

func Check(osName, arch string) SupportStatus {
	osName = NormalizeOS(osName)
	arch = NormalizeArch(arch)
	status := SupportStatus{OS: osName, Arch: arch}

	switch osName {
	case "linux":
		switch arch {
		case "amd64", "arm64", "armv7":
			status.Supported = true
			status.ManagedServiceSupport = true
		default:
			status.Reason = fmt.Sprintf("linux arch %s is not in the supported production matrix", arch)
		}
	case "windows":
		switch arch {
		case "amd64", "arm64":
			status.Supported = true
			status.ManagedServiceSupport = true
		default:
			status.Reason = fmt.Sprintf("windows arch %s is not in the supported production matrix", arch)
		}
	case "darwin":
		switch arch {
		case "amd64", "arm64":
			status.Supported = true
			status.ManagedServiceSupport = false
			status.Reason = "macOS supports backup, package validation, restore planning, and isolated local verification only"
		default:
			status.Reason = fmt.Sprintf("macOS arch %s is not in the supported production matrix", arch)
		}
	default:
		status.Reason = fmt.Sprintf("unsupported operating system: %s", osName)
	}

	return status
}
