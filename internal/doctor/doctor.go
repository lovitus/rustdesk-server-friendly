package doctor

import (
	"fmt"
	"io"

	"github.com/lovitus/rustdesk-server-friendly/internal/runtimeinfo"
)

type Result struct {
	Checks         []string
	Warnings       []string
	BlockingIssues []string
	Runtime        runtimeinfo.Runtime
}

func Run(out io.Writer) Result {
	rt := runtimeinfo.Detect("")
	res := Result{Runtime: rt}
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
	if out != nil {
		fmt.Fprintf(out, "[OK] Runtime: %s/%s\n", rt.OS, rt.Arch)
		for _, check := range res.Checks {
			fmt.Fprintf(out, "[CHECK] %s\n", check)
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
