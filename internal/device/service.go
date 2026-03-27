// Package device wraps adb for device management operations.
package device

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/android-cli/acli/pkg/aclerr"
	"github.com/android-cli/acli/pkg/android"
	"github.com/android-cli/acli/pkg/runner"
)

// Device represents a connected ADB device or emulator.
type Device struct {
	Serial    string
	State     string // "device", "offline", "unauthorized", "bootloader"
	Product   string
	Model     string
	DeviceID  string
	IsEmu     bool
	Transport string
}

// Service wraps ADB device operations.
type Service struct {
	loc *android.SDKLocator
}

// New creates a new Service.
func New(loc *android.SDKLocator) *Service {
	return &Service{loc: loc}
}

// List returns all connected ADB devices/emulators.
func (s *Service) List(ctx context.Context) ([]Device, error) {
	adb, err := s.adb()
	if err != nil {
		return nil, err
	}

	res, err := runner.RunCapture(ctx, adb, []string{"devices", "-l"})
	if err != nil {
		return nil, fmt.Errorf("adb devices: %w", err)
	}
	return parseDeviceList(res.Stdout), nil
}

// Shell runs a shell command on the target device.
// If cmd is empty, opens an interactive shell (passthrough).
func (s *Service) Shell(ctx context.Context, serial, cmd string) error {
	adb, err := s.adb()
	if err != nil {
		return err
	}

	args := s.withSerial(serial, "shell")
	if cmd != "" {
		args = append(args, strings.Fields(cmd)...)
	}

	return runner.RunWith(ctx, adb, runner.Options{
		Args:        args,
		PassThrough: true,
	})
}

// Install installs an APK on the target device.
func (s *Service) Install(ctx context.Context, serial, apkPath string, grantAll, reinstall, downgrade bool) error {
	adb, err := s.adb()
	if err != nil {
		return err
	}

	args := s.withSerial(serial, "install")
	if grantAll {
		args = append(args, "-g")
	}
	if reinstall {
		args = append(args, "-r")
	}
	if downgrade {
		args = append(args, "-d")
	}
	args = append(args, apkPath)

	res, err := runner.RunCapture(ctx, adb, args)
	if err != nil {
		return fmt.Errorf("adb install: %w", err)
	}
	combined := res.Stdout + res.Stderr
	if res.ExitCode != 0 || strings.Contains(combined, "INSTALL_FAILED") {
		if ae := aclerr.Classify("adb", combined); ae != nil {
			return ae
		}
		return fmt.Errorf("adb install failed: %s", combined)
	}
	return nil
}

