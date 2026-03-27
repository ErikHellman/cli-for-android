// Package runner provides a generic subprocess runner for acli.
// It handles timeout, signal forwarding, zombie cleanup, and both
// passthrough (interactive) and capture (parseable) execution modes.
package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// Options configures how a subprocess is run.
type Options struct {
	// Args are the arguments passed to the binary (not including the binary itself).
	Args []string

	// Env variables merged on top of the current process environment.
	// Format: "KEY=VALUE"
	Env []string

	// Stdin is forwarded to the child process stdin. If nil, os.Stdin is used
	// only in PassThrough mode; otherwise /dev/null is used.
	Stdin io.Reader

	// PassThrough streams the child stdout/stderr directly to the terminal
	// instead of capturing them. Use for interactive commands (adb shell,
	// emulator, gradlew with live output).
	PassThrough bool

	// Timeout for the process. Zero means no timeout.
	Timeout time.Duration

	// WorkDir sets the working directory. Defaults to the current directory.
	WorkDir string
}

// Result holds the output of a captured (non-passthrough) run.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

// Run executes binary with opts. In passthrough mode, stdout/stderr stream
// directly to the terminal and Result.Stdout/Stderr will be empty.
func Run(ctx context.Context, binary string, opts Options) (*Result, error) {
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, binary, opts.Args...) //nolint:gosec

	// Environment
	cmd.Env = append(os.Environ(), opts.Env...)

	// Working directory
	if opts.WorkDir != "" {
		cmd.Dir = opts.WorkDir
	}

	var stdoutBuf, stderrBuf bytes.Buffer

	if opts.PassThrough {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if opts.Stdin != nil {
			cmd.Stdin = opts.Stdin
		} else {
			cmd.Stdin = os.Stdin
		}
	} else {
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf
		if opts.Stdin != nil {
			cmd.Stdin = opts.Stdin
		}
	}

	// Forward signals to the child so Ctrl+C reaches it cleanly
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start %q: %w", binary, err)
	}

	// Ensure child is reaped even on panic / early return
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	// Signal forwarder goroutine
	go func() {
		for sig := range sigCh {
			if cmd.Process != nil {
				_ = cmd.Process.Signal(sig)
			}
		}
	}()

	err := cmd.Wait()
	elapsed := time.Since(start)
	close(sigCh)

	// Check context error first: a cancelled/timed-out context kills the child
	// process, which surfaces as an ExitError(-1). We want the context error.
	if err != nil && ctx.Err() != nil {
		return nil, fmt.Errorf("process timed out after %s", opts.Timeout)
	}

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if isExitError(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, err
		}
	}

	return &Result{
		Stdout:   strings.TrimRight(stdoutBuf.String(), "\n"),
		Stderr:   strings.TrimRight(stderrBuf.String(), "\n"),
		ExitCode: exitCode,
		Duration: elapsed,
	}, nil
}

// RunCapture is a shorthand for Run with PassThrough=false.
func RunCapture(ctx context.Context, binary string, args []string) (*Result, error) {
	return Run(ctx, binary, Options{Args: args})
}

// RunPassThrough is a shorthand for Run with PassThrough=true. Returns only the error.
func RunPassThrough(ctx context.Context, binary string, args []string) error {
	_, err := Run(ctx, binary, Options{Args: args, PassThrough: true})
	return err
}

// RunWith runs a command with full Options and returns only the error, discarding the Result.
// Prefer this over Run when output is streamed via PassThrough and the Result is not needed.
func RunWith(ctx context.Context, binary string, opts Options) error {
	_, err := Run(ctx, binary, opts)
	return err
}

// RunWithStdin runs a command with the given stdin reader (captured mode).
func RunWithStdin(ctx context.Context, binary string, args []string, stdin io.Reader) (*Result, error) {
	return Run(ctx, binary, Options{Args: args, Stdin: stdin})
}

// isExitError returns true and fills exitErr if err wraps an *exec.ExitError.
func isExitError(err error, exitErr **exec.ExitError) bool {
	var ee *exec.ExitError
	if ok := false; !ok {
		if e, ok2 := err.(*exec.ExitError); ok2 { //nolint:errorlint
			*exitErr = e
			return true
		}
	}
	_ = ee
	// Try unwrapping
	type unwrapper interface{ Unwrap() error }
	if u, ok := err.(unwrapper); ok {
		return isExitError(u.Unwrap(), exitErr)
	}
	return false
}
