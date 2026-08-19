package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const appVersion = "0.9.0"
const releaseAPI = "https://api.github.com/repos/mrspock64/wspr-beacon/releases/latest"

type UpdateInfo struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	Available      bool   `json:"available"`
	AssetURL       string `json:"assetURL"`
	Notes          string `json:"notes"`
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	Body    string `json:"body"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func (a *App) CheckForUpdate() (UpdateInfo, error) {
	request, err := http.NewRequest(http.MethodGet, releaseAPI, nil)
	if err != nil {
		return UpdateInfo{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "WSPR-Beacon-Updater")
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return UpdateInfo{}, fmt.Errorf("kunde inte kontakta GitHub: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return UpdateInfo{CurrentVersion: appVersion, LatestVersion: appVersion}, nil
	}
	if response.StatusCode != http.StatusOK {
		return UpdateInfo{}, fmt.Errorf("GitHub svarade %s", response.Status)
	}
	var release githubRelease
	if err := json.NewDecoder(response.Body).Decode(&release); err != nil {
		return UpdateInfo{}, err
	}
	latest := strings.TrimPrefix(release.TagName, "v")
	info := UpdateInfo{CurrentVersion: appVersion, LatestVersion: latest, Available: versionGreater(latest, appVersion), Notes: release.Body}
	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, "macos-arm64") && strings.HasSuffix(asset.Name, ".zip") {
			info.AssetURL = asset.URL
			break
		}
	}
	if info.Available && info.AssetURL == "" {
		return UpdateInfo{}, errors.New("senaste releasen saknar macOS Apple Silicon-arkiv")
	}
	return info, nil
}

func versionGreater(candidate, current string) bool {
	var a, b, c, x, y, z int
	fmt.Sscanf(candidate, "%d.%d.%d", &a, &b, &c)
	fmt.Sscanf(current, "%d.%d.%d", &x, &y, &z)
	if a != x {
		return a > x
	}
	if b != y {
		return b > y
	}
	return c > z
}

func (a *App) InstallUpdate(assetURL string) error {
	if goruntime.GOOS != "darwin" {
		return errors.New("automatisk installation är för närvarande tillgänglig på macOS")
	}
	if !strings.HasPrefix(assetURL, "https://github.com/mrspock64/wspr-beacon/releases/download/") {
		return errors.New("ogiltig uppdateringslänk")
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	appPath := filepath.Clean(filepath.Join(executable, "../../.."))
	if filepath.Ext(appPath) != ".app" {
		return errors.New("appen måste köras från WSPR Beacon.app för att kunna uppdateras")
	}
	temporary, err := os.MkdirTemp("", "wspr-beacon-update-*")
	if err != nil {
		return err
	}
	archivePath := filepath.Join(temporary, "update.zip")
	if err := downloadFile(assetURL, archivePath); err != nil {
		return err
	}
	newApp, err := extractApp(archivePath, temporary)
	if err != nil {
		return err
	}
	scriptPath := filepath.Join(temporary, "install-update.sh")
	script := fmt.Sprintf("#!/bin/sh\nsleep 1\nrm -rf %q\nditto %q %q\nopen %q\n", appPath, newApp, appPath, appPath)
	if err := os.WriteFile(scriptPath, []byte(script), 0700); err != nil {
		return err
	}
	if err := exec.Command("/bin/sh", scriptPath).Start(); err != nil {
		return err
	}
	runtime.Quit(a.ctx)
	return nil
}

func downloadFile(url, target string) error {
	client := &http.Client{Timeout: 2 * time.Minute}
	response, err := client.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("kunde inte hämta uppdateringen: %s", response.Status)
	}
	file, err := os.Create(target)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, response.Body)
	return err
}

func extractApp(archivePath, destination string) (string, error) {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer archive.Close()
	for _, entry := range archive.File {
		name := filepath.Clean(entry.Name)
		if strings.HasPrefix(name, "../") || filepath.IsAbs(name) {
			return "", errors.New("osäker sökväg i uppdateringsarkivet")
		}
		target := filepath.Join(destination, name)
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return "", err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return "", err
		}
		input, err := entry.Open()
		if err != nil {
			return "", err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, entry.Mode())
		if err != nil {
			input.Close()
			return "", err
		}
		_, copyErr := io.Copy(output, input)
		input.Close()
		output.Close()
		if copyErr != nil {
			return "", copyErr
		}
	}
	appPath := filepath.Join(destination, "wspr-beacon.app")
	if _, err := os.Stat(appPath); err != nil {
		return "", errors.New("uppdateringsarkivet saknar wspr-beacon.app")
	}
	return appPath, nil
}
