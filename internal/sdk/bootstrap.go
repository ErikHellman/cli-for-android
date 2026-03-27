package sdk

import (
	"archive/zip"
	"context"
	"crypto/sha1" //nolint:gosec // repository2-3.xml uses SHA-1
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/android-cli/acli/pkg/aclerr"
)

const (
	manifestURL     = "https://dl.google.com/android/repository/repository2-3.xml"
	downloadBaseURL = "https://dl.google.com/android/repository/"
)

// Bootstrapper installs Android SDK command-line tools from scratch.
// It has no dependency on android.SDKLocator and works without any pre-existing SDK.
type Bootstrapper struct{}

// NewBootstrapper creates a Bootstrapper.
func NewBootstrapper() *Bootstrapper { return &Bootstrapper{} }

// Bootstrap installs cmdline-tools into the resolved SDK root.
// targetDir overrides auto-detection when non-empty; otherwise the resolution
// order is $ANDROID_HOME → $ANDROID_SDK_ROOT → platform default.
// Returns (sdkRoot, alreadyInstalled, error).
func (b *Bootstrapper) Bootstrap(ctx context.Context, targetDir string, force bool) (string, bool, error) {
	sdkRoot := targetDir
	if sdkRoot == "" {
		sdkRoot = resolveSDKRoot()
	}

	// Check if already installed
	marker := sdkmanagerPath(sdkRoot)
	if !force {
		if _, err := os.Stat(marker); err == nil {
			return sdkRoot, true, nil
		}
	}

	// Fetch and parse the repository manifest
	m, err := fetchManifest(ctx)
	if err != nil {
		return "", false, &aclerr.AcliError{
			Code:       aclerr.ErrBootstrapFailed,
			Message:    "Failed to fetch the Android SDK repository manifest.",
			Detail:     "Could not reach dl.google.com. Check your internet connection or proxy settings.",
			FixCmds:    []string{"acli sdk bootstrap"},
			Underlying: err,
		}
	}

	// Find the archive for this host OS
	ar, err := findCmdlineTools(m)
	if err != nil {
		return "", false, &aclerr.AcliError{
			Code:       aclerr.ErrBootstrapFailed,
			Message:    "No command-line tools found in the manifest for this operating system.",
			Underlying: err,
		}
	}

	// Ensure SDK root directory exists
	if err := os.MkdirAll(sdkRoot, 0o755); err != nil {
		return "", false, fmt.Errorf("creating SDK root %q: %w", sdkRoot, err)
	}

	if err := downloadAndInstall(ctx, ar, sdkRoot); err != nil {
		return "", false, err
	}

	return sdkRoot, false, nil
}

// ── XML types for repository2-3.xml ──────────────────────────────────────

type repoManifest struct {
	Packages []remotePackage `xml:"remotePackage"`
}

type remotePackage struct {
	Path     string    `xml:"path,attr"`
	Archives []archive `xml:"archives>archive"`
}

type archive struct {
	HostOS   string `xml:"host-os"`
	URL      string `xml:"complete>url"`
	Checksum string `xml:"complete>checksum"`
	Size     int64  `xml:"complete>size"`
}

// ── manifest fetching ─────────────────────────────────────────────────────

func fetchManifest(ctx context.Context) (*repoManifest, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching manifest: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("manifest returned HTTP %d", resp.StatusCode)
	}
	var m repoManifest
	if err := xml.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}
	return &m, nil
}

func findCmdlineTools(m *repoManifest) (*archive, error) {
	hostOS := hostOSTag()
	// Packages are ordered newest-first in the manifest. Take the first stable
	// cmdline-tools entry (path = "cmdline-tools;<version>") that has an archive
	// for the current host OS.
	for i := range m.Packages {
		path := m.Packages[i].Path
		if !strings.HasPrefix(path, "cmdline-tools;") {
			continue
		}
		// Skip pre-release versions (e.g. "cmdline-tools;19.0-alpha01")
		version := strings.TrimPrefix(path, "cmdline-tools;")
		if strings.Contains(version, "-") {
			continue
		}
		for j := range m.Packages[i].Archives {
			ar := &m.Packages[i].Archives[j]
			if ar.HostOS == hostOS {
				return ar, nil
			}
		}
	}
	return nil, fmt.Errorf("no cmdline-tools archive for host OS %q", hostOS)
}

