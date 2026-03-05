package wizard

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/lovitus/rustdesk-server-friendly/internal/backup"
	"github.com/lovitus/rustdesk-server-friendly/internal/guide"
	"github.com/lovitus/rustdesk-server-friendly/internal/restore"
)

type Options struct {
	Output string
}

func Run(in io.Reader, out io.Writer, opts Options) error {
	reader := bufio.NewReader(in)

	fmt.Fprintln(out, "RustDesk Server Friendly")
	fmt.Fprintln(out, "Choose what to execute:")
	fmt.Fprintln(out)

	action, err := promptChoice(reader, out, "Action", []string{"backup", "import", "generate-guide"}, "backup")
	if err != nil {
		return err
	}

	switch action {
	case "backup":
		return runBackupFlow(reader, out)
	case "import":
		return runImportFlow(reader, out)
	default:
		return runGuideFlow(reader, out, opts)
	}
}

func runBackupFlow(reader *bufio.Reader, out io.Writer) error {
	defaultOS := hostOS()
	sourceOS, err := promptChoice(reader, out, "Source OS", []string{"windows", "linux"}, defaultOS)
	if err != nil {
		return err
	}

	autoDetect, err := promptYesNo(reader, out, "Auto-detect source data directory?", true)
	if err != nil {
		return err
	}
	sourceDir := ""
	if !autoDetect {
		sourceDir, err = promptText(reader, out, "Source data dir", defaultSourceDir(sourceOS))
		if err != nil {
			return err
		}
	}

	defaultOut := defaultBackupOutput(sourceOS)
	output, err := promptText(reader, out, "Backup archive output", defaultOut)
	if err != nil {
		return err
	}
	absOut, _ := filepath.Abs(output)

	force := false
	if st, err := os.Stat(absOut); err == nil && !st.IsDir() {
		force, err = promptYesNo(reader, out, "Output exists. Overwrite?", false)
		if err != nil {
			return err
		}
		if !force {
			return fmt.Errorf("aborted by user: output already exists")
		}
	}

	_, err = backup.Run(backup.Options{
		SourceOS:      sourceOS,
		SourceDataDir: sourceDir,
		Output:        absOut,
		Force:         force,
		Out:           out,
	})
	return err
}

func runImportFlow(reader *bufio.Reader, out io.Writer) error {
	defaultOS := hostOS()
	targetOS, err := promptChoice(reader, out, "Target OS", []string{"windows", "linux"}, defaultOS)
	if err != nil {
		return err
	}

	archive, err := promptText(reader, out, "Backup archive path (.zip/.tgz/.tar.gz)", "")
	if err != nil {
		return err
	}
	archive = strings.TrimSpace(strings.Trim(archive, `"`))
	for {
		if archive == "" {
			archive, _ = promptText(reader, out, "Backup archive path (.zip/.tgz/.tar.gz)", "")
			archive = strings.TrimSpace(strings.Trim(archive, `"`))
			continue
		}
		if st, err := os.Stat(archive); err == nil && !st.IsDir() {
			break
		}
		fmt.Fprintln(out, "File not found. Please input a valid archive file path.")
		archive, _ = promptText(reader, out, "Backup archive path (.zip/.tgz/.tar.gz)", "")
		archive = strings.TrimSpace(strings.Trim(archive, `"`))
	}

	autoDetect, err := promptYesNo(reader, out, "Auto-detect target data directory?", true)
	if err != nil {
		return err
	}
	targetDir := ""
	if !autoDetect {
		targetDir, err = promptText(reader, out, "Target data dir", defaultTargetDir(targetOS))
		if err != nil {
			return err
		}
	}

	force, err := promptYesNo(reader, out, "Allow overwrite when target already has key/db files?", false)
	if err != nil {
		return err
	}

	_, err = restore.Run(restore.Options{
		TargetOS:      targetOS,
		Archive:       archive,
		TargetDataDir: targetDir,
		Force:         force,
		Out:           out,
	})
	return err
}

func runGuideFlow(reader *bufio.Reader, out io.Writer, opts Options) error {
	cfg := guide.DefaultConfig()
	var err error

	cfg.Target, err = promptChoice(reader, out, "Target", guide.SupportedTargets, cfg.Target)
	if err != nil {
		return err
	}
	cfg.Topic, err = promptChoice(reader, out, "Topic", guide.SupportedTopics, cfg.Topic)
	if err != nil {
		return err
	}
	if cfg.Topic == "migrate" {
		cfg.MigrationSourceOS, _ = promptChoice(reader, out, "Migration source OS", guide.SupportedMigrationOS, cfg.MigrationSourceOS)
		cfg.MigrationTargetOS, _ = promptChoice(reader, out, "Migration target OS", guide.SupportedMigrationOS, cfg.MigrationTargetOS)
	}
	if cfg.Topic != "migrate" {
		cfg.Host, _ = promptText(reader, out, "Public host/IP used by hbbs -r", cfg.Host)
	}

	rendered, err := guide.Render(cfg)
	if err != nil {
		return err
	}

	if cfg.Topic == "migrate" {
		fmt.Fprintln(out, "[IMPORTANT] This is a generated guide only. It does NOT execute migration.")
	}
	fmt.Fprint(out, "\n===== Generated Guide =====\n\n")
	fmt.Fprintln(out, rendered)

	output := strings.TrimSpace(opts.Output)
	if output == "" {
		save, err := promptYesNo(reader, out, "Export guide to file?", true)
		if err != nil {
			return err
		}
		if save {
			def := defaultFilename(cfg)
			output, err = promptText(reader, out, "Output file", def)
			if err != nil {
				return err
			}
		}
	}

	if strings.TrimSpace(output) != "" {
		absOut, err := filepath.Abs(output)
		if err != nil {
			absOut = output
		}
		if err := os.MkdirAll(filepath.Dir(absOut), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(absOut, []byte(rendered), 0o644); err != nil {
			return err
		}
		st, err := os.Stat(absOut)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "Saved: %s (%d bytes)\n", absOut, st.Size())
	}
	return nil
}

func hostOS() string {
	if runtime.GOOS == "windows" {
		return "windows"
	}
	return "linux"
}

func defaultSourceDir(osName string) string {
	if osName == "windows" {
		return `C:\RustDesk-Server\data`
	}
	return "/var/lib/rustdesk-server"
}

func defaultTargetDir(osName string) string {
	return defaultSourceDir(osName)
}

func defaultBackupOutput(sourceOS string) string {
	if sourceOS == "windows" {
		return `C:\rustdesk-migration-backup\rustdesk-migration-backup.zip`
	}
	return "/tmp/rustdesk-migration-backup.tgz"
}

func defaultFilename(cfg guide.Config) string {
	if cfg.Topic == "migrate" {
		return fmt.Sprintf("rustdesk-%s-to-%s-migration.md", cfg.MigrationSourceOS, cfg.MigrationTargetOS)
	}
	return fmt.Sprintf("rustdesk-%s-%s-guide.md", cfg.Target, cfg.Topic)
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
		fmt.Fprintln(out, "Please answer y or n.")
	}
}
