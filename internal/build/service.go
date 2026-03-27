// Package build wraps the Gradle wrapper for Android project builds.
package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/android-cli/acli/pkg/aclerr"
	"github.com/android-cli/acli/pkg/runner"
)

// Service wraps Gradle wrapper operations.
type Service struct{}

// New creates a new Service.
func New() *Service { return &Service{} }

// FindProjectRoot walks up from cwd looking for settings.gradle / settings.gradle.kts.
func (s *Service) FindProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		for _, f := range []string{"settings.gradle", "settings.gradle.kts", "build.gradle", "build.gradle.kts"} {
			if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", &aclerr.AcliError{
		Code:    aclerr.ErrGradleNotFound,
		Message: "No Android project found in the current directory tree.",
		Detail:  "Could not find settings.gradle or build.gradle in the current directory or any parent.",
		FixCmds: []string{"cd /path/to/your/android/project", "acli project init <name>"},
	}
}

// gradlew returns the path to the Gradle wrapper in root.
func gradlew(root string) (string, error) {
	name := "gradlew"
	if runtime.GOOS == "windows" {
		name = "gradlew.bat"
	}
	p := filepath.Join(root, name)
	if _, err := os.Stat(p); err != nil {
		return "", &aclerr.AcliError{
			Code:    aclerr.ErrGradleNotFound,
			Message: "gradlew not found in project root.",
			Detail:  "The Gradle wrapper script is missing. Re-generate it with `gradle wrapper` or check out the project from version control.",
			FixCmds: []string{"gradle wrapper --gradle-version 8.7"},
		}
	}
	return p, nil
}

// Assemble builds the project.
func (s *Service) Assemble(ctx context.Context, variant, module string) error {
	return s.runGradle(ctx, gradleTask("assemble", variant, module), nil)
}

// Test runs tests.
func (s *Service) Test(ctx context.Context, unit, instrumented bool, module string) error {
	if unit || (!unit && !instrumented) {
		if err := s.runGradle(ctx, gradleTask("test", "", module), nil); err != nil {
			return err
		}
	}
	if instrumented {
		if err := s.runGradle(ctx, gradleTask("connectedAndroidTest", "", module), nil); err != nil {
			return err
		}
	}
	return nil
}

// Clean cleans the build.
func (s *Service) Clean(ctx context.Context) error {
	return s.runGradle(ctx, "clean", nil)
}

// Lint runs lint.
func (s *Service) Lint(ctx context.Context, module string, fix bool) error {
	task := gradleTask("lint", "", module)
	if fix {
		task = gradleTask("lintFix", "", module)
	}
	return s.runGradle(ctx, task, nil)
}

// Bundle builds an App Bundle.
func (s *Service) Bundle(ctx context.Context, variant, module string) error {
	return s.runGradle(ctx, gradleTask("bundle", variant, module), nil)
}

// RunTask runs an arbitrary Gradle task.
func (s *Service) RunTask(ctx context.Context, task string, extraArgs []string) error {
	return s.runGradle(ctx, task, extraArgs)
}

// ── helpers ───────────────────────────────────────────────────────────────

func (s *Service) runGradle(ctx context.Context, task string, extra []string) error {
	root, err := s.FindProjectRoot()
	if err != nil {
		return err
	}
	gw, err := gradlew(root)
	if err != nil {
		return err
	}

	args := []string{task}
	args = append(args, extra...)

	err = runner.RunWith(ctx, gw, runner.Options{
		Args:        args,
		WorkDir:     root,
		PassThrough: true,
	})
	if err != nil {
		if ae := aclerr.Classify("gradle", err.Error()); ae != nil {
			return ae
		}
		return fmt.Errorf("gradle %s: %w", task, err)
	}
	return nil
}

// gradleTask builds a task name like ":app:assembleDebug".
func gradleTask(base, variant, module string) string {
	v := strings.Title(strings.ToLower(variant)) //nolint:staticcheck
	task := base + v
	if module != "" {
		if !strings.HasPrefix(module, ":") {
			module = ":" + module
		}
		return module + ":" + task
	}
	return task
}
