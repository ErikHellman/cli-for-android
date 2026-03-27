// Package android locates the Android SDK root and its constituent binaries.
// Discovery order: $ANDROID_HOME → $ANDROID_SDK_ROOT → well-known paths.
package android

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// SDKLocator finds Android SDK binaries.
type SDKLocator struct {
	root string // cached SDK root
}

// New creates an SDKLocator, immediately attempting to discover the SDK root.
func New() (*SDKLocator, error) {
	l := &SDKLocator{}
	root, err := l.findRoot()
	if err != nil {
		return nil, err
	}
	l.root = root
	return l, nil
}

// Root returns the discovered Android SDK root directory.
func (l *SDKLocator) Root() string { return l.root }

// Binary returns the absolute path to a named Android SDK binary, or an error
// if it cannot be located.
// name can be "adb", "sdkmanager", "avdmanager", "emulator", or "fastboot".
func (l *SDKLocator) Binary(name string) (string, error) {
	// Candidate locations within the SDK
	candidates := binaryPaths(l.root, name)
	for _, p := range candidates {
		if isExecutable(p) {
			return p, nil
		}
	}

	// Fall through to PATH
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}

	return "", fmt.Errorf("binary %q not found in SDK root %q or $PATH – run `acli doctor` for details", name, l.root)
}

// MustBinary is like Binary but panics if the binary cannot be found.
func (l *SDKLocator) MustBinary(name string) string {
	p, err := l.Binary(name)
	if err != nil {
		panic(err)
	}
	return p
}

// ── root discovery ────────────────────────────────────────────────────────

func (l *SDKLocator) findRoot() (string, error) {
	// 1. Explicit env vars
	for _, ev := range []string{"ANDROID_HOME", "ANDROID_SDK_ROOT"} {
		if v := os.Getenv(ev); v != "" {
			if isDir(v) {
				return v, nil
			}
		}
	}

	// 2. Well-known platform paths
	for _, p := range wellKnownRoots() {
		expanded := expandHome(p)
		if isDir(expanded) {
			return expanded, nil
		}
	}

	return "", fmt.Errorf(
		"Android SDK not found. Set $ANDROID_HOME to your SDK directory or run `acli doctor`.\n"+
			"Common locations:\n  macOS: ~/Library/Android/sdk\n  Linux: ~/Android/Sdk",
	)
}

func wellKnownRoots() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"~/Library/Android/sdk",
			"~/Library/Android/Sdk",
		}
	case "linux":
		return []string{
			"~/Android/Sdk",
			"~/android/sdk",
			"/opt/android-sdk",
			"/usr/lib/android-sdk",
		}
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		return []string{
			filepath.Join(localAppData, "Android", "Sdk"),
		}
	}
	return nil
}

// ── binary path resolution ────────────────────────────────────────────────

// binaryPaths returns candidate absolute paths for a binary within the SDK.
func binaryPaths(root, name string) []string {
	exe := name
	if runtime.GOOS == "windows" {
		exe = name + ".exe"
		if name == "sdkmanager" || name == "avdmanager" {
			exe = name + ".bat"
		}
	}
	return []string{
		// platform-tools: adb, fastboot
		filepath.Join(root, "platform-tools", exe),
		// emulator
		filepath.Join(root, "emulator", exe),
		// cmdline-tools – try "latest" first, then scan for any version
		filepath.Join(root, "cmdline-tools", "latest", "bin", exe),
		filepath.Join(root, "cmdline-tools", "bin", exe),
		// legacy location
		filepath.Join(root, "tools", "bin", exe),
		filepath.Join(root, "tools", exe),
	}
}

// ── helpers ───────────────────────────────────────────────────────────────

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func isExecutable(p string) bool {
	fi, err := os.Stat(p)
	if err != nil || fi.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true // trust extension
	}
	return fi.Mode()&0o111 != 0
}

func expandHome(p string) string {
	if len(p) > 1 && p[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}
