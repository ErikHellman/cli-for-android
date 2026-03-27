package aclerr_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/android-cli/acli/pkg/aclerr"
)

func TestAcliError_Error(t *testing.T) {
	ae := &aclerr.AcliError{
		Code:    aclerr.ErrDeviceNotFound,
		Message: "No device connected.",
	}
	want := "[device_not_found] No device connected."
	if got := ae.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestAcliError_Error_WithUnderlying(t *testing.T) {
	underlying := errors.New("adb: no devices")
	ae := &aclerr.AcliError{
		Code:       aclerr.ErrDeviceNotFound,
		Message:    "No device connected.",
		Underlying: underlying,
	}
	want := "[device_not_found] No device connected.: adb: no devices"
	if got := ae.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestAcliError_Unwrap(t *testing.T) {
	underlying := errors.New("root cause")
	ae := &aclerr.AcliError{
		Code:       aclerr.ErrUnknown,
		Message:    "outer",
		Underlying: underlying,
	}
	if !errors.Is(ae, underlying) {
		t.Error("errors.Is should unwrap to the underlying error")
	}
}

func TestNew(t *testing.T) {
	ae := aclerr.New(aclerr.ErrSdkNotFound, "SDK not found.")
	if ae.Code != aclerr.ErrSdkNotFound {
		t.Errorf("Code = %q, want %q", ae.Code, aclerr.ErrSdkNotFound)
	}
	if ae.Message != "SDK not found." {
		t.Errorf("Message = %q, want %q", ae.Message, "SDK not found.")
	}
}

func TestNewf(t *testing.T) {
	ae := aclerr.Newf(aclerr.ErrBuildFailed, "build %s failed with code %d", "debug", 42)
	want := "build debug failed with code 42"
	if ae.Message != want {
		t.Errorf("Message = %q, want %q", ae.Message, want)
	}
}

func TestWrap(t *testing.T) {
	root := errors.New("subprocess died")
	ae := aclerr.Wrap(root, aclerr.ErrAdbServerDead, "ADB server is not running.")
	if ae.Underlying != root {
		t.Error("Underlying should be the wrapped error")
	}
}

func TestIs(t *testing.T) {
	ae := aclerr.New(aclerr.ErrDeviceNotFound, "No device.")
	if !aclerr.Is(ae, aclerr.ErrDeviceNotFound) {
		t.Error("Is should match the correct code")
	}
	if aclerr.Is(ae, aclerr.ErrMultipleDevices) {
		t.Error("Is should not match a different code")
	}
	if aclerr.Is(fmt.Errorf("plain error"), aclerr.ErrDeviceNotFound) {
		t.Error("Is should return false for non-AcliErrors")
	}
}

func TestAs(t *testing.T) {
	ae := aclerr.New(aclerr.ErrBuildFailed, "Build failed.")
	wrapped := fmt.Errorf("wrapping: %w", ae)

	var target *aclerr.AcliError
	if !aclerr.As(wrapped, &target) {
		t.Error("As should unwrap through the chain")
	}
	if target.Code != aclerr.ErrBuildFailed {
		t.Errorf("Code = %q, want %q", target.Code, aclerr.ErrBuildFailed)
	}
}

func TestExitCode(t *testing.T) {
	tests := []struct {
		code     aclerr.ErrorCode
		wantExit int
	}{
		{aclerr.ErrDeviceNotFound, 3},
		{aclerr.ErrMultipleDevices, 3},
		{aclerr.ErrSdkNotFound, 4},
		{aclerr.ErrLicenseNotAccepted, 4},
		{aclerr.ErrBuildFailed, 5},
		{aclerr.ErrEmulatorTimeout, 6},
		{aclerr.ErrUnknown, 1},
	}
	for _, tt := range tests {
		if got := tt.code.ExitCode(); got != tt.wantExit {
			t.Errorf("ExitCode(%q) = %d, want %d", tt.code, got, tt.wantExit)
		}
	}
}

func TestCatalog_Classify(t *testing.T) {
	tests := []struct {
		tool     string
		stderr   string
		wantCode aclerr.ErrorCode
	}{
		{"adb", "error: no devices/emulators found", aclerr.ErrDeviceNotFound},
		{"adb", "error: more than one device/emulator", aclerr.ErrMultipleDevices},
		{"adb", "device unauthorized - RSA key not approved", aclerr.ErrDeviceUnauthorized},
		{"adb", "device offline", aclerr.ErrDeviceOffline},
		{"adb", "Failure [INSTALL_FAILED_ALREADY_EXISTS]", aclerr.ErrApkInstallFailed},
		{"adb", "Failure [INSTALL_FAILED_INSUFFICIENT_STORAGE]", aclerr.ErrApkInstallFailed},
		{"adb", "Failure [INSTALL_FAILED_OLDER_SDK]", aclerr.ErrApkInstallFailed},
		{"sdkmanager", "Do you Accept (y/N):", aclerr.ErrLicenseNotAccepted},
		{"sdkmanager", "licenses not accepted", aclerr.ErrLicenseNotAccepted},
		{"sdkmanager", "Failed to find package 'bad;package'", aclerr.ErrInvalidPackage},
		{"sdkmanager", "connection refused: network unreachable", aclerr.ErrNetworkError},
		{"emulator", "PANIC: No such AVD: Pixel_4", aclerr.ErrAVDNotFound},
		{"emulator", "Address already in use: port in use", aclerr.ErrPortInUse},
		{"gradle", "build failed", aclerr.ErrBuildFailed},
		{"gradle", "java.lang.OutOfMemoryError: Java heap space", aclerr.ErrOutOfMemory},
	}

	for _, tt := range tests {
		t.Run(tt.tool+"/"+string(tt.wantCode), func(t *testing.T) {
			ae := aclerr.Classify(tt.tool, tt.stderr)
			if ae == nil {
				t.Fatalf("Classify(%q, %q) returned nil, want code %q", tt.tool, tt.stderr, tt.wantCode)
			}
			if ae.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", ae.Code, tt.wantCode)
			}
			if len(ae.FixCmds) == 0 {
				t.Errorf("expected at least one fix command for %q", tt.wantCode)
			}
		})
	}
}

func TestCatalog_UnknownError(t *testing.T) {
	ae := aclerr.Classify("adb", "some random unrecognized output")
	if ae != nil {
		t.Errorf("Classify should return nil for unrecognized error, got %v", ae)
	}
}
