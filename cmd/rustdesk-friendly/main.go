package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lovitus/rustdesk-server-friendly/internal/backup"
	"github.com/lovitus/rustdesk-server-friendly/internal/doctor"
	"github.com/lovitus/rustdesk-server-friendly/internal/guide"
	"github.com/lovitus/rustdesk-server-friendly/internal/install"
	"github.com/lovitus/rustdesk-server-friendly/internal/restore"
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
	case "new-service":
		if err := runNewService(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "diagnose":
		doctor.Run(os.Stdout)
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
		return fmt.Errorf("apply requires a subcommand: backup|import|confirm-live-verify|new-service|diagnose")
	}
	switch strings.ToLower(args[0]) {
	case "backup":
		return runApplyBackup(args[1:])
	case "import":
		return runApplyImport(args[1:])
	case "confirm-live-verify":
		return runConfirmLiveVerify(args[1:])
	case "new-service":
		return runNewService(args[1:])
	case "diagnose":
		doctor.Run(os.Stdout)
		return nil
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
	liveVerify := fs.Bool("live-verify", false, "immediately run isolated live-restore verification on the local host")
	tripleConfirmed := fs.Bool("triple-confirmed", false, "acknowledge high-risk verification changes on the local host")
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
	if *liveVerify {
		restoreRes, err := restore.Run(restore.Options{
			TargetOS:        *source,
			Archive:         res.ArchivePath,
			Force:           true,
			LiveVerify:      true,
			TripleConfirmed: *tripleConfirmed,
			Out:             os.Stdout,
		})
		if err != nil {
			return err
		}
		fmt.Printf("[OK] Isolated live verification prepared in: %s\n", restoreRes.IsolatedValidationDataDir)
	}
	return nil
}

func runApplyImport(args []string) error {
	fs := flag.NewFlagSet("apply import", flag.ContinueOnError)
	target := fs.String("target", "", "target os: windows|linux (default auto by current OS)")
	archive := fs.String("archive", "", "backup archive path (.zip/.tgz/.tar.gz)")
	targetDataDir := fs.String("target-data-dir", "", "target rustdesk data dir")
	force := fs.Bool("force", false, "overwrite existing migration files in target dir")
	validateOnly := fs.Bool("validate-only", false, "validate archive and restore plan without writing target data")
	liveVerify := fs.Bool("live-verify", false, "restore to isolated validation directory and service plan")
	userConfirmedLive := fs.Bool("user-confirmed-live", false, "mark isolated live restore as verified")
	tripleConfirmed := fs.Bool("triple-confirmed", false, "confirm high-risk overwrite flow was acknowledged")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*archive) == "" {
		return fmt.Errorf("--archive is required")
	}
	if *userConfirmedLive {
		return fmt.Errorf("--user-confirmed-live is no longer accepted here; use apply confirm-live-verify --archive <backup-archive> --verification-dir <dir>")
	}

	res, err := restore.Run(restore.Options{
		TargetOS:        *target,
		Archive:         *archive,
		TargetDataDir:   *targetDataDir,
		Force:           *force,
		ValidateOnly:    *validateOnly,
		LiveVerify:      *liveVerify,
		TripleConfirmed: *tripleConfirmed,
		Out:             os.Stdout,
	})
	if err != nil {
		return err
	}
	fmt.Printf("[OK] Import ready in: %s (%d files)\n", res.TargetDataDir, len(res.RestoredFiles))
	return nil
}

func runNewService(args []string) error {
	fs := flag.NewFlagSet("new-service", flag.ContinueOnError)
	target := fs.String("target", "", "target os: windows|linux|darwin")
	tripleConfirmed := fs.Bool("triple-confirmed", false, "acknowledge takeover when existing service/data is present")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_, err := install.Run(install.Options{
		TargetOS:        *target,
		TripleConfirmed: *tripleConfirmed,
		Out:             os.Stdout,
	})
	return err
}

func runConfirmLiveVerify(args []string) error {
	fs := flag.NewFlagSet("confirm-live-verify", flag.ContinueOnError)
	archive := fs.String("archive", "", "backup archive path (.zip/.tgz/.tar.gz)")
	verificationDir := fs.String("verification-dir", "", "isolated verification directory containing live verify state")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*archive) == "" {
		return fmt.Errorf("--archive is required")
	}
	if err := restore.ConfirmLiveRestoreVerified(*archive, *verificationDir); err != nil {
		return err
	}
	fmt.Printf("[OK] Archive marked as live_restore_verified: %s\n", *archive)
	return nil
}

func printHelp() {
	fmt.Print(`rustdesk-friendly (Go rewrite)

Usage:
  rustdesk-friendly                 # interactive wizard
  rustdesk-friendly new-service
  rustdesk-friendly diagnose
  rustdesk-friendly wizard [--output FILE]
  rustdesk-friendly guide [flags]
  rustdesk-friendly apply backup [flags]
  rustdesk-friendly apply import [flags]
  rustdesk-friendly apply confirm-live-verify --archive <backup-archive> --verification-dir <dir>
  rustdesk-friendly version

Guide flags:
  --target linux|windows|cross
  --topic deploy|logs|service|migrate|all
  --host <PUBLIC_HOST_OR_IP>
  --output <file>
  --migration-source linux|windows
  --migration-target linux|windows

Apply backup flags:
  --source windows|linux|darwin
  --source-data-dir <dir>
  --output <archive-path>
  --force
  --live-verify
  --triple-confirmed

Apply import flags:
  --target windows|linux|darwin
  --archive <backup-archive>
  --target-data-dir <dir>
  --force
  --validate-only
  --live-verify
  --triple-confirmed
`)
}
