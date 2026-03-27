package output_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/android-cli/acli/pkg/aclerr"
	"github.com/android-cli/acli/pkg/output"
)

// captureStderr temporarily replaces os.Stderr and returns captured content.
func captureStderr(fn func()) string {
	orig := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	fn()

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r) //nolint:errcheck
	os.Stderr = orig
	return buf.String()
}

// captureStdout temporarily replaces os.Stdout and returns captured content.
func captureStdout(fn func()) string {
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r) //nolint:errcheck
	os.Stdout = orig
	return buf.String()
}

func TestRenderer_JSON_Error(t *testing.T) {
	output.Init(true, false, true)
	ae := &aclerr.AcliError{
		Code:    aclerr.ErrDeviceNotFound,
		Message: "No device connected.",
		Detail:  "Use USB debugging.",
		FixCmds: []string{"acli device list"},
		DocsURL: "https://developer.android.com/tools/adb",
	}

	captured := captureStderr(func() {
		output.Error(ae)
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(captured), &result); err != nil {
		t.Fatalf("JSON output is not valid JSON: %v\nOutput was: %s", err, captured)
	}

	errMap, ok := result["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'error' key in JSON output, got: %v", result)
	}
	if errMap["code"] != "device_not_found" {
		t.Errorf("code = %v, want device_not_found", errMap["code"])
	}
	if errMap["message"] != "No device connected." {
		t.Errorf("message = %v, want 'No device connected.'", errMap["message"])
	}
	fixSlice, ok := errMap["fix"].([]any)
	if !ok || len(fixSlice) == 0 {
		t.Error("expected 'fix' to be a non-empty array")
	}
}

func TestRenderer_JSON_Table(t *testing.T) {
	output.Init(true, false, true)

	captured := captureStdout(func() {
		output.Table(
			[]string{"Serial", "State"},
			[][]string{
				{"emulator-5554", "device"},
				{"abc123", "offline"},
			},
		)
	})

	var result []map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(captured)), &result); err != nil {
		t.Fatalf("Table JSON is not valid: %v\nOutput: %s", err, captured)
	}
	if len(result) != 2 {
		t.Errorf("len(result) = %d, want 2", len(result))
	}
	if result[0]["serial"] != "emulator-5554" {
		t.Errorf("result[0][serial] = %q, want emulator-5554", result[0]["serial"])
	}
}

func TestRenderer_JSON_CheckList(t *testing.T) {
	output.Init(true, false, true)

	captured := captureStdout(func() {
		output.CheckList([]output.CheckItem{
			{Label: "adb found", OK: true, Detail: "/usr/bin/adb"},
			{Label: "SDK not found", OK: false, FixCmds: []string{"acli doctor"}},
		})
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(captured)), &result); err != nil {
		t.Fatalf("CheckList JSON is not valid: %v\nOutput: %s", err, captured)
	}
	checks, ok := result["checks"].([]any)
	if !ok {
		t.Fatalf("expected 'checks' array, got: %v", result)
	}
	if len(checks) != 2 {
		t.Errorf("len(checks) = %d, want 2", len(checks))
	}
}

func TestRenderer_HumanError_ContainsCode(t *testing.T) {
	output.Init(false, false, true) // human mode, no color

	ae := &aclerr.AcliError{
		Code:    aclerr.ErrLicenseNotAccepted,
		Message: "SDK licenses have not been accepted.",
		FixCmds: []string{"acli sdk licenses"},
	}

	captured := captureStderr(func() {
		output.Error(ae)
	})

	if !strings.Contains(captured, "sdk_license_not_accepted") {
		t.Errorf("human output should contain the error code, got: %s", captured)
	}
	if !strings.Contains(captured, "acli sdk licenses") {
		t.Errorf("human output should contain fix command, got: %s", captured)
	}
}

func TestRenderer_NilError(t *testing.T) {
	output.Init(false, false, true)
	// Should not panic
	captured := captureStderr(func() {
		output.Error(nil)
	})
	if captured != "" {
		t.Errorf("nil error should produce no output, got: %q", captured)
	}
}

func TestRenderer_PlainError_WrappedAsUnknown(t *testing.T) {
	output.Init(true, false, true)

	captured := captureStderr(func() {
		output.Error(io.ErrUnexpectedEOF)
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(captured), &result); err != nil {
		t.Fatalf("plain error JSON is not valid: %v\nOutput: %s", err, captured)
	}
	errMap := result["error"].(map[string]any)
	if errMap["code"] != "unknown_error" {
		t.Errorf("plain errors should be wrapped as unknown_error, got: %v", errMap["code"])
	}
}
