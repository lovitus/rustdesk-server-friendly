package wizard

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/lovitus/rustdesk-server-friendly/internal/backup"
	"github.com/lovitus/rustdesk-server-friendly/internal/doctor"
	"github.com/lovitus/rustdesk-server-friendly/internal/guide"
	"github.com/lovitus/rustdesk-server-friendly/internal/install"
	"github.com/lovitus/rustdesk-server-friendly/internal/restore"
	"github.com/lovitus/rustdesk-server-friendly/internal/runtimeinfo"
)

type Options struct {
	Output string
}

func Run(in io.Reader, out io.Writer, opts Options) error {
	reader := bufio.NewReader(in)
	fmt.Fprintln(out, "RustDesk Friendly")
	fmt.Fprintln(out, "Choose what to do:")
	fmt.Fprintln(out)

	action, err := promptChoice(reader, out, "Action", []string{"new-service", "backup-migrate", "restore-service", "diagnose-repair", "advanced-mode"}, "backup-migrate")
	if err != nil {
		return err
	}

	switch action {
	case "new-service":
		return runInstallFlow(reader, out)
	case "backup-migrate":
		return runBackupFlow(reader, out)
	case "restore-service":
		return runRestoreFlow(reader, out)
	case "diagnose-repair":
		doctor.Run(out)
		return nil
	default:
		return runAdvancedFlow(reader, out, opts)
	}
}

func runInstallFlow(reader *bufio.Reader, out io.Writer) error {
	rt := runtimeinfo.Detect("")
	confirmed, err := requireTripleConfirmationIfNeeded(reader, out, rt.ExistingService || len(runtimeinfo.PortConflicts(rt.Ports)) > 0 || rt.DataDir != "")
	if err != nil {
		return err
	}
	_, err = install.Run(install.Options{
		TargetOS:        hostOS(),
		TripleConfirmed: confirmed,
		Out:             out,
	})
	return err
}

func runBackupFlow(reader *bufio.Reader, out io.Writer) error {
	sourceOS, err := promptChoice(reader, out, "Source OS", []string{"windows", "linux", "darwin"}, hostOS())
	if err != nil {
		return err
	}
	output, err := promptText(reader, out, "Backup archive output", defaultBackupOutput(sourceOS))
	if err != nil {
		return err
	}
	force := false
	absOut, _ := filepath.Abs(output)
	if st, err := os.Stat(absOut); err == nil && !st.IsDir() {
		force, err = promptYesNo(reader, out, "Output exists. Overwrite?", false)
		if err != nil {
			return err
		}
		if !force {
			return errors.New("aborted by user")
		}
	}
	backupRes, err := backup.Run(backup.Options{
		SourceOS: sourceOS,
		Output:   absOut,
		Force:    force,
		Out:      out,
	})
	if err != nil {
		return err
	}
	if sourceOS != hostOS() {
		fmt.Fprintln(out, "[WARN] Immediate live-restore verification is only available on the current local host runtime.")
		return nil
	}
	runVerify, err := promptYesNo(reader, out, "Run isolated live-restore verification now?", true)
	if err != nil {
		return err
	}
	if !runVerify {
		return nil
	}
	rt := runtimeinfo.Detect(sourceOS)
	confirmed, err := requireTripleConfirmationIfNeeded(reader, out, rt.ExistingService || rt.DataDir != "")
	if err != nil {
		return err
	}
	res, err := restore.Run(restore.Options{
		TargetOS:        sourceOS,
		Archive:         backupRes.ArchivePath,
		Force:           true,
		LiveVerify:      true,
		TripleConfirmed: confirmed,
		Out:             out,
	})
	if err != nil {
		return err
	}
	ok, err := promptYesNo(reader, out, "After client-side testing, did the isolated verification restore succeed without affecting production clients?", false)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintln(out, "[WARN] Live restore verification was not confirmed. The archive remains at restorable_verified.")
		return nil
	}
	return restore.ConfirmLiveRestoreVerified(backupRes.ArchivePath, res.IsolatedValidationDataDir)
}

func runRestoreFlow(reader *bufio.Reader, out io.Writer) error {
	targetOS, err := promptChoice(reader, out, "Target OS", []string{"windows", "linux", "darwin"}, hostOS())
	if err != nil {
		return err
	}
	archivePath, err := promptExistingFile(reader, out, "Backup archive path")
	if err != nil {
		return err
	}
	liveVerify, err := promptYesNo(reader, out, "Run isolated live-restore verification after restore?", true)
	if err != nil {
		return err
	}
	rt := runtimeinfo.Detect(targetOS)
	confirmed, err := requireTripleConfirmationIfNeeded(reader, out, rt.ExistingService || rt.DataDir != "")
	if err != nil {
		return err
	}
	res, err := restore.Run(restore.Options{
		TargetOS:        targetOS,
		Archive:         archivePath,
		Force:           true,
		LiveVerify:      liveVerify,
		TripleConfirmed: confirmed,
		Out:             out,
	})
	if err != nil {
		return err
	}
	if liveVerify && res.IsolatedValidationDataDir != "" {
		ok, err := promptYesNo(reader, out, "Did the isolated restore validate successfully and leave clients unaffected?", false)
		if err != nil {
			return err
		}
		if ok {
			return restore.ConfirmLiveRestoreVerified(archivePath, res.IsolatedValidationDataDir)
		}
	}
	return nil
}

