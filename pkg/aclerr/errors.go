// Package aclerr defines structured, user-facing errors for acli.
// Every error carries a machine-readable code, a human summary, optional
// detail text, suggested fix commands, and an optional docs URL.
package aclerr

import (
	stderrors "errors"
	"fmt"
)

// ErrorCode is a machine-readable error identifier.
type ErrorCode string

// ExitCode maps an ErrorCode to a POSIX exit code.
func (c ErrorCode) ExitCode() int {
	switch c {
	case ErrDeviceNotFound, ErrMultipleDevices, ErrDeviceUnauthorized, ErrDeviceOffline:
		return 3
	case ErrSdkNotFound, ErrBinaryNotFound, ErrLicenseNotAccepted, ErrBootstrapFailed:
		return 4
	case ErrBuildFailed:
		return 5
	case ErrEmulatorTimeout:
		return 6
	default:
		return 1
	}
}

const (
	// Device / ADB
	ErrDeviceNotFound     ErrorCode = "device_not_found"
	ErrMultipleDevices    ErrorCode = "multiple_devices"
	ErrDeviceUnauthorized ErrorCode = "device_unauthorized"
	ErrDeviceOffline      ErrorCode = "device_offline"
	ErrAdbServerDead      ErrorCode = "adb_server_not_running"
	ErrApkInstallFailed   ErrorCode = "apk_install_failed"

	// SDK / environment
	ErrSdkNotFound        ErrorCode = "sdk_not_found"
	ErrBinaryNotFound     ErrorCode = "binary_not_found"
	ErrLicenseNotAccepted ErrorCode = "sdk_license_not_accepted"
	ErrInvalidPackage     ErrorCode = "invalid_package"
	ErrNetworkError       ErrorCode = "network_error"

	// Build / Gradle
	ErrGradleNotFound ErrorCode = "gradle_not_found"
	ErrBuildFailed    ErrorCode = "build_failed"
	ErrOutOfMemory    ErrorCode = "out_of_memory"

	// Emulator / AVD
	ErrAVDNotFound      ErrorCode = "avd_not_found"
	ErrEmulatorTimeout  ErrorCode = "emulator_boot_timeout"
	ErrPortInUse        ErrorCode = "port_in_use"

	// Bootstrap
	ErrBootstrapFailed ErrorCode = "bootstrap_failed"

	// Misc
	ErrPermissionDenied ErrorCode = "permission_denied"
	ErrUnknown          ErrorCode = "unknown_error"
)

// AcliError is a structured, user-facing error with context and fix suggestions.
type AcliError struct {
	Code       ErrorCode
	Message    string   // one-sentence human summary
	Detail     string   // fuller explanation (optional)
	FixCmds    []string // exact commands the user can run to fix the issue
	DocsURL    string   // link to relevant docs (optional)
	Underlying error    // original error, printed only in --verbose
}

func (e *AcliError) Error() string {
	if e.Underlying != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Underlying)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *AcliError) Unwrap() error { return e.Underlying }

// New creates a simple AcliError.
func New(code ErrorCode, message string) *AcliError {
	return &AcliError{Code: code, Message: message}
}

// Newf creates an AcliError with a formatted message.
func Newf(code ErrorCode, format string, args ...any) *AcliError {
	return &AcliError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Wrap wraps an underlying error with additional context.
func Wrap(err error, code ErrorCode, message string) *AcliError {
	return &AcliError{Code: code, Message: message, Underlying: err}
}

// Is reports whether any error in err's chain matches code.
func Is(err error, code ErrorCode) bool {
	var ae *AcliError
	if stderrors.As(err, &ae) {
		return ae.Code == code
	}
	return false
}

// As is a convenience wrapper around errors.As for AcliError.
func As(err error, target **AcliError) bool {
	return stderrors.As(err, target)
}
