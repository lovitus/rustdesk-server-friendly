package restore

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/lovitus/rustdesk-server-friendly/internal/backup"
	"github.com/lovitus/rustdesk-server-friendly/internal/bundle"
	"github.com/lovitus/rustdesk-server-friendly/internal/common"
	"github.com/lovitus/rustdesk-server-friendly/internal/platform"
	"github.com/lovitus/rustdesk-server-friendly/internal/runtimeinfo"
	"github.com/lovitus/rustdesk-server-friendly/internal/service"
	"github.com/lovitus/rustdesk-server-friendly/internal/upstream"
)

type Options struct {
	TargetOS          string
	Archive           string
	TargetDataDir     string
	Force             bool
	ValidateOnly      bool
	LiveVerify        bool
	UserConfirmedLive bool
	TripleConfirmed   bool
	Out               io.Writer
}

type Result struct {
	TargetDataDir              string
	RestoredFiles              []string
	PreBackupDir               string
	Checks                     []string
	Warnings                   []string
	BlockingIssues             []string
	DetectedRuntime            runtimeinfo.Runtime
	ServiceManager             string
	PackageContents            []bundle.FileEntry
	RestorePlan                []string
	RollbackState              []string
	VerificationLevel          string
	UserConfirmedLiveRestore   bool
	IsolatedValidationDataDir  string
	IsolatedValidationServices []string
	VerificationInstructions   []string
	VerificationReportPath     string
	ClientValidationTemplates  map[string][]string
}

