// Package project implements template-based Android project scaffolding.
package project

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ErikHellman/android-cli/pkg/aclerr"
	"github.com/ErikHellman/android-cli/pkg/runner"
)

// Service provides project scaffolding operations.
type Service struct{}

// New returns a new project Service.
func New() *Service { return &Service{} }

// Download fetches the latest files from a Git repository into outputDir
// without preserving history. It validates that the result is an Android project.
func (s *Service) Download(ctx context.Context, repoURL, outputDir string) error {
	if _, err := os.Stat(outputDir); err == nil {
		return fmt.Errorf("directory %q already exists", outputDir)
	}

	// Shallow-clone into a temp dir, then move contents (minus .git) to outputDir.
	tmpDir, err := os.MkdirTemp("", "acli-template-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	res, err := runner.RunCapture(ctx, "git", []string{"clone", "--depth=1", repoURL, tmpDir})
	if err != nil {
		return classifyGitError(res, err, repoURL)
	}
	if res.ExitCode != 0 {
		return classifyGitError(res, fmt.Errorf("git clone exited %d", res.ExitCode), repoURL)
	}

	// Remove .git so the user starts clean.
	if err := os.RemoveAll(filepath.Join(tmpDir, ".git")); err != nil {
		return fmt.Errorf("removing .git: %w", err)
	}

	// Move the contents to the real output dir.
	if err := os.Rename(tmpDir, outputDir); err != nil {
		// Cross-device rename: fall back to copy.
		if err := copyDir(tmpDir, outputDir); err != nil {
			return fmt.Errorf("copying template to %s: %w", outputDir, err)
		}
	}

	// Validate it's an Android project.
	if !isAndroidProject(outputDir) {
		return &aclerr.AcliError{
			Code:    aclerr.ErrNotAndroidProject,
			Message: "The repository is not a valid Android project.",
			Detail:  "No settings.gradle or settings.gradle.kts was found in the repository root.",
			FixCmds: []string{"git ls-remote " + repoURL},
		}
	}
	return nil
}

// InitRepo initialises a fresh Git repository in projectDir, adds all files,
// and creates an initial commit.
func (s *Service) InitRepo(ctx context.Context, projectDir, repoURL string) error {
	for _, args := range [][]string{
		{"init"},
		{"add", "-A"},
		{"commit", "-m", fmt.Sprintf("Initial commit from template %s", repoURL)},
	} {
		res, err := runner.Run(ctx, "git", runner.Options{
			Args:    args,
			WorkDir: projectDir,
		})
		if err != nil {
			return fmt.Errorf("git %s: %w", args[0], err)
		}
		if res.ExitCode != 0 {
			return fmt.Errorf("git %s failed: %s", args[0], res.Stderr)
		}
	}
	return nil
}

// RefactorPackage detects the existing base package and rewrites it to newPkg
// across Gradle files, source directories, and source file declarations.
func (s *Service) RefactorPackage(projectDir, newPkg string) error {
	oldPkg, err := detectPackage(projectDir)
	if err != nil {
		return err
	}
	if oldPkg == newPkg {
		return nil
	}

	gradleFiles, err := findGradleFiles(projectDir)
	if err != nil {
		return err
	}

	// Update applicationId and namespace in Gradle files.
	appIDRe := regexp.MustCompile(`(applicationId\s*=?\s*")` + regexp.QuoteMeta(oldPkg) + `"`)
	nsRe := regexp.MustCompile(`(namespace\s*=?\s*")` + regexp.QuoteMeta(oldPkg) + `"`)
	for _, f := range gradleFiles {
		if err := replaceInFile(f, appIDRe, "${1}"+newPkg+`"`); err != nil {
			return err
		}
		if err := replaceInFile(f, nsRe, "${1}"+newPkg+`"`); err != nil {
			return err
		}
	}

	// Move source directories.
	oldPkgPath := strings.ReplaceAll(oldPkg, ".", string(os.PathSeparator))
	newPkgPath := strings.ReplaceAll(newPkg, ".", string(os.PathSeparator))

	sourceRoots, err := findSourceRoots(projectDir)
	if err != nil {
		return err
	}
	for _, root := range sourceRoots {
		if err := movePackageDir(root, oldPkgPath, newPkgPath); err != nil {
			return err
		}
	}

	// Update package declarations and imports in all .kt and .java files.
	pkgDeclRe := regexp.MustCompile(`(?m)^(package\s+)` + regexp.QuoteMeta(oldPkg) + `\b`)
	importRe := regexp.MustCompile(`(import\s+)` + regexp.QuoteMeta(oldPkg) + `\.`)

	err = filepath.WalkDir(projectDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := filepath.Ext(path)
		if ext != ".kt" && ext != ".java" {
			return nil
		}
		if err := replaceInFile(path, pkgDeclRe, "${1}"+newPkg); err != nil {
			return err
		}
		return replaceInFile(path, importRe, "${1}"+newPkg+".")
	})
	if err != nil {
		return &aclerr.AcliError{
			Code:       aclerr.ErrPackageRefactor,
			Message:    "Failed to refactor package declarations.",
			Underlying: err,
		}
	}

	// Clean up empty directories left behind.
	return cleanEmptyDirs(projectDir)
}

