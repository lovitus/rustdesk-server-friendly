package wizard

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lovitus/rustdesk-server-friendly/internal/guide"
)

type Options struct {
	Output string
}

func Run(in io.Reader, out io.Writer, opts Options) error {
	reader := bufio.NewReader(in)
	cfg := guide.DefaultConfig()

	fmt.Fprintln(out, "RustDesk Server Friendly Wizard")
	fmt.Fprintln(out, "Generate deployment/service/log/migration runbooks with guided prompts.")
	fmt.Fprintln(out)

	mode, err := promptChoice(reader, out, "Choose workflow", []string{"guided-setup", "guided-migration", "custom"}, "guided-setup")
	if err != nil {
		return err
	}

	switch mode {
	case "guided-setup":
		target, err := promptChoice(reader, out, "Target OS", []string{"linux", "windows"}, "linux")
		if err != nil {
			return err
		}
		cfg.Target = target
		cfg.Topic = "all"
		cfg.Host, err = promptText(reader, out, "Public host/IP used by hbbs -r", cfg.Host)
		if err != nil {
			return err
		}

		auto, err := promptYesNo(reader, out, "Auto-detect runtime paths from running services/processes (recommended)?", true)
		if err != nil {
			return err
		}
		if !auto {
			if target == "linux" {
				cfg.LinuxInstallDir, _ = promptText(reader, out, "Linux install dir (fallback)", cfg.LinuxInstallDir)
				cfg.LinuxDataDir, _ = promptText(reader, out, "Linux data dir (fallback)", cfg.LinuxDataDir)
				cfg.LinuxLogDir, _ = promptText(reader, out, "Linux log dir (fallback)", cfg.LinuxLogDir)
			} else {
				cfg.WindowsDir, _ = promptText(reader, out, "Windows root dir (fallback)", cfg.WindowsDir)
			}
		}

	case "guided-migration":
		cfg.Target = "cross"
		cfg.Topic = "migrate"
		var err error
		cfg.MigrationSourceOS, err = promptChoice(reader, out, "Migration source OS", guide.SupportedMigrationOS, cfg.MigrationSourceOS)
		if err != nil {
			return err
		}
		cfg.MigrationTargetOS, err = promptChoice(reader, out, "Migration target OS", guide.SupportedMigrationOS, cfg.MigrationTargetOS)
		if err != nil {
			return err
		}

		auto, err := promptYesNo(reader, out, "Auto-detect source/target data paths from running services (recommended)?", true)
		if err != nil {
			return err
		}
		if !auto {
			if cfg.MigrationSourceOS == "linux" {
				cfg.MigrationSourceLinux, _ = promptText(reader, out, "Source Linux data dir (fallback)", cfg.MigrationSourceLinux)
			} else {
				cfg.MigrationSourceWindows, _ = promptText(reader, out, "Source Windows root dir (fallback)", cfg.MigrationSourceWindows)
			}
			if cfg.MigrationTargetOS == "linux" {
				cfg.MigrationTargetLinux, _ = promptText(reader, out, "Target Linux data dir (fallback)", cfg.MigrationTargetLinux)
			} else {
				cfg.MigrationTargetWindows, _ = promptText(reader, out, "Target Windows root dir (fallback)", cfg.MigrationTargetWindows)
			}
		}

	default:
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
	}

	rendered, err := guide.Render(cfg)
	if err != nil {
		return err
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
		if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(output, []byte(rendered), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(out, "Saved: %s\n", output)
	}

	return nil
}

func defaultFilename(cfg guide.Config) string {
	if cfg.Topic == "migrate" {
		return fmt.Sprintf("rustdesk-%s-to-%s-migration.md", cfg.MigrationSourceOS, cfg.MigrationTargetOS)
	}
	return fmt.Sprintf("rustdesk-%s-%s-guide.md", cfg.Target, cfg.Topic)
}

func promptText(reader *bufio.Reader, out io.Writer, label, def string) (string, error) {
	fmt.Fprintf(out, "%s [%s]: ", label, def)
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
