// Package avd wraps avdmanager and the emulator binary for AVD management.
package avd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/android-cli/acli/pkg/aclerr"
	"github.com/android-cli/acli/pkg/android"
	"github.com/android-cli/acli/pkg/runner"
)

// AVD represents a single Android Virtual Device.
type AVD struct {
	Name    string
	Path    string
	Target  string
	TagABI  string
	Sdcard  string
	Running bool
}

// Image represents an installable system image.
type Image struct {
	Path    string
	Version string
	Description string
}

// Service wraps avdmanager + emulator operations.
type Service struct {
	loc *android.SDKLocator
}

// New creates a new Service.
func New(loc *android.SDKLocator) *Service {
	return &Service{loc: loc}
}

// List returns all AVDs, optionally filtered to running ones.
func (s *Service) List(ctx context.Context, runningOnly bool) ([]AVD, error) {
	bin, err := s.loc.Binary("avdmanager")
	if err != nil {
		return nil, avdBinaryErr(err)
	}

	res, err := runner.RunCapture(ctx, bin, []string{"list", "avd"})
	if err != nil {
		return nil, fmt.Errorf("avdmanager list avd: %w", err)
	}

	avds := parseAVDList(res.Stdout)

	if runningOnly {
		running := runningAVDs(ctx, s.loc)
		runningSet := make(map[string]bool, len(running))
		for _, r := range running {
			runningSet[r] = true
		}
		var out []AVD
		for _, a := range avds {
			if runningSet[a.Name] {
				a.Running = true
				out = append(out, a)
			}
		}
		return out, nil
	}

	// Mark running ones
	running := runningAVDs(ctx, s.loc)
	runningSet := make(map[string]bool, len(running))
	for _, r := range running {
		runningSet[r] = true
	}
	for i := range avds {
		avds[i].Running = runningSet[avds[i].Name]
	}
	return avds, nil
}

// Create creates a new AVD.
func (s *Service) Create(ctx context.Context, name, api, device, abi, sdcard string, force bool) error {
	bin, err := s.loc.Binary("avdmanager")
	if err != nil {
		return avdBinaryErr(err)
	}

	// Build system image ID
	if abi == "" {
		abi = "x86_64"
	}
	imageID := fmt.Sprintf("system-images;android-%s;google_apis;%s", api, abi)

	args := []string{"create", "avd", "-n", name, "-k", imageID, "--force"}
	if device != "" {
		args = append(args, "-d", device)
	}
	if sdcard != "" {
		args = append(args, "-c", sdcard)
	}
	if force {
		args = append(args, "-f")
	}

	// avdmanager prompts "Do you wish to create a custom hardware profile?" → send "no"
	res, err := runner.RunWithStdin(ctx, bin, args, strings.NewReader("no\n"))
	if err != nil {
		return fmt.Errorf("avdmanager create: %w", err)
	}
	if res.ExitCode != 0 {
		if ae := aclerr.Classify("emulator", res.Stderr); ae != nil {
			return ae
		}
		return fmt.Errorf("avdmanager create: %s", res.Stderr)
	}
	return nil
}