func runAdvancedFlow(reader *bufio.Reader, out io.Writer, opts Options) error {
	action, err := promptChoice(reader, out, "Advanced action", []string{"generate-guide", "run-guide-topic"}, "generate-guide")
	if err != nil {
		return err
	}
	cfg := guide.DefaultConfig()
	if action == "run-guide-topic" {
		cfg.Target, err = promptChoice(reader, out, "Target", guide.SupportedTargets, cfg.Target)
		if err != nil {
			return err
		}
		cfg.Topic, err = promptChoice(reader, out, "Topic", guide.SupportedTopics, cfg.Topic)
		if err != nil {
			return err
		}
	}
	rendered, err := guide.Render(cfg)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, rendered)
	output := strings.TrimSpace(opts.Output)
	if output == "" {
		save, err := promptYesNo(reader, out, "Export guide to file?", true)
		if err != nil {
			return err
		}
		if save {
			output, err = promptText(reader, out, "Output file", defaultFilename(cfg))
			if err != nil {
				return err
			}
		}
	}
	if strings.TrimSpace(output) != "" {
		output, _ = filepath.Abs(output)
		if err := os.WriteFile(output, []byte(rendered), 0o644); err != nil {
			return err
		}
		st, err := os.Stat(output)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "Saved: %s (%d bytes)\n", output, st.Size())
	}
	return nil
}

func requireTripleConfirmationIfNeeded(reader *bufio.Reader, out io.Writer, needed bool) (bool, error) {
	if !needed {
		return true, nil
	}
	fmt.Fprintln(out, "[WARN] Existing RustDesk service, data, or ports were detected.")
	asks := []string{
		"Confirm that you understand existing RustDesk service or data was detected",
		"Confirm that you allow the program to take over and modify the target side",
		"Confirm that you understand rollback is automatic but manual intervention may still be required",
	}
	for _, ask := range asks {
		ok, err := promptYesNo(reader, out, ask, false)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, errors.New("aborted: triple confirmation not completed")
		}
	}
	return true, nil
}

func hostOS() string {
	switch runtime.GOOS {
	case "windows", "darwin":
		return runtime.GOOS
	default:
		return "linux"
	}
}

func defaultBackupOutput(sourceOS string) string {
	switch sourceOS {
	case "windows":
		return `C:\rustdesk-migration-backup\rustdesk-lifecycle-backup.zip`
	default:
		return "/tmp/rustdesk-lifecycle-backup.tgz"
	}
}

func defaultFilename(cfg guide.Config) string {
	return fmt.Sprintf("rustdesk-%s-%s-guide.md", cfg.Target, cfg.Topic)
}

func promptExistingFile(reader *bufio.Reader, out io.Writer, label string) (string, error) {
	for {
		path, err := promptText(reader, out, label, "")
		if err != nil {
			return "", err
		}
		path = strings.TrimSpace(strings.Trim(path, `"`))
		if path == "" {
			fmt.Fprintln(out, "Please enter a file path.")
			continue
		}
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			return path, nil
		}
		fmt.Fprintln(out, "File not found. Try again.")
	}
}

func promptText(reader *bufio.Reader, out io.Writer, label, def string) (string, error) {
	if def == "" {
		fmt.Fprintf(out, "%s: ", label)
	} else {
		fmt.Fprintf(out, "%s [%s]: ", label, def)
	}
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def, nil
	}
	return line, nil
}

func promptChoice(reader *bufio.Reader, out io.Writer, label string, options []string, def string) (string, error) {
	fmt.Fprintln(out, label)
	for i, opt := range options {
		mark := ""
		if opt == def {
			mark = " (default)"
		}
		fmt.Fprintf(out, "  %d. %s%s\n", i+1, opt, mark)
	}
	for {
		fmt.Fprintf(out, "Select 1-%d or value [%s]: ", len(options), def)
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", err
		}
		line = strings.ToLower(strings.TrimSpace(line))
		if line == "" {
			return def, nil
		}
		for _, opt := range options {
			if line == opt {
				return opt, nil
			}
		}
		if n, err := strconv.Atoi(line); err == nil && n >= 1 && n <= len(options) {
			return options[n-1], nil
		}
		fmt.Fprintln(out, "Invalid selection. Try again.")
	}
}

func promptYesNo(reader *bufio.Reader, out io.Writer, label string, defaultYes bool) (bool, error) {
	suffix := "[Y/n]"
	if !defaultYes {
		suffix = "[y/N]"
	}
	for {
		fmt.Fprintf(out, "%s %s: ", label, suffix)
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return false, err
		}
		line = strings.ToLower(strings.TrimSpace(line))
		if line == "" {
			return defaultYes, nil
		}
		if line == "y" || line == "yes" {
			return true, nil
		}
		if line == "n" || line == "no" {
			return false, nil
		}
		fmt.Fprintln(out, "Please answer yes or no.")
	}
}
