package upstream

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const repoAPI = "https://api.github.com/repos/rustdesk/rustdesk-server/releases/latest"

type Release struct {
	TagName string `json:"tag_name"`
}

func AssetName(osName, arch string) (string, []string, error) {
	osName = strings.ToLower(strings.TrimSpace(osName))
	arch = strings.ToLower(strings.TrimSpace(arch))
	switch osName {
	case "linux":
		switch arch {
		case "amd64":
			return "rustdesk-server-linux-amd64.zip", nil, nil
		case "arm64":
			return "rustdesk-server-linux-arm64v8.zip", nil, nil
		case "armv7":
			return "rustdesk-server-linux-armv7.zip", nil, nil
		}
	case "windows":
		switch arch {
		case "amd64":
			return "rustdesk-server-windows-x86_64-unsigned.zip", nil, nil
		case "arm64":
			return "rustdesk-server-windows-x86_64-unsigned.zip", []string{"windows arm64 falls back to upstream x86_64 build under emulation"}, nil
		}
	}
	return "", nil, fmt.Errorf("no upstream asset mapping for %s/%s", osName, arch)
}

func LatestTag() (string, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest(http.MethodGet, repoAPI, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("release lookup failed: %s", resp.Status)
	}
	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", err
	}
	if strings.TrimSpace(rel.TagName) == "" {
		return "", fmt.Errorf("release tag missing in upstream response")
	}
	return rel.TagName, nil
}

func DownloadAndExtract(osName, arch, installDir string) ([]string, []string, error) {
	if os.Getenv("RUSTDESK_FRIENDLY_SKIP_DOWNLOAD") == "1" {
		return createPlaceholders(osName, installDir)
	}
	asset, warnings, err := AssetName(osName, arch)
	if err != nil {
		return nil, nil, err
	}
	tag, err := LatestTag()
	if err != nil {
		return nil, nil, err
	}
	url := fmt.Sprintf("https://github.com/rustdesk/rustdesk-server/releases/download/%s/%s", tag, asset)
	tmpDir, err := os.MkdirTemp("", "rustdesk-friendly-upstream-")
	if err != nil {
		return nil, nil, err
	}
	defer os.RemoveAll(tmpDir)
	zipPath := filepath.Join(tmpDir, asset)
	if err := download(url, zipPath); err != nil {
		return nil, warnings, err
	}
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return nil, warnings, err
	}
	return extractBinaries(zipPath, installDir, osName)
}

func createPlaceholders(osName, installDir string) ([]string, []string, error) {
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return nil, nil, err
	}
	files := []string{}
	for _, name := range []string{"hbbs", "hbbr"} {
		dst := filepath.Join(installDir, name)
		if osName == "windows" {
			dst += ".exe"
		}
		content := []byte("placeholder generated because RUSTDESK_FRIENDLY_SKIP_DOWNLOAD=1\n")
		if err := os.WriteFile(dst, content, 0o755); err != nil {
			return nil, nil, err
		}
		files = append(files, dst)
	}
	return files, []string{"upstream download skipped by RUSTDESK_FRIENDLY_SKIP_DOWNLOAD=1"}, nil
}

func download(url, dst string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download failed: %s", resp.Status)
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

func extractBinaries(zipPath, installDir, osName string) ([]string, []string, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, nil, err
	}
	defer zr.Close()
	extracted := []string{}
	warnings := []string{}
	want := []string{"hbbs", "hbbr"}
	for _, file := range zr.File {
		base := strings.ToLower(filepath.Base(file.Name))
		for _, binary := range want {
			if base != binary && base != binary+".exe" {
				continue
			}
			rc, err := file.Open()
			if err != nil {
				return nil, warnings, err
			}
			dstName := binary
			if osName == "windows" {
				dstName += ".exe"
			}
			dst := filepath.Join(installDir, dstName)
			out, err := os.Create(dst)
			if err != nil {
				rc.Close()
				return nil, warnings, err
			}
			if _, err := io.Copy(out, rc); err != nil {
				out.Close()
				rc.Close()
				return nil, warnings, err
			}
			out.Close()
			rc.Close()
			if osName != "windows" {
				_ = os.Chmod(dst, 0o755)
			}
			extracted = append(extracted, dst)
		}
	}
	if len(extracted) < 2 {
		return nil, warnings, fmt.Errorf("failed to extract hbbs/hbbr from upstream asset")
	}
	return extracted, warnings, nil
}
