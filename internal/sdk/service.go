// Package sdk wraps sdkmanager for high-level SDK management operations.
package sdk

import (
	"context"
	"fmt"
	"strings"

	"github.com/android-cli/acli/pkg/aclerr"
	"github.com/android-cli/acli/pkg/android"
	"github.com/android-cli/acli/pkg/runner"
)

// Package represents an SDK package entry from sdkmanager --list.
type Package struct {
	Path        string
	Version     string
	Description string
	Location    string
	Installed   bool
}

// Service wraps sdkmanager operations.
type Service struct {
	loc *android.SDKLocator
}

// New creates a new Service.
func New(loc *android.SDKLocator) *Service {
	return &Service{loc: loc}
}

// List returns installed and/or available SDK packages.
func (s *Service) List(ctx context.Context, installed, available bool, channel string) ([]Package, error) {
	bin, err := s.loc.Binary("sdkmanager")
	if err != nil {
		return nil, sdkBinaryErr(err)
	}

	args := []string{"--list"}
	switch channel {
	case "beta":
		args = append(args, "--channel=1")
	case "dev":
		args = append(args, "--channel=2")
	case "canary":
		args = append(args, "--channel=3")
	}

	res, err := runner.RunCapture(ctx, bin, args)
	if err != nil {
		return nil, fmt.Errorf("sdkmanager list: %w", err)
	}
	if res.ExitCode != 0 {
		return nil, classifySDKError(res.Stderr)
	}

	all := parseList(res.Stdout)

	var out []Package
	for _, p := range all {
		if installed && p.Installed {
			out = append(out, p)
		}
		if available && !p.Installed {
			out = append(out, p)
		}
		if !installed && !available {
			out = append(out, p) // return everything if no filter
		}
	}
	return out, nil
}

// Install installs one or more SDK packages by path.
func (s *Service) Install(ctx context.Context, packages []string) error {
	bin, err := s.loc.Binary("sdkmanager")
	if err != nil {
		return sdkBinaryErr(err)
	}

	args := append([]string{}, packages...)
	res, err := runner.RunCapture(ctx, bin, args)
	if err != nil {
		return fmt.Errorf("sdkmanager install: %w", err)
	}
	if res.ExitCode != 0 {
		return classifySDKError(res.Stderr)
	}
	return nil
}

// Uninstall uninstalls one or more SDK packages.
func (s *Service) Uninstall(ctx context.Context, packages []string) error {
	bin, err := s.loc.Binary("sdkmanager")
	if err != nil {
		return sdkBinaryErr(err)
	}

	args := append([]string{"--uninstall"}, packages...)
	res, err := runner.RunCapture(ctx, bin, args)
	if err != nil {
		return fmt.Errorf("sdkmanager uninstall: %w", err)
	}
	if res.ExitCode != 0 {
		return classifySDKError(res.Stderr)
	}
	return nil
}

// Update updates all installed SDK packages.
func (s *Service) Update(ctx context.Context) error {
	bin, err := s.loc.Binary("sdkmanager")
	if err != nil {
		return sdkBinaryErr(err)
	}

	res, err := runner.RunCapture(ctx, bin, []string{"--update"})
	if err != nil {
		return fmt.Errorf("sdkmanager update: %w", err)
	}
	if res.ExitCode != 0 {
		return classifySDKError(res.Stderr)
	}
	return nil
}

// AcceptLicenses pipes "y" to all license prompts.
func (s *Service) AcceptLicenses(ctx context.Context) (int, error) {
	bin, err := s.loc.Binary("sdkmanager")
	if err != nil {
		return 0, sdkBinaryErr(err)
	}

	// Build a stdin that answers "y" to every prompt
	yesInput := strings.NewReader(strings.Repeat("y\n", 100))
	res, err := runner.RunWithStdin(ctx, bin, []string{"--licenses"}, yesInput)
	if err != nil {
		return 0, fmt.Errorf("sdkmanager licenses: %w", err)
	}

	// Count how many licenses were accepted (lines containing "accepted")
	count := 0
	for _, line := range strings.Split(res.Stdout+res.Stderr, "\n") {
		if strings.Contains(strings.ToLower(line), "accepted") {
			count++
		}
	}
	return count, nil
}

// ── parsing ───────────────────────────────────────────────────────────────

// parseList parses the output of `sdkmanager --list`.
// Format (pipe-delimited, indented):
//
//	  Path                      | Version | Description        | Location
//	  ----                      | ------- | -----------        | --------
//	  build-tools;30.0.3        | 30.0.3  | Build-Tools 30.0.3 | build-tools/30.0.3
func parseList(raw string) []Package {
	var pkgs []Package
	section := "" // "installed" or "available"

	for _, line := range strings.Split(raw, "\n") {
		lower := strings.ToLower(strings.TrimSpace(line))
		if strings.HasPrefix(lower, "installed packages") {
			section = "installed"
			continue
		}
		if strings.HasPrefix(lower, "available packages") || strings.HasPrefix(lower, "available updates") {
			section = "available"
			continue
		}
		if section == "" || strings.TrimSpace(line) == "" {
			continue
		}
		// Skip header/separator lines
		if strings.HasPrefix(strings.TrimSpace(line), "Path") || strings.HasPrefix(strings.TrimSpace(line), "---") {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 3 {
			continue
		}
		p := Package{
			Path:        strings.TrimSpace(parts[0]),
			Version:     strings.TrimSpace(parts[1]),
			Description: strings.TrimSpace(parts[2]),
			Installed:   section == "installed",
		}
		if len(parts) >= 4 {
			p.Location = strings.TrimSpace(parts[3])
		}
		if p.Path != "" && !strings.HasPrefix(p.Path, "-") {
			pkgs = append(pkgs, p)
		}
	}
	return pkgs
}

// ── error helpers ─────────────────────────────────────────────────────────

func classifySDKError(stderr string) error {
	if ae := aclerr.Classify("sdkmanager", stderr); ae != nil {
		return ae
	}
	return fmt.Errorf("sdkmanager error: %s", stderr)
}

func sdkBinaryErr(err error) error {
	return &aclerr.AcliError{
		Code:       aclerr.ErrBinaryNotFound,
		Message:    "sdkmanager not found.",
		Detail:     "Make sure the Android SDK command-line tools are installed and $ANDROID_HOME is set.",
		FixCmds:    []string{"acli doctor"},
		Underlying: err,
	}
}
