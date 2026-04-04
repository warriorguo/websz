package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
)

const releaseURL = "https://api.github.com/repos/warriorguo/websz/releases/latest"

type releaseInfo struct {
	TagName string  `json:"tag_name"`
	Assets  []asset `json:"assets"`
}

type asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func assetName() string {
	name := fmt.Sprintf("websz-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

func checkForUpdate() (*releaseInfo, error) {
	req, err := http.NewRequest("GET", releaseURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to check for updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %w", err)
	}
	return &release, nil
}

// CheckLatest prints the latest available version from GitHub releases.
func CheckLatest() error {
	fmt.Println("Checking latest version...")

	release, err := checkForUpdate()
	if err != nil {
		return err
	}

	fmt.Printf("Latest version: %s\n", release.TagName)

	if Version == "dev" {
		fmt.Println("Current version: dev (no version info)")
		return nil
	}

	fmt.Printf("Current version: %s\n", Version)
	cmp, err := compareVersions(release.TagName, Version)
	if err != nil {
		return fmt.Errorf("failed to compare versions: %w", err)
	}
	if cmp <= 0 {
		fmt.Println("You are up to date.")
	} else {
		fmt.Println("A newer version is available. Run with -update to upgrade.")
	}
	return nil
}

func Update() error {
	if Version == "dev" {
		return fmt.Errorf("cannot update a development build (no version info). Please install from a release")
	}

	fmt.Printf("Current version: %s\n", Version)
	fmt.Println("Checking for updates...")

	release, err := checkForUpdate()
	if err != nil {
		return err
	}

	cmp, err := compareVersions(release.TagName, Version)
	if err != nil {
		return fmt.Errorf("failed to compare versions: %w", err)
	}
	if cmp <= 0 {
		fmt.Printf("Already up to date (%s)\n", Version)
		return nil
	}

	fmt.Printf("New version available: %s\n", release.TagName)

	want := assetName()
	var downloadURL string
	for _, a := range release.Assets {
		if a.Name == want {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		return fmt.Errorf("no release asset found for %s/%s (expected %s)", runtime.GOOS, runtime.GOARCH, want)
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	if err := checkWritePermission(execPath); err != nil {
		return err
	}

	execDir := filepath.Dir(execPath)
	tmpFile, err := os.CreateTemp(execDir, "websz-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	fmt.Printf("Downloading %s...\n", want)
	if err := downloadFile(downloadURL, tmpPath); err != nil {
		return err
	}

	info, err := os.Stat(execPath)
	if err != nil {
		return fmt.Errorf("failed to stat current binary: %w", err)
	}
	if err := os.Chmod(tmpPath, info.Mode()); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	if err := replaceBinary(execPath, tmpPath); err != nil {
		return err
	}

	fmt.Printf("Updated successfully: %s -> %s\n", Version, release.TagName)
	return nil
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return fmt.Errorf("failed to open temp file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	return nil
}

func checkWritePermission(execPath string) error {
	// Check write permission on the binary file itself
	f, err := os.OpenFile(execPath, os.O_WRONLY, 0)
	if err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("no write permission on %s\n\nTry running with elevated privileges:\n  sudo %s -update", execPath, execPath)
		}
		return fmt.Errorf("cannot access binary: %w", err)
	}
	f.Close()

	// Check write permission on the directory (needed for temp file + rename)
	execDir := filepath.Dir(execPath)
	tmp, err := os.CreateTemp(execDir, ".websz-perm-check-*")
	if err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("no write permission on directory %s\n\nTry running with elevated privileges:\n  sudo %s -update", execDir, execPath)
		}
		return fmt.Errorf("cannot write to directory %s: %w", execDir, err)
	}
	tmp.Close()
	os.Remove(tmp.Name())

	return nil
}
