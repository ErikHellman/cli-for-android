// Package flash wraps fastboot for device flashing operations.
package flash

import (
	"context"
	"fmt"

	"github.com/android-cli/acli/pkg/aclerr"
	"github.com/android-cli/acli/pkg/android"
	"github.com/android-cli/acli/pkg/runner"
)

// Device is a fastboot-mode device.
type Device struct {
	Serial string
	State  string
}

// Service wraps fastboot operations.
type Service struct {
	loc *android.SDKLocator
}

// New creates a new Service.
func New(loc *android.SDKLocator) *Service {
	return &Service{loc: loc}
}

// List returns devices in fastboot mode.
func (s *Service) List(ctx context.Context) ([]Device, error) {
	fb, err := s.fastboot()
	if err != nil {
		return nil, err
	}

	res, err := runner.RunCapture(ctx, fb, []string{"devices"})
	if err != nil {
		return nil, fmt.Errorf("fastboot devices: %w", err)
	}

	var devices []Device
	for _, line := range splitLines(res.Stdout) {
		if line == "" {
			continue
		}
		fields := splitFields(line)
		if len(fields) >= 2 {
			devices = append(devices, Device{Serial: fields[0], State: fields[1]})
		}
	}
	return devices, nil
}

// Reboot reboots the device in fastboot mode.
func (s *Service) Reboot(ctx context.Context, serial string, toSystem, toBootloader bool) error {
	fb, err := s.fastboot()
	if err != nil {
		return err
	}

	args := withSerial(serial)
	if toBootloader {
		args = append(args, "reboot-bootloader")
	} else if toSystem {
		args = append(args, "reboot")
	} else {
		args = append(args, "reboot")
	}

	return runner.RunWith(ctx, fb, runner.Options{Args: args, PassThrough: true})
}

// FlashImage flashes a single partition image.
func (s *Service) FlashImage(ctx context.Context, serial, partition, file string) error {
	fb, err := s.fastboot()
	if err != nil {
		return err
	}

	args := append(withSerial(serial), "flash", partition, file)
	return runner.RunWith(ctx, fb, runner.Options{Args: args, PassThrough: true})
}

// FlashFactory flashes all partitions from a factory image zip.
// The zip is expected to contain a flash-all.sh or the standard partition images.
func (s *Service) FlashFactory(ctx context.Context, serial, zipPath string) error {
	fb, err := s.fastboot()
	if err != nil {
		return err
	}

	args := append(withSerial(serial), "update", zipPath)
	return runner.RunWith(ctx, fb, runner.Options{Args: args, PassThrough: true})
}

// Unlock sends the OEM unlock command.
func (s *Service) Unlock(ctx context.Context, serial string) error {
	fb, err := s.fastboot()
	if err != nil {
		return err
	}

	args := append(withSerial(serial), "flashing", "unlock")
	res, err := runner.RunCapture(ctx, fb, args)
	if err != nil {
		return fmt.Errorf("fastboot unlock: %w", err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("fastboot unlock: %s", res.Stderr)
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────

func (s *Service) fastboot() (string, error) {
	bin, err := s.loc.Binary("fastboot")
	if err != nil {
		return "", &aclerr.AcliError{
			Code:    aclerr.ErrBinaryNotFound,
			Message: "fastboot not found.",
			Detail:  "Make sure Android Platform Tools are installed.",
			FixCmds: []string{"acli sdk install platform-tools", "acli doctor"},
		}
	}
	return bin, nil
}

func withSerial(serial string) []string {
	if serial != "" {
		return []string{"-s", serial}
	}
	return nil
}

func splitLines(s string) []string {
	result := make([]string, 0)
	start := 0
	for i, c := range s {
		if c == '\n' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}

func splitFields(s string) []string {
	var fields []string
	inField := false
	start := 0
	for i, c := range s {
		if c == ' ' || c == '\t' {
			if inField {
				fields = append(fields, s[start:i])
				inField = false
			}
		} else {
			if !inField {
				start = i
				inField = true
			}
		}
	}
	if inField {
		fields = append(fields, s[start:])
	}
	return fields
}