// Delete removes an AVD.
func (s *Service) Delete(ctx context.Context, name string) error {
	bin, err := s.loc.Binary("avdmanager")
	if err != nil {
		return avdBinaryErr(err)
	}

	res, err := runner.RunCapture(ctx, bin, []string{"delete", "avd", "-n", name})
	if err != nil {
		return fmt.Errorf("avdmanager delete: %w", err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("avdmanager delete: %s", res.Stderr)
	}
	return nil
}

// Start launches an emulator for the given AVD name.
func (s *Service) Start(ctx context.Context, name string, headless bool, port int, waitBoot bool) error {
	bin, err := s.loc.Binary("emulator")
	if err != nil {
		return &aclerr.AcliError{
			Code:       aclerr.ErrBinaryNotFound,
			Message:    "emulator binary not found.",
			Detail:     "Make sure the Android Emulator is installed via the SDK Manager.",
			FixCmds:    []string{"acli sdk install emulator"},
			Underlying: err,
		}
	}

	args := []string{"-avd", name}
	if headless {
		args = append(args, "-no-window", "-no-audio", "-no-boot-anim")
	}
	if port > 0 {
		args = append(args, "-port", fmt.Sprintf("%d", port))
	}

	if waitBoot {
		// Start in background context so we can wait independently
		go func() {
			_, _ = runner.Run(context.Background(), bin, runner.Options{
				Args:        args,
				PassThrough: !headless,
			})
		}()
		return s.waitForBoot(ctx, name)
	}

	return runner.RunWith(ctx, bin, runner.Options{
		Args:        args,
		PassThrough: !headless,
	})
}

// waitForBoot polls adb until the emulator reports boot completion.
func (s *Service) waitForBoot(ctx context.Context, _ string) error {
	adb, err := s.loc.Binary("adb")
	if err != nil {
		return fmt.Errorf("adb not found for boot wait: %w", err)
	}

	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		res, _ := runner.RunCapture(ctx, adb, []string{"shell", "getprop", "sys.boot_completed"})
		if strings.TrimSpace(res.Stdout) == "1" {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return &aclerr.AcliError{
		Code:    aclerr.ErrEmulatorTimeout,
		Message: "Emulator did not finish booting within 3 minutes.",
		FixCmds: []string{"acli avd list --running"},
	}
}

// Stop kills a running emulator by serial or AVD name.
func (s *Service) Stop(ctx context.Context, serial string) error {
	adb, err := s.loc.Binary("adb")
	if err != nil {
		return fmt.Errorf("adb not found: %w", err)
	}

	res, err := runner.RunCapture(ctx, adb, []string{"-s", serial, "emu", "kill"})
	if err != nil {
		return fmt.Errorf("stopping emulator: %w", err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("stopping emulator: %s", res.Stderr)
	}
	return nil
}

// ListImages returns installable system images, optionally filtered by API level.
func (s *Service) ListImages(ctx context.Context, api string) ([]Image, error) {
	bin, err := s.loc.Binary("sdkmanager")
	if err != nil {
		return nil, fmt.Errorf("sdkmanager not found: %w", err)
	}

	res, err := runner.RunCapture(ctx, bin, []string{"--list"})
	if err != nil {
		return nil, err
	}

	var images []Image
	for _, line := range strings.Split(res.Stdout, "\n") {
		if !strings.Contains(line, "system-images") {
			continue
		}
		if api != "" && !strings.Contains(line, "android-"+api) {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 3 {
			continue
		}
		images = append(images, Image{
			Path:        strings.TrimSpace(parts[0]),
			Version:     strings.TrimSpace(parts[1]),
			Description: strings.TrimSpace(parts[2]),
		})
	}
	return images, nil
}

// ── parsing ───────────────────────────────────────────────────────────────

func parseAVDList(raw string) []AVD {
	var avds []AVD
	var cur AVD
	inAVD := false

	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "Name:") {
			if inAVD && cur.Name != "" {
				avds = append(avds, cur)
			}
			cur = AVD{Name: strings.TrimSpace(strings.TrimPrefix(trimmed, "Name:"))}
			inAVD = true
		} else if strings.HasPrefix(trimmed, "Path:") && inAVD {
			cur.Path = strings.TrimSpace(strings.TrimPrefix(trimmed, "Path:"))
		} else if strings.HasPrefix(trimmed, "Target:") && inAVD {
			cur.Target = strings.TrimSpace(strings.TrimPrefix(trimmed, "Target:"))
		} else if strings.HasPrefix(trimmed, "Tag/ABI:") && inAVD {
			cur.TagABI = strings.TrimSpace(strings.TrimPrefix(trimmed, "Tag/ABI:"))
		} else if strings.HasPrefix(trimmed, "Sdcard:") && inAVD {
			cur.Sdcard = strings.TrimSpace(strings.TrimPrefix(trimmed, "Sdcard:"))
		} else if trimmed == "---------" && inAVD {
			if cur.Name != "" {
				avds = append(avds, cur)
			}
			cur = AVD{}
			inAVD = false
		}
	}
	if inAVD && cur.Name != "" {
		avds = append(avds, cur)
	}
	return avds
}

// runningAVDs returns the names of currently running AVDs via adb.
func runningAVDs(ctx context.Context, loc *android.SDKLocator) []string {
	adb, err := loc.Binary("adb")
	if err != nil {
		return nil
	}
	res, err := runner.RunCapture(ctx, adb, []string{"devices", "-l"})
	if err != nil || res.ExitCode != 0 {
		return nil
	}
	var names []string
	for _, line := range strings.Split(res.Stdout, "\n") {
		if strings.HasPrefix(line, "emulator-") {
			serial := strings.Fields(line)[0]
			// Query the avd name via emu avd name
			r2, err := runner.RunCapture(ctx, adb, []string{"-s", serial, "emu", "avd", "name"})
			if err == nil && r2.ExitCode == 0 {
				name := strings.TrimSpace(strings.Split(r2.Stdout, "\n")[0])
				if name != "" {
					names = append(names, name)
				}
			}
		}
	}
	return names
}

func avdBinaryErr(err error) error {
	return &aclerr.AcliError{
		Code:       aclerr.ErrBinaryNotFound,
		Message:    "avdmanager not found.",
		Detail:     "Make sure the Android SDK command-line tools are installed.",
		FixCmds:    []string{"acli sdk install cmdline-tools", "acli doctor"},
		Underlying: err,
	}
}