// UpdateMinSdk replaces minSdk values in all Gradle build files.
func (s *Service) UpdateMinSdk(projectDir string, version int) error {
	return updateGradleProperty(projectDir, `minSdk`, version)
}

// UpdateTargetSdk replaces targetSdk values in all Gradle build files.
func (s *Service) UpdateTargetSdk(projectDir string, version int) error {
	return updateGradleProperty(projectDir, `targetSdk`, version)
}

// UpdateJavaVersion replaces sourceCompatibility, targetCompatibility, and
// jvmTarget in all Gradle build files.
func (s *Service) UpdateJavaVersion(projectDir, version string) error {
	gradleFiles, err := findGradleFiles(projectDir)
	if err != nil {
		return err
	}

	javaConst := "JavaVersion.VERSION_" + strings.ReplaceAll(version, ".", "_")

	srcCompat := regexp.MustCompile(`(sourceCompatibility\s*=\s*)JavaVersion\.\w+`)
	tgtCompat := regexp.MustCompile(`(targetCompatibility\s*=\s*)JavaVersion\.\w+`)
	jvmTarget := regexp.MustCompile(`(jvmTarget\s*=\s*")([^"]*)(")`)

	for _, f := range gradleFiles {
		if err := replaceInFile(f, srcCompat, "${1}"+javaConst); err != nil {
			return err
		}
		if err := replaceInFile(f, tgtCompat, "${1}"+javaConst); err != nil {
			return err
		}
		if err := replaceInFile(f, jvmTarget, "${1}"+version+"${3}"); err != nil {
			return err
		}
	}
	return nil
}

// ── private helpers ────────────────────────────��─────────────────────────────

func classifyGitError(res *runner.Result, err error, repoURL string) error {
	stderr := ""
	if res != nil {
		stderr = res.Stderr
	}
	if ae := aclerr.Classify("git", stderr); ae != nil {
		ae.Underlying = err
		return ae
	}
	return &aclerr.AcliError{
		Code:       aclerr.ErrCloneFailed,
		Message:    fmt.Sprintf("Failed to clone %s.", repoURL),
		Detail:     stderr,
		Underlying: err,
	}
}

func isAndroidProject(dir string) bool {
	for _, name := range []string{"settings.gradle", "settings.gradle.kts"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

func detectPackage(projectDir string) (string, error) {
	gradleFiles, err := findGradleFiles(projectDir)
	if err != nil {
		return "", err
	}

	nsRe := regexp.MustCompile(`namespace\s*=?\s*"([^"]+)"`)
	appIDRe := regexp.MustCompile(`applicationId\s*=?\s*"([^"]+)"`)

	var fallback string
	for _, f := range gradleFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		content := string(data)
		if m := nsRe.FindStringSubmatch(content); m != nil {
			return m[1], nil
		}
		if fallback == "" {
			if m := appIDRe.FindStringSubmatch(content); m != nil {
				fallback = m[1]
			}
		}
	}
	if fallback != "" {
		return fallback, nil
	}
	return "", &aclerr.AcliError{
		Code:    aclerr.ErrNotAndroidProject,
		Message: "Could not detect the existing package name.",
		Detail:  "No namespace or applicationId found in any build.gradle file.",
	}
}

func findGradleFiles(projectDir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(projectDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if name == "build.gradle" || name == "build.gradle.kts" {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func findSourceRoots(projectDir string) ([]string, error) {
	var roots []string
	err := filepath.WalkDir(projectDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		name := d.Name()
		if name == "java" || name == "kotlin" {
			parent := filepath.Base(filepath.Dir(path))
			grandparent := filepath.Base(filepath.Dir(filepath.Dir(path)))
			if grandparent == "src" || parent == "src" {
				roots = append(roots, path)
			}
		}
		return nil
	})
	return roots, err
}

func replaceInFile(path string, re *regexp.Regexp, repl string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	updated := re.ReplaceAll(data, []byte(repl))
	if string(updated) == string(data) {
		return nil
	}
	return os.WriteFile(path, updated, 0o644)
}

func movePackageDir(sourceRoot, oldPkgPath, newPkgPath string) error {
	oldDir := filepath.Join(sourceRoot, oldPkgPath)
	if _, err := os.Stat(oldDir); os.IsNotExist(err) {
		return nil
	}

	newDir := filepath.Join(sourceRoot, newPkgPath)
	if err := os.MkdirAll(filepath.Dir(newDir), 0o755); err != nil {
		return err
	}

	return os.Rename(oldDir, newDir)
}

func cleanEmptyDirs(root string) error {
	// Walk bottom-up: collect dirs, then remove empty ones in reverse order.
	var dirs []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != root {
			dirs = append(dirs, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	// Reverse to process deepest first.
	for i := len(dirs) - 1; i >= 0; i-- {
		entries, err := os.ReadDir(dirs[i])
		if err != nil {
			continue
		}
		if len(entries) == 0 {
			os.Remove(dirs[i])
		}
	}
	return nil
}

func updateGradleProperty(projectDir, prop string, version int) error {
	gradleFiles, err := findGradleFiles(projectDir)
	if err != nil {
		return err
	}
	re := regexp.MustCompile(prop + `\s*=?\s*\d+`)
	repl := fmt.Sprintf("%s = %d", prop, version)
	for _, f := range gradleFiles {
		if err := replaceInFile(f, re, repl); err != nil {
			return err
		}
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