func Run(opts Options) (Result, error) {
	targetOS := normalizeOS(opts.TargetOS)
	rt := runtimeinfo.Detect(targetOS)
	if !rt.Supported {
		return Result{}, fmt.Errorf("unsupported target runtime: %s/%s: %s", rt.OS, rt.Arch, rt.SupportReason)
	}

	archivePath := common.Abs(strings.TrimSpace(opts.Archive))
	if archivePath == "" {
		return Result{}, errors.New("archive path is required")
	}
	if st, err := os.Stat(archivePath); err != nil || st.IsDir() {
		return Result{}, fmt.Errorf("archive not found: %s", archivePath)
	}

	manifest, err := backup.VerifyArchive(archivePath)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		DetectedRuntime:   rt,
		ServiceManager:    rt.ServiceManager,
		PackageContents:   manifest.PackageContents,
		RestorePlan:       manifest.RestorePlan,
		VerificationLevel: manifest.VerificationLevel,
	}
	result.Checks = append(result.Checks, "archive manifest validated")

	if len(runtimeinfo.PortConflicts([]int{21115, 21116, 21117, 21118, 21119})) > 0 && !opts.LiveVerify {
		result.Warnings = append(result.Warnings, "target has active listeners on standard RustDesk ports")
	}

	targetDataDir := strings.TrimSpace(opts.TargetDataDir)
	if targetDataDir == "" {
		targetDataDir = rt.DataDir
	}
	if targetDataDir == "" {
		targetDataDir = defaultTargetDataDir(rt.OS)
	}
	targetDataDir = common.Abs(targetDataDir)
	result.TargetDataDir = targetDataDir

	if err := os.MkdirAll(targetDataDir, 0o755); err != nil {
		return result, fmt.Errorf("cannot create target data dir: %w", err)
	}

	existingServiceOrData := rt.ExistingService || hasExistingData(targetDataDir)
	if existingServiceOrData && !opts.TripleConfirmed {
		result.BlockingIssues = append(result.BlockingIssues, "existing RustDesk service or data detected; triple confirmation is required")
		return result, errors.New(result.BlockingIssues[0])
	}

	stagingDir, files, _, err := backupExtract(archivePath)
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(stagingDir)

	result.Checks = append(result.Checks, "archive extracted into staging area")
	conflicts := existingConflicts(targetDataDir, files)
	if len(conflicts) > 0 && !opts.Force {
		result.BlockingIssues = append(result.BlockingIssues, fmt.Sprintf("target contains %d conflicting files", len(conflicts)))
		return result, errors.New(result.BlockingIssues[0])
	}

	preBackupDir, rollbackFiles, err := backupCurrentTarget(targetDataDir, conflicts)
	if err != nil {
		return result, err
	}
	result.PreBackupDir = preBackupDir
	result.RollbackState = rollbackFiles
	if preBackupDir != "" {
		result.Checks = append(result.Checks, "pre-restore rollback copy created")
	}

	if opts.ValidateOnly {
		if opts.UserConfirmedLive {
			if err := markLiveRestoreVerified(archivePath); err != nil {
				return result, err
			}
			result.VerificationLevel = bundle.VerificationLiveRestore
			result.UserConfirmedLiveRestore = true
			result.Checks = append(result.Checks, "archive marked as live restore verified")
		}
		result.Checks = append(result.Checks, "validate-only mode completed without writing target data")
		return result, nil
	}

	restoreBase := targetDataDir
	if opts.LiveVerify {
		restoreBase = isolatedDataDir(targetDataDir)
		result.IsolatedValidationDataDir = restoreBase
	}
	if err := os.MkdirAll(restoreBase, 0o755); err != nil {
		return result, err
	}

	restored, err := restoreFiles(files, restoreBase)
	if err != nil {
		_ = rollback(targetDataDir, preBackupDir, conflicts, opts.Out)
		return result, err
	}
	result.RestoredFiles = restored

	if err := ensureTargetBinaries(rt, manifest, opts.Out); err != nil {
		if preBackupDir != "" {
			_ = rollback(targetDataDir, preBackupDir, conflicts, opts.Out)
		}
		return result, err
	}
	result.Checks = append(result.Checks, "target binaries are available or a download plan was prepared")

	if rt.ManagedService && rt.OS != "darwin" {
		svcNames, err := configureManagedServices(rt, restoreBase, opts.LiveVerify, opts.Out)
		if err != nil {
			if preBackupDir != "" {
				_ = rollback(targetDataDir, preBackupDir, conflicts, opts.Out)
			}
			return result, err
		}
		if opts.LiveVerify {
			result.IsolatedValidationServices = svcNames
		}
	}

	if err := healthCheck(restoreBase, opts.LiveVerify); err != nil {
		if preBackupDir != "" {
			_ = rollback(targetDataDir, preBackupDir, conflicts, opts.Out)
		}
		return result, err
	}
	result.Checks = append(result.Checks, "restore health checks passed")

	if opts.LiveVerify {
		result.VerificationLevel = bundle.VerificationRestorable
		result.Warnings = append(result.Warnings, "isolated live-restore environment is running side-by-side; wait for operator confirmation before marking success")
		result.VerificationInstructions = verificationInstructions(rt, archivePath, restoreBase, result.IsolatedValidationServices)
		result.ClientValidationTemplates = clientValidationTemplates(result.IsolatedValidationServices)
		if err := writeLiveVerifyState(restoreBase, archivePath, result.VerificationLevel, false); err != nil {
			return result, err
		}
		if opts.UserConfirmedLive {
			if err := markLiveRestoreVerified(archivePath); err != nil {
				return result, err
			}
			if err := writeLiveVerifyState(restoreBase, archivePath, bundle.VerificationLiveRestore, true); err != nil {
				return result, err
			}
			result.VerificationLevel = bundle.VerificationLiveRestore
			result.UserConfirmedLiveRestore = true
			result.Checks = append(result.Checks, "operator confirmed isolated live restore validation")
		}
	}

	reportPath, err := writeVerificationReport(result, archivePath)
	if err != nil {
		return result, err
	}
	result.VerificationReportPath = reportPath

	sort.Strings(result.RestoredFiles)
	logResult(opts.Out, result)
	return result, nil
}

