// Package instrument provides device state manipulation for testing and profiling.
package instrument

import (
	"context"
	"fmt"
	"strings"

	"github.com/android-cli/acli/pkg/android"
	"github.com/android-cli/acli/pkg/runner"
)

// Service wraps adb shell instrumentation commands.
type Service struct {
	loc *android.SDKLocator
}

// New creates a new Service.
func New(loc *android.SDKLocator) *Service {
	return &Service{loc: loc}
}

// Battery sets battery simulation properties.
// level: 0-100, status: "charging" | "discharging" | "full" | "not-charging"
func (s *Service) Battery(ctx context.Context, serial string, level int, status string) error {
	adb, err := s.adb()
	if err != nil {
		return err
	}

	cmds := [][]string{}
	if level >= 0 {
		cmds = append(cmds,
			s.shell(serial, "dumpsys", "battery", "set", "level", fmt.Sprintf("%d", level)),
		)
	}
	if status != "" {
		statusCode := batteryStatus(status)
		cmds = append(cmds,
			s.shell(serial, "dumpsys", "battery", "set", "status", fmt.Sprintf("%d", statusCode)),
		)
	}
	// Unplug from AC to simulate battery drain
	cmds = append(cmds, s.shell(serial, "dumpsys", "battery", "unplug"))

	for _, args := range cmds {
		res, err := runner.RunCapture(ctx, adb, args)
		if err != nil || res.ExitCode != 0 {
			return fmt.Errorf("battery command: %s", res.Stderr)
		}
	}
	return nil
}

// BatteryReset resets battery simulation to real values.
func (s *Service) BatteryReset(ctx context.Context, serial string) error {
	adb, err := s.adb()
	if err != nil {
		return err
	}
	args := s.shell(serial, "dumpsys", "battery", "reset")
	res, err := runner.RunCapture(ctx, adb, args)
	if err != nil || res.ExitCode != 0 {
		return fmt.Errorf("battery reset: %s", res.Stderr)
	}
	return nil
}

// Network applies network speed/latency simulation via the emulator console.
// speed: "full" | "hsdpa" | "edge" | "gprs" | "evdo" | "1x" | "gsm"
// latency: "none" | "umts" | "edge" | "gprs"
func (s *Service) Network(ctx context.Context, serial, speed, latency string) error {
	adb, err := s.adb()
	if err != nil {
		return err
	}

	if speed != "" {
		args := s.shell(serial, "am", "broadcast",
			"-a", "android.net.conn.CONNECTIVITY_CHANGE",
		)
		_, _ = runner.RunCapture(ctx, adb, args)
	}

	// For emulators, use the emulator console command via adb emu
	if speed != "" {
		args := append(serialArgs(serial), "emu", "network", "speed", speed)
		res, _ := runner.RunCapture(ctx, adb, args)
		if res != nil && res.ExitCode != 0 {
			return fmt.Errorf("network speed: %s", res.Stderr)
		}
	}
	if latency != "" {
		args := append(serialArgs(serial), "emu", "network", "delay", latency)
		res, _ := runner.RunCapture(ctx, adb, args)
		if res != nil && res.ExitCode != 0 {
			return fmt.Errorf("network latency: %s", res.Stderr)
		}
	}
	return nil
}

// Location sets a mock GPS location.
func (s *Service) Location(ctx context.Context, serial string, lat, lng float64) error {
	adb, err := s.adb()
	if err != nil {
		return err
	}

	args := append(serialArgs(serial), "emu", "geo", "fix",
		fmt.Sprintf("%f", lng), fmt.Sprintf("%f", lat),
	)
	res, err := runner.RunCapture(ctx, adb, args)
	if err != nil || res.ExitCode != 0 {
		// Fall back to shell am command for non-emulators
		args2 := s.shell(serial, "am", "broadcast",
			"-a", "android.intent.action.MOCK_LOCATION",
			"--ef", "latitude", fmt.Sprintf("%f", lat),
			"--ef", "longitude", fmt.Sprintf("%f", lng),
		)
		_, _ = runner.RunCapture(ctx, adb, args2)
	}
	return nil
}

// InputText types text on the device.
func (s *Service) InputText(ctx context.Context, serial, text string) error {
	adb, err := s.adb()
	if err != nil {
		return err
	}

	// Escape spaces
	escaped := strings.ReplaceAll(text, " ", "%s")
	args := s.shell(serial, "input", "text", escaped)
	res, err := runner.RunCapture(ctx, adb, args)
	if err != nil || res.ExitCode != 0 {
		return fmt.Errorf("input text: %s", res.Stderr)
	}
	return nil
}

// InputTap taps at the given screen coordinates.
func (s *Service) InputTap(ctx context.Context, serial string, x, y int) error {
	adb, err := s.adb()
	if err != nil {
		return err
	}

	args := s.shell(serial, "input", "tap", fmt.Sprintf("%d", x), fmt.Sprintf("%d", y))
	res, err := runner.RunCapture(ctx, adb, args)
	if err != nil || res.ExitCode != 0 {
		return fmt.Errorf("input tap: %s", res.Stderr)
	}
	return nil
}

// InputKey sends a keycode event.
func (s *Service) InputKey(ctx context.Context, serial, keycode string) error {
	adb, err := s.adb()
	if err != nil {
		return err
	}

	args := s.shell(serial, "input", "keyevent", keycode)
	res, err := runner.RunCapture(ctx, adb, args)
	if err != nil || res.ExitCode != 0 {
		return fmt.Errorf("input key: %s", res.Stderr)
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────

func (s *Service) adb() (string, error) {
	return s.loc.Binary("adb")
}

func (s *Service) shell(serial string, cmd ...string) []string {
	args := serialArgs(serial)
	args = append(args, "shell")
	return append(args, cmd...)
}

func serialArgs(serial string) []string {
	if serial != "" {
		return []string{"-s", serial}
	}
	return nil
}

func batteryStatus(status string) int {
	switch strings.ToLower(status) {
	case "charging":
		return 2
	case "discharging":
		return 3
	case "not-charging":
		return 4
	case "full":
		return 5
	default:
		return 3
	}
}