// Uninstall removes a package from the target device.
func (s *Service) Uninstall(ctx context.Context, serial, pkg string) error {
	adb, err := s.adb()
	if err != nil {
		return err
	}

	args := append(s.withSerial(serial, "uninstall"), pkg)
	res, err := runner.RunCapture(ctx, adb, args)
	if err != nil {
		return fmt.Errorf("adb uninstall: %w", err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("adb uninstall: %s", res.Stderr)
	}
	return nil
}

// Logs streams logcat output (passthrough).
func (s *Service) Logs(ctx context.Context, serial, tag, level string, follow, clear bool) error {
	adb, err := s.adb()
	if err != nil {
		return err
	}

	if clear {
		args := append(s.withSerial(serial, "logcat"), "-c")
		_, _ = runner.RunCapture(ctx, adb, args)
	}

	args := s.withSerial(serial, "logcat")
	if !follow {
		args = append(args, "-d") // dump and exit
	}
	if tag != "" && level != "" {
		args = append(args, "*:S", tag+":"+level)
	} else if level != "" {
		args = append(args, "*:"+level)
	}

	return runner.RunWith(ctx, adb, runner.Options{Args: args, PassThrough: true})
}

// Push copies a local file to the device.
func (s *Service) Push(ctx context.Context, serial, local, remote string) error {
	adb, err := s.adb()
	if err != nil {
		return err
	}

	args := append(s.withSerial(serial, "push"), local, remote)
	res, err := runner.RunCapture(ctx, adb, args)
	if err != nil || res.ExitCode != 0 {
		return fmt.Errorf("adb push: %s", res.Stderr)
	}
	return nil
}

// Pull copies a file from the device to the host.
func (s *Service) Pull(ctx context.Context, serial, remote, local string) error {
	adb, err := s.adb()
	if err != nil {
		return err
	}

	if local == "" {
		local = "."
	}
	args := append(s.withSerial(serial, "pull"), remote, local)
	res, err := runner.RunCapture(ctx, adb, args)
	if err != nil || res.ExitCode != 0 {
		return fmt.Errorf("adb pull: %s", res.Stderr)
	}
	return nil
}

// Screenshot captures a screenshot and saves it to outputPath.
func (s *Service) Screenshot(ctx context.Context, serial, outputPath string) error {
	adb, err := s.adb()
	if err != nil {
		return err
	}

	if outputPath == "" {
		outputPath = fmt.Sprintf("screenshot-%d.png", time.Now().Unix())
	}

	// Use exec-out to stream directly to a file
	args := append(s.withSerial(serial, "exec-out"), "screencap", "-p")
	res, err := runner.RunCapture(ctx, adb, args)
	if err != nil {
		return fmt.Errorf("adb screencap: %w", err)
	}

	if err := os.WriteFile(outputPath, []byte(res.Stdout), 0o644); err != nil {
		return fmt.Errorf("writing screenshot: %w", err)
	}
	return nil
}

// Record records the screen to a file.
func (s *Service) Record(ctx context.Context, serial string, duration int, size, outputPath string) error {
	adb, err := s.adb()
	if err != nil {
		return err
	}

	if outputPath == "" {
		outputPath = fmt.Sprintf("screenrecord-%d.mp4", time.Now().Unix())
	}
	remotePath := "/sdcard/acli-record.mp4"

	recordArgs := []string{"screenrecord"}
	if size != "" {
		recordArgs = append(recordArgs, "--size", size)
	}
	if duration > 0 {
		recordArgs = append(recordArgs, "--time-limit", fmt.Sprintf("%d", duration))
	}
	recordArgs = append(recordArgs, remotePath)

	shellArgs := append(s.withSerial(serial, "shell"), recordArgs...)
	_, _ = runner.Run(ctx, adb, runner.Options{Args: shellArgs, PassThrough: true})

	// Pull the file
	return s.Pull(ctx, serial, remotePath, filepath.Dir(outputPath))
}

// Reboot reboots the device.
func (s *Service) Reboot(ctx context.Context, serial string, bootloader, recovery bool) error {
	adb, err := s.adb()
	if err != nil {
		return err
	}

	args := s.withSerial(serial, "reboot")
	if bootloader {
		args = append(args, "bootloader")
	} else if recovery {
		args = append(args, "recovery")
	}

	res, err := runner.RunCapture(ctx, adb, args)
	if err != nil {
		return fmt.Errorf("adb reboot: %w", err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("adb reboot: %s", res.Stderr)
	}
	return nil
}

// Forward sets up a TCP port forward from host to device.
func (s *Service) Forward(ctx context.Context, serial, localPort, remotePort string) error {
	adb, err := s.adb()
	if err != nil {
		return err
	}

	args := append(s.withSerial(serial, "forward"), "tcp:"+localPort, "tcp:"+remotePort)
	res, err := runner.RunCapture(ctx, adb, args)
	if err != nil || res.ExitCode != 0 {
		return fmt.Errorf("adb forward: %s", res.Stderr)
	}
	return nil
}

// Reverse sets up a reverse TCP port forward (device → host).
func (s *Service) Reverse(ctx context.Context, serial, remotePort, localPort string) error {
	adb, err := s.adb()
	if err != nil {
		return err
	}

	args := append(s.withSerial(serial, "reverse"), "tcp:"+remotePort, "tcp:"+localPort)
	res, err := runner.RunCapture(ctx, adb, args)
	if err != nil || res.ExitCode != 0 {
		return fmt.Errorf("adb reverse: %s", res.Stderr)
	}
	return nil
}

// Pair pairs a device over Wi-Fi (Android 11+).
func (s *Service) Pair(ctx context.Context, addr string) error {
	adb, err := s.adb()
	if err != nil {
		return err
	}

	return runner.RunWith(ctx, adb, runner.Options{
		Args:        []string{"pair", addr},
		PassThrough: true,
	})
}

// Connect connects to a device over Wi-Fi.
func (s *Service) Connect(ctx context.Context, addr string) error {
	adb, err := s.adb()
	if err != nil {
		return err
	}

	res, err := runner.RunCapture(ctx, adb, []string{"connect", addr})
	if err != nil {
		return fmt.Errorf("adb connect: %w", err)
	}
	if strings.Contains(res.Stdout+res.Stderr, "failed") {
		return fmt.Errorf("adb connect: %s", res.Stdout)
	}
	return nil
}

// Info returns device properties.
func (s *Service) Info(ctx context.Context, serial string) (map[string]string, error) {
	adb, err := s.adb()
	if err != nil {
		return nil, err
	}

	props := []string{
		"ro.product.model",
		"ro.build.version.release",
		"ro.build.version.sdk",
		"ro.product.cpu.abi",
		"ro.product.manufacturer",
		"ro.product.name",
	}

	info := make(map[string]string, len(props)+1)
	info["serial"] = serial

	for _, prop := range props {
		args := append(s.withSerial(serial, "shell"), "getprop", prop)
		res, err := runner.RunCapture(ctx, adb, args)
		if err == nil && res.ExitCode == 0 {
			info[prop] = strings.TrimSpace(res.Stdout)
		}
	}
	return info, nil
}

// ── parsing ───────────────────────────────────────────────────────────────

// parseDeviceList parses `adb devices -l` output.
func parseDeviceList(raw string) []Device {
	var devices []Device
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of") || strings.HasPrefix(line, "*") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		d := Device{Serial: fields[0], State: fields[1]}
		d.IsEmu = strings.HasPrefix(d.Serial, "emulator-")

		for _, kv := range fields[2:] {
			parts := strings.SplitN(kv, ":", 2)
			if len(parts) != 2 {
				continue
			}
			switch parts[0] {
			case "product":
				d.Product = parts[1]
			case "model":
				d.Model = parts[1]
			case "device":
				d.DeviceID = parts[1]
			case "transport_id":
				d.Transport = parts[1]
			}
		}
		devices = append(devices, d)
	}
	return devices
}

// ── helpers ───────────────────────────────────────────────────────────────

func (s *Service) adb() (string, error) {
	bin, err := s.loc.Binary("adb")
	if err != nil {
		return "", &aclerr.AcliError{
			Code:    aclerr.ErrBinaryNotFound,
			Message: "adb not found.",
			Detail:  "Make sure Android Platform Tools are installed and in your PATH.",
			FixCmds: []string{"acli sdk install platform-tools", "acli doctor"},
		}
	}
	return bin, nil
}

// withSerial prepends -s <serial> if serial is non-empty.
func (s *Service) withSerial(serial, subcmd string) []string {
	if serial != "" {
		return []string{"-s", serial, subcmd}
	}
	return []string{subcmd}
}