func backupExtract(archivePath string) (string, []string, bundle.Manifest, error) {
	return backup.ExtractArchiveForRestore(archivePath)
}

func existingConflicts(targetDir string, extracted []string) []string {
	out := []string{}
	for _, f := range extracted {
		p := filepath.Join(targetDir, filepath.Base(f))
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			out = append(out, p)
		}
	}
	return out
}

func backupCurrentTarget(targetDir string, conflicts []string) (string, []string, error) {
	if len(conflicts) == 0 {
		return "", nil, nil
	}
	backupDir := filepath.Join(targetDir, ".rustdesk-friendly-preimport-"+time.Now().Format("20060102-150405"))
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", nil, err
	}
	rollbackFiles := []string{}
	for _, src := range conflicts {
		dst := filepath.Join(backupDir, filepath.Base(src))
		if err := common.CopyFile(src, dst); err != nil {
			return "", nil, err
		}
		rollbackFiles = append(rollbackFiles, dst)
	}
	return backupDir, rollbackFiles, nil
}

func restoreFiles(files []string, targetDir string) ([]string, error) {
	restored := []string{}
	for _, extracted := range files {
		base := filepath.Base(extracted)
		dst := filepath.Join(targetDir, base)
		if err := common.CopyFile(extracted, dst); err != nil {
			return nil, err
		}
		restored = append(restored, dst)
	}
	return restored, nil
}

func rollback(targetDir, preBackupDir string, conflicts []string, out io.Writer) error {
	if preBackupDir == "" {
		return nil
	}
	logf(out, "[ROLLBACK] restoring pre-import files from %s", preBackupDir)
	for _, conflict := range conflicts {
		src := filepath.Join(preBackupDir, filepath.Base(conflict))
		if err := common.CopyFile(src, conflict); err != nil {
			return err
		}
	}
	return nil
}

func ensureTargetBinaries(rt runtimeinfo.Runtime, manifest bundle.Manifest, out io.Writer) error {
	if len(rt.BinaryPaths) > 0 {
		return nil
	}
	logf(out, "[CHECK] target binaries were not detected; downloading upstream binaries for %s/%s", rt.OS, rt.Arch)
	status := platform.Check(rt.OS, rt.Arch)
	if !status.Supported {
		return fmt.Errorf("cannot map binaries for unsupported target %s/%s", rt.OS, rt.Arch)
	}
	installDir := defaultInstallDir(rt.OS)
	if rt.InstallDir != "" {
		installDir = rt.InstallDir
	}
	_, warnings, err := upstream.DownloadAndExtract(rt.OS, rt.Arch, installDir)
	for _, warning := range warnings {
		logf(out, "[WARN] %s", warning)
	}
	if err != nil {
		return err
	}
	return nil
}

func configureManagedServices(rt runtimeinfo.Runtime, dataDir string, isolated bool, out io.Writer) ([]string, error) {
	if !rt.ManagedService {
		return nil, nil
	}
	installDir := defaultInstallDir(rt.OS)
	if rt.InstallDir != "" {
		installDir = rt.InstallDir
	}
	logDir := filepath.Join(filepath.Dir(dataDir), "logs")
	if rt.LogDir != "" && !isolated {
		logDir = rt.LogDir
	}
	res, err := service.Apply(service.Config{
		OS:          rt.OS,
		ServiceName: "rustdesk",
		DataDir:     dataDir,
		InstallDir:  installDir,
		LogDir:      logDir,
		RelayHost:   "127.0.0.1",
		VerifyMode:  isolated,
	})
	if err != nil {
		return nil, err
	}
	for _, unit := range res.UnitPaths {
		logf(out, "[CHECK] service artifact: %s", unit)
	}
	for _, check := range res.Checks {
		logf(out, "[CHECK] %s", check)
	}
	for _, warning := range res.Warnings {
		logf(out, "[WARN] %s", warning)
	}
	return res.ServiceNames, nil
}

