// Package update handles self-update of the acli binary via GitHub Releases.
package update

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const defaultRepo = "android-cli/acli"

// Release represents a GitHub release.
type Release struct {
	TagName string  `json:"tag_name"`
	Body    string  `json:"body"`
	Assets  []Asset `json:"assets"`
}

// Asset is a downloadable file attached to a release.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// LatestRelease queries the GitHub Releases API and returns the latest release.
func LatestRelease(repo string) (*Release, error) {
	if repo == "" {
		repo = defaultRepo
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)

	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "acli-updater/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("contacting GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("repository %q not found on GitHub", repo)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decoding release: %w", err)
	}
	return &rel, nil
}

// Install downloads and atomically replaces the current binary with the
// specified release's asset for the current GOOS/GOARCH.
func Install(rel *Release) error {
	assetName := assetFileName()
	var target *Asset
	for i := range rel.Assets {
		if rel.Assets[i].Name == assetName {
			target = &rel.Assets[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("no asset named %q in release %s", assetName, rel.TagName)
	}

	// Determine current binary path
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding own executable: %w", err)
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return fmt.Errorf("resolving symlinks: %w", err)
	}

	// Download to a temp file in the same directory
	dir := filepath.Dir(self)
	tmp, err := os.CreateTemp(dir, ".acli-update-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmp.Name())

	if err := download(target.BrowserDownloadURL, tmp); err != nil {
		return err
	}
	tmp.Close()

	// Verify checksum if a .sha256 asset exists
	checksumAsset := assetName + ".sha256"
	for _, a := range rel.Assets {
		if a.Name == checksumAsset {
			if err := verifyChecksum(tmp.Name(), a.BrowserDownloadURL); err != nil {
				return fmt.Errorf("checksum mismatch: %w", err)
			}
			break
		}
	}

	// Make executable and atomically replace
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}
	if err := os.Rename(tmp.Name(), self); err != nil {
		return fmt.Errorf("replacing binary: %w", err)
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────

func assetFileName() string {
	name := fmt.Sprintf("acli-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

func download(url string, dst *os.File) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url) //nolint:noctx
	if err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	if _, err := io.Copy(dst, resp.Body); err != nil {
		return fmt.Errorf("writing download: %w", err)
	}
	return nil
}

func verifyChecksum(path, checksumURL string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(checksumURL) //nolint:noctx
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	expected := strings.Fields(string(data))
	if len(expected) == 0 {
		return fmt.Errorf("empty checksum file")
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != expected[0] {
		return fmt.Errorf("got %s, want %s", got, expected[0])
	}
	return nil
}