func hostOSTag() string {
	switch runtime.GOOS {
	case "darwin":
		return "macosx"
	case "windows":
		return "windows"
	default:
		return "linux"
	}
}

// ── download and install ──────────────────────────────────────────────────

func downloadAndInstall(ctx context.Context, ar *archive, sdkRoot string) error {
	url := downloadBaseURL + ar.URL

	// Download to a temp file
	tmp, err := os.CreateTemp("", "acli-cmdlinetools-*.zip")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := downloadFile(ctx, url, tmp); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	// Verify SHA-1 checksum from the manifest
	if ar.Checksum != "" {
		if err := verifySHA1(tmpName, ar.Checksum); err != nil {
			return &aclerr.AcliError{
				Code:       aclerr.ErrBootstrapFailed,
				Message:    "Checksum verification failed for the downloaded zip.",
				Detail:     "The file may be corrupt or the manifest checksum is stale. Try again.",
				Underlying: err,
			}
		}
	}

	// Extract: strip the leading "cmdline-tools/" component so contents land at
	// <sdk-root>/cmdline-tools/latest/
	destBase := filepath.Join(sdkRoot, "cmdline-tools", "latest")
	if err := extractZipStrippingFirst(tmpName, destBase); err != nil {
		return &aclerr.AcliError{
			Code:       aclerr.ErrBootstrapFailed,
			Message:    "Failed to extract the command-line tools zip.",
			Underlying: err,
		}
	}
	return nil
}

func downloadFile(ctx context.Context, url string, dst *os.File) error {
	client := &http.Client{Timeout: 10 * time.Minute}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}
	if _, err := io.Copy(dst, resp.Body); err != nil {
		return fmt.Errorf("writing download: %w", err)
	}
	return nil
}

func verifySHA1(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha1.New() //nolint:gosec
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != strings.ToLower(expected) {
		return fmt.Errorf("SHA-1 mismatch: got %s, want %s", got, strings.ToLower(expected))
	}
	return nil
}

// extractZipStrippingFirst extracts zipPath to destBase, stripping the first
// path component from every entry so that "cmdline-tools/bin/sdkmanager"
// becomes destBase+"/bin/sdkmanager".
func extractZipStrippingFirst(zipPath, destBase string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("opening zip: %w", err)
	}
	defer r.Close()

	cleanDest := filepath.Clean(destBase)

	for _, f := range r.File {
		// Normalize to forward slashes for splitting
		slashed := filepath.ToSlash(f.Name)
		parts := strings.SplitN(slashed, "/", 2)
		if len(parts) < 2 || parts[1] == "" {
			continue // skip the top-level directory entry itself
		}
		rel := parts[1]

		dest := filepath.Join(cleanDest, filepath.FromSlash(rel))

		// Guard against zip slip
		if !strings.HasPrefix(filepath.Clean(dest)+string(os.PathSeparator), cleanDest+string(os.PathSeparator)) {
			return fmt.Errorf("zip entry %q would escape destination", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dest, f.Mode()); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := writeZipEntry(f, dest); err != nil {
			return err
		}
	}
	return nil
}

func writeZipEntry(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}

// ── path helpers ──────────────────────────────────────────────────────────

// resolveSDKRoot returns the best SDK root to install into:
// $ANDROID_HOME → $ANDROID_SDK_ROOT → platform default.
func resolveSDKRoot() string {
	for _, env := range []string{"ANDROID_HOME", "ANDROID_SDK_ROOT"} {
		if v := os.Getenv(env); v != "" {
			return v
		}
	}
	return platformDefaultSDKRoot()
}

func platformDefaultSDKRoot() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Android", "sdk")
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		return filepath.Join(localAppData, "Android", "Sdk")
	default:
		return filepath.Join(home, "Android", "Sdk")
	}
}

// sdkmanagerPath returns the expected sdkmanager binary path after bootstrap.
func sdkmanagerPath(sdkRoot string) string {
	name := "sdkmanager"
	if runtime.GOOS == "windows" {
		name = "sdkmanager.bat"
	}
	return filepath.Join(sdkRoot, "cmdline-tools", "latest", "bin", name)
}