func chooseBinary(rt runtimeinfo.Runtime, name string) string {
	if path := rt.BinaryPaths[name]; path != "" {
		return path
	}
	if rt.OS == "windows" {
		return filepath.Join(defaultInstallDir(rt.OS), name+".exe")
	}
	return filepath.Join(defaultInstallDir(rt.OS), name)
}

func healthCheck(dataDir string, isolated bool) error {
	required := []string{"id_ed25519", "id_ed25519.pub"}
	for _, name := range required {
		if st, err := os.Stat(filepath.Join(dataDir, name)); err != nil || st.IsDir() {
			return fmt.Errorf("missing restored file: %s", name)
		}
	}
	if isolated {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return fmt.Errorf("isolated verification port check failed: %w", err)
		}
		_ = ln.Close()
	}
	return nil
}

func markLiveRestoreVerified(archivePath string) error {
	tmp, files, manifest, err := backup.ExtractArchiveForRestore(archivePath)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	manifest.VerificationLevel = bundle.VerificationLiveRestore
	manifest.UserConfirmedLiveRestore = true
	manifest.Checks = append(manifest.Checks, "isolated live restore validated by operator")
	entries := make([]backup.ArchiveRewriteEntry, 0, len(files))
	for _, file := range files {
		rel, err := filepath.Rel(tmp, file)
		if err != nil {
			return err
		}
		entries = append(entries, backup.ArchiveRewriteEntry{Src: file, Dst: filepath.ToSlash(rel)})
	}
	return backup.RewriteArchiveManifest(archivePath, entries, manifest)
}

func writeLiveVerifyState(dir, archivePath, level string, confirmed bool) error {
	if strings.TrimSpace(dir) == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	payload := map[string]any{
		"archive":                     archivePath,
		"verification_level":          level,
		"user_confirmed_live_restore": confirmed,
		"updated_at":                  time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ".rustdesk-friendly-live-verify.json"), data, 0o644)
}

