package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lovitus/rustdesk-server-friendly/internal/backup"
	"github.com/lovitus/rustdesk-server-friendly/internal/guide"
	"github.com/lovitus/rustdesk-server-friendly/internal/wizard"
)

var version = "dev"

func main() {
	if len(os.Args) == 1 {
		if err := wizard.Run(os.Stdin, os.Stdout, wizard.Options{}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	sub := strings.ToLower(os.Args[1])
	switch sub {
	case "guide":
		if err := runGuide(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "apply":
		if err := runApply(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "wizard":
		fs := flag.NewFlagSet("wizard", flag.ExitOnError)
		output := fs.String("output", "", "auto save generated guide to this file")
		_ = fs.Parse(os.Args[2:])
		if err := wizard.Run(os.Stdin, os.Stdout, wizard.Options{Output: *output}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "version", "-v", "--version":
		fmt.Println(version)
	case "help", "-h", "--help":
		printHelp()
	default:
		if err := wizard.Run(os.Stdin, os.Stdout, wizard.Options{}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

func runGuide(args []string) error {
	cfg := guide.DefaultConfig()
	fs := flag.NewFlagSet("guide", flag.ContinueOnError)
	fs.StringVar(&cfg.Target, "target", cfg.Target, "linux|windows|cross")
	fs.StringVar(&cfg.Topic, "topic", cfg.Topic, "deploy|logs|service|migrate|all")
	fs.StringVar(&cfg.Host, "host", cfg.Host, "public host or IP")
	fs.StringVar(&cfg.WindowsDir, "windows-dir", cfg.WindowsDir, "windows root dir")
	fs.StringVar(&cfg.LinuxInstallDir, "linux-install-dir", cfg.LinuxInstallDir, "linux install dir")
	fs.StringVar(&cfg.LinuxDataDir, "linux-data-dir", cfg.LinuxDataDir, "linux data dir")
	fs.StringVar(&cfg.LinuxLogDir, "linux-log-dir", cfg.LinuxLogDir, "linux log dir")
	fs.StringVar(&cfg.MigrationSourceOS, "migration-source", cfg.MigrationSourceOS, "linux|windows")
	fs.StringVar(&cfg.MigrationTargetOS, "migration-target", cfg.MigrationTargetOS, "linux|windows")
	fs.StringVar(&cfg.MigrationSourceWindows, "source-windows-dir", cfg.MigrationSourceWindows, "source windows root dir")
	fs.StringVar(&cfg.MigrationTargetWindows, "target-windows-dir", cfg.MigrationTargetWindows, "target windows root dir")
	fs.StringVar(&cfg.MigrationSourceLinux, "source-linux-data-dir", cfg.MigrationSourceLinux, "source linux data dir")
	fs.StringVar(&cfg.MigrationTargetLinux, "target-linux-data-dir", cfg.MigrationTargetLinux, "target linux data dir")
	output := fs.String("output", "", "write output markdown to this file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	rendered, err := guide.Render(cfg)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*output) == "" {
		fmt.Println(rendered)
		return nil
	}
	abs, err := filepath.Abs(*output)
	if err != nil {
		abs = *output
	}
	if err := os.WriteFile(abs, []byte(rendered), 0o644); err != nil {
		return err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return err
	}
	fmt.Printf("Saved: %s (%d bytes)\n", abs, st.Size())
	return nil
}

func runApply(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("apply requires a subcommand: backup")
	}
	switch strings.ToLower(args[0]) {
	case "backup":
		return runApplyBackup(args[1:])
	default:
		return fmt.Errorf("unknown apply subcommand: %s", args[0])
	}
}

func runApplyBackup(args []string) error {
	fs := flag.NewFlagSet("apply backup", flag.ContinueOnError)
	source := fs.String("source", "", "source os: windows|linux (default auto by current OS)")
	sourceDataDir := fs.String("source-data-dir", "", "source rustdesk data dir")
	output := fs.String("output", "", "archive output path")
	force := fs.Bool("force", false, "overwrite existing output archive")
	if err := fs.Parse(args); err != nil {
		return err
	}

	res, err := backup.Run(backup.Options{
		SourceOS:      *source,
		SourceDataDir: *sourceDataDir,
		Output:        *output,
		Force:         *force,
		Out:           os.Stdout,
	})
	if err != nil {
		return err
	}
	fmt.Printf("[OK] Backup ready: %s\n", res.ArchivePath)
	return nil
}

func printHelp() {
	fmt.Print(`rustdesk-friendly (Go rewrite)

Usage:
  rustdesk-friendly                 # interactive wizard
  rustdesk-friendly wizard [--output FILE]
  rustdesk-friendly guide [flags]
  rustdesk-friendly apply backup [flags]
  rustdesk-friendly version

Guide flags:
  --target linux|windows|cross
  --topic deploy|logs|service|migrate|all
  --host <PUBLIC_HOST_OR_IP>
  --output <file>
  --migration-source linux|windows
  --migration-target linux|windows

Apply backup flags:
  --source windows|linux
  --source-data-dir <dir>
  --output <archive-path>
  --force
`)
}
