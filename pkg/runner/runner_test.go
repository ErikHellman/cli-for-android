package runner_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/android-cli/acli/pkg/runner"
)

func TestRunCapture_Echo(t *testing.T) {
	res, err := runner.RunCapture(context.Background(), "echo", []string{"hello world"})
	if err != nil {
		t.Fatalf("RunCapture: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", res.ExitCode)
	}
	if !strings.Contains(res.Stdout, "hello world") {
		t.Errorf("Stdout = %q, want to contain 'hello world'", res.Stdout)
	}
}

func TestRunCapture_NonZeroExit(t *testing.T) {
	res, err := runner.RunCapture(context.Background(), "sh", []string{"-c", "exit 42"})
	if err != nil {
		t.Fatalf("RunCapture returned unexpected error: %v", err)
	}
	if res.ExitCode != 42 {
		t.Errorf("ExitCode = %d, want 42", res.ExitCode)
	}
}

func TestRunCapture_Stderr(t *testing.T) {
	res, err := runner.RunCapture(context.Background(), "sh", []string{"-c", "echo oops >&2; exit 1"})
	if err != nil {
		t.Fatalf("RunCapture: %v", err)
	}
	if !strings.Contains(res.Stderr, "oops") {
		t.Errorf("Stderr = %q, want to contain 'oops'", res.Stderr)
	}
	if res.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", res.ExitCode)
	}
}

func TestRunCapture_Env(t *testing.T) {
	res, err := runner.Run(context.Background(), "sh", runner.Options{
		Args: []string{"-c", "echo $ACLI_TEST_VAR"},
		Env:  []string{"ACLI_TEST_VAR=hello_from_env"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(res.Stdout, "hello_from_env") {
		t.Errorf("Stdout = %q, want 'hello_from_env'", res.Stdout)
	}
}

func TestRunCapture_Stdin(t *testing.T) {
	res, err := runner.RunWithStdin(
		context.Background(),
		"cat",
		nil,
		strings.NewReader("stdin content"),
	)
	if err != nil {
		t.Fatalf("RunWithStdin: %v", err)
	}
	if !strings.Contains(res.Stdout, "stdin content") {
		t.Errorf("Stdout = %q, want 'stdin content'", res.Stdout)
	}
}

func TestRunCapture_Timeout(t *testing.T) {
	ctx := context.Background()
	start := time.Now()
	_, err := runner.Run(ctx, "sleep", runner.Options{
		Args:    []string{"10"},
		Timeout: 100 * time.Millisecond,
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if elapsed > 2*time.Second {
		t.Errorf("timeout took too long: %s", elapsed)
	}
}

func TestRunCapture_BinaryNotFound(t *testing.T) {
	_, err := runner.RunCapture(context.Background(), "/nonexistent/binary/acli-fake", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent binary, got nil")
	}
}

func TestRunCapture_WorkDir(t *testing.T) {
	tmp := t.TempDir()
	res, err := runner.Run(context.Background(), "pwd", runner.Options{
		WorkDir: tmp,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// On macOS, /tmp may resolve to /private/tmp via symlinks
	realTmp, _ := filepath.EvalSymlinks(tmp)
	if !strings.Contains(res.Stdout, realTmp) && !strings.Contains(res.Stdout, tmp) {
		t.Errorf("Stdout = %q, want it to contain %q", res.Stdout, tmp)
	}
}

func TestRunCapture_Duration(t *testing.T) {
	res, err := runner.RunCapture(context.Background(), "true", nil)
	if err != nil {
		t.Fatalf("RunCapture: %v", err)
	}
	if res.Duration <= 0 {
		t.Errorf("Duration = %s, want > 0", res.Duration)
	}
}