func writeVerificationReport(result Result, archivePath string) (string, error) {
	baseDir := result.TargetDataDir
	if result.IsolatedValidationDataDir != "" {
		baseDir = result.IsolatedValidationDataDir
	}
	if strings.TrimSpace(baseDir) == "" {
		baseDir = filepath.Dir(archivePath)
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return "", err
	}
	payload := map[string]any{
		"archive":                      archivePath,
		"target_data_dir":              result.TargetDataDir,
		"verification_level":           result.VerificationLevel,
		"user_confirmed_live_restore":  result.UserConfirmedLiveRestore,
		"isolated_validation_data_dir": result.IsolatedValidationDataDir,
		"isolated_validation_services": result.IsolatedValidationServices,
		"verification_instructions":    result.VerificationInstructions,
		"client_validation_templates":  result.ClientValidationTemplates,
		"manual_validation_fields": map[string]string{
			"operator_name":              "",
			"validation_started_at":      "",
			"validation_completed_at":    "",
			"test_client_os":             "",
			"test_client_identifier":     "",
			"client_config_before":       "",
			"client_config_after":        "",
			"production_clients_healthy": "",
			"isolated_server_healthy":    "",
			"test_session_result":        "",
			"rollback_needed":            "",
			"final_notes":                "",
		},
		"checks":          result.Checks,
		"warnings":        result.Warnings,
		"blocking_issues": result.BlockingIssues,
		"restored_files":  result.RestoredFiles,
		"service_manager": result.ServiceManager,
		"runtime":         fmt.Sprintf("%s/%s", result.DetectedRuntime.OS, result.DetectedRuntime.Arch),
		"generated_at":    time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	jsonPath := filepath.Join(baseDir, ".rustdesk-friendly-verification-report.json")
	if err := os.WriteFile(jsonPath, data, 0o644); err != nil {
		return "", err
	}
	md := buildVerificationMarkdown(result, archivePath)
	if err := os.WriteFile(filepath.Join(baseDir, "rustdesk-friendly-verification-report.md"), []byte(md), 0o644); err != nil {
		return "", err
	}
	return jsonPath, nil
}

func buildVerificationMarkdown(result Result, archivePath string) string {
	lines := []string{
		"# RustDesk Friendly Verification Report",
		"",
		fmt.Sprintf("- Archive: `%s`", archivePath),
		fmt.Sprintf("- Runtime: `%s/%s`", result.DetectedRuntime.OS, result.DetectedRuntime.Arch),
		fmt.Sprintf("- Verification Level: `%s`", result.VerificationLevel),
		fmt.Sprintf("- User Confirmed Live Restore: `%t`", result.UserConfirmedLiveRestore),
		fmt.Sprintf("- Target Data Dir: `%s`", result.TargetDataDir),
	}
	if result.IsolatedValidationDataDir != "" {
		lines = append(lines, fmt.Sprintf("- Isolated Validation Dir: `%s`", result.IsolatedValidationDataDir))
	}
	if len(result.IsolatedValidationServices) > 0 {
		lines = append(lines, fmt.Sprintf("- Isolated Services: `%s`", strings.Join(result.IsolatedValidationServices, "`, `")))
	}
	lines = append(lines, "", "## Checks")
	for _, check := range result.Checks {
		lines = append(lines, "- "+check)
	}
	if len(result.Warnings) > 0 {
		lines = append(lines, "", "## Warnings")
		for _, warning := range result.Warnings {
			lines = append(lines, "- "+warning)
		}
	}
	if len(result.VerificationInstructions) > 0 {
		lines = append(lines, "", "## Verification Instructions")
		for _, step := range result.VerificationInstructions {
			lines = append(lines, "- "+step)
		}
	}
	if len(result.ClientValidationTemplates) > 0 {
		lines = append(lines, "", "## Client Validation Templates")
		for _, clientOS := range []string{"windows", "linux", "macos"} {
			steps := result.ClientValidationTemplates[clientOS]
			if len(steps) == 0 {
				continue
			}
			lines = append(lines, "", "### "+strings.Title(clientOS)+" Client")
			for _, step := range steps {
				lines = append(lines, "- "+step)
			}
		}
	}
	lines = append(lines,
		"",
		"## Manual Validation Record",
		"- Operator Name: ",
		"- Validation Started At: ",
		"- Validation Completed At: ",
		"- Test Client OS: ",
		"- Test Client Identifier: ",
		"- Client Config Before: ",
		"- Client Config After: ",
		"- Production Clients Healthy: ",
		"- Isolated Server Healthy: ",
		"- Test Session Result: ",
		"- Rollback Needed: ",
		"- Final Notes: ",
	)
	return strings.Join(lines, "\n") + "\n"
}

func verificationInstructions(rt runtimeinfo.Runtime, archivePath, dataDir string, serviceNames []string) []string {
	lines := []string{
		fmt.Sprintf("verification archive: %s", archivePath),
		fmt.Sprintf("isolated data dir: %s", dataDir),
	}
	if len(serviceNames) > 0 {
		lines = append(lines, "isolated services: "+strings.Join(serviceNames, ", "))
	}
	lines = append(lines,
		"pick one non-production test client and note its current ID server / relay configuration before changing anything",
		"temporarily point that test client at the isolated verification server configuration only",
		"confirm the original production server continues serving existing clients without impact while the test client is redirected",
		"confirm the isolated verification instance starts and accepts the expected RustDesk traffic",
		"confirm the test client can register, stay online, and complete an expected session path against the isolated instance",
		"after validation, return to this tool and confirm live restore success to mark the archive as live_restore_verified",
	)
	if rt.OS == "windows" {
		lines = append(lines,
			"on Windows, verify the temporary service instances in Services, NSSM, or pm2 depending on the detected manager",
			"on Windows, confirm the test client can connect after updating its ID/Relay server fields to the isolated validation instance",
		)
	} else if rt.OS == "linux" {
		lines = append(lines,
			"on Linux, verify the temporary systemd units and listening sockets before client testing",
			"on Linux, confirm the test client can connect after updating its ID/Relay server fields to the isolated validation instance",
		)
	}
	return lines
}

func clientValidationTemplates(serviceNames []string) map[string][]string {
	targetHint := "the isolated validation server"
	if len(serviceNames) > 0 {
		targetHint = fmt.Sprintf("the isolated validation server (%s)", strings.Join(serviceNames, ", "))
	}
	base := []string{
		"Use one non-production test client only.",
		"Record the current ID server and relay server values before changing anything.",
		fmt.Sprintf("Temporarily point the client to %s.", targetHint),
		"Reconnect the client and verify it registers against the isolated instance.",
		"Run one controlled session and verify it completes successfully.",
		"Restore the original client settings after the test.",
	}
	return map[string][]string{
		"windows": append([]string{
			"Open RustDesk on Windows and enter Settings -> Network.",
		}, base...),
		"linux": append([]string{
			"Open RustDesk on Linux and enter Settings -> Network.",
		}, base...),
		"macos": append([]string{
			"Open RustDesk on macOS and enter Settings -> Network.",
		}, base...),
	}
}

func hasExistingData(dir string) bool {
	for _, name := range []string{"id_ed25519", "id_ed25519.pub", "db_v2.sqlite3", "db.sqlite3"} {
		if st, err := os.Stat(filepath.Join(dir, name)); err == nil && !st.IsDir() {
			return true
		}
	}
	return false
}

func isolatedDataDir(base string) string {
	return filepath.Join(filepath.Dir(base), filepath.Base(base)+"-verify")
}

func defaultTargetDataDir(osName string) string {
	switch osName {
	case "windows":
		return `C:\RustDesk-Server\data`
	case "darwin":
		return filepath.Join(os.Getenv("HOME"), "Library/Application Support/RustDeskServer")
	default:
		return "/var/lib/rustdesk-server"
	}
}

func defaultInstallDir(osName string) string {
	if env := strings.TrimSpace(os.Getenv("RUSTDESK_FRIENDLY_INSTALL_DIR")); env != "" {
		return env
	}
	switch osName {
	case "windows":
		return `C:\RustDesk-Server`
	case "darwin":
		return "/usr/local/bin"
	default:
		return "/opt/rustdesk-server/bin"
	}
}

func normalizeOS(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return runtime.GOOS
	}
	return v
}

func logResult(out io.Writer, result Result) {
	if out == nil {
		return
	}
	fmt.Fprintf(out, "[OK] Target runtime: %s/%s\n", result.DetectedRuntime.OS, result.DetectedRuntime.Arch)
	fmt.Fprintf(out, "[OK] Target data dir: %s\n", result.TargetDataDir)
	if result.IsolatedValidationDataDir != "" {
		fmt.Fprintf(out, "[OK] Isolated validation dir: %s\n", result.IsolatedValidationDataDir)
	}
	fmt.Fprintf(out, "[OK] Verification level: %s\n", result.VerificationLevel)
	if len(result.IsolatedValidationServices) > 0 {
		fmt.Fprintf(out, "[OK] Isolated services: %s\n", strings.Join(result.IsolatedValidationServices, ", "))
	}
	if strings.TrimSpace(result.VerificationReportPath) != "" {
		fmt.Fprintf(out, "[OK] Verification report: %s\n", result.VerificationReportPath)
	}
	for _, check := range result.Checks {
		fmt.Fprintf(out, "[CHECK] %s\n", check)
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(out, "[WARN] %s\n", warning)
	}
	for _, line := range result.VerificationInstructions {
		fmt.Fprintf(out, "[VERIFY] %s\n", line)
	}
}

func logf(out io.Writer, format string, args ...any) {
	if out != nil {
		fmt.Fprintf(out, format+"\n", args...)
	}
}
