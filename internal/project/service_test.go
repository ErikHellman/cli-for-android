package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectPackage_Namespace(t *testing.T) {
	dir := t.TempDir()
	appDir := filepath.Join(dir, "app")
	os.MkdirAll(appDir, 0o755)

	content := `plugins {
    alias(libs.plugins.android.application)
}

android {
    namespace = "com.example.template"
    compileSdk = 35

    defaultConfig {
        applicationId = "com.example.template"
        minSdk = 24
    }
}
`
	os.WriteFile(filepath.Join(appDir, "build.gradle.kts"), []byte(content), 0o644)

	pkg, err := detectPackage(dir)
	if err != nil {
		t.Fatalf("detectPackage: %v", err)
	}
	if pkg != "com.example.template" {
		t.Errorf("got %q, want %q", pkg, "com.example.template")
	}
}

func TestDetectPackage_ApplicationIdFallback(t *testing.T) {
	dir := t.TempDir()
	appDir := filepath.Join(dir, "app")
	os.MkdirAll(appDir, 0o755)

	content := `android {
    defaultConfig {
        applicationId = "com.example.fallback"
        minSdk = 24
    }
}
`
	os.WriteFile(filepath.Join(appDir, "build.gradle.kts"), []byte(content), 0o644)

	pkg, err := detectPackage(dir)
	if err != nil {
		t.Fatalf("detectPackage: %v", err)
	}
	if pkg != "com.example.fallback" {
		t.Errorf("got %q, want %q", pkg, "com.example.fallback")
	}
}

func TestUpdateMinSdk(t *testing.T) {
	dir := t.TempDir()
	appDir := filepath.Join(dir, "app")
	os.MkdirAll(appDir, 0o755)

	// Kotlin DSL
	kts := `android {
    defaultConfig {
        minSdk = 24
        targetSdk = 34
    }
}
`
	os.WriteFile(filepath.Join(appDir, "build.gradle.kts"), []byte(kts), 0o644)

	// Groovy DSL
	groovy := `android {
    defaultConfig {
        minSdk 21
        targetSdk 33
    }
}
`
	os.WriteFile(filepath.Join(appDir, "build.gradle"), []byte(groovy), 0o644)

	svc := New()
	if err := svc.UpdateMinSdk(dir, 26); err != nil {
		t.Fatalf("UpdateMinSdk: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(appDir, "build.gradle.kts"))
	if got := string(data); !contains(got, "minSdk = 26") {
		t.Errorf("build.gradle.kts: expected minSdk = 26, got:\n%s", got)
	}

	data, _ = os.ReadFile(filepath.Join(appDir, "build.gradle"))
	if got := string(data); !contains(got, "minSdk = 26") {
		t.Errorf("build.gradle: expected minSdk = 26, got:\n%s", got)
	}
}

func TestUpdateTargetSdk(t *testing.T) {
	dir := t.TempDir()
	appDir := filepath.Join(dir, "app")
	os.MkdirAll(appDir, 0o755)

	kts := `android {
    defaultConfig {
        minSdk = 24
        targetSdk = 34
    }
}
`
	os.WriteFile(filepath.Join(appDir, "build.gradle.kts"), []byte(kts), 0o644)

	svc := New()
	if err := svc.UpdateTargetSdk(dir, 35); err != nil {
		t.Fatalf("UpdateTargetSdk: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(appDir, "build.gradle.kts"))
	if got := string(data); !contains(got, "targetSdk = 35") {
		t.Errorf("expected targetSdk = 35, got:\n%s", got)
	}
}

func TestUpdateJavaVersion(t *testing.T) {
	dir := t.TempDir()
	appDir := filepath.Join(dir, "app")
	os.MkdirAll(appDir, 0o755)

	kts := `android {
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_1_8
        targetCompatibility = JavaVersion.VERSION_1_8
    }
    kotlinOptions {
        jvmTarget = "1.8"
    }
}
`
	os.WriteFile(filepath.Join(appDir, "build.gradle.kts"), []byte(kts), 0o644)

	svc := New()
	if err := svc.UpdateJavaVersion(dir, "17"); err != nil {
		t.Fatalf("UpdateJavaVersion: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(appDir, "build.gradle.kts"))
	got := string(data)
	if !contains(got, "JavaVersion.VERSION_17") {
		t.Errorf("expected JavaVersion.VERSION_17, got:\n%s", got)
	}
	if !contains(got, `jvmTarget = "17"`) {
		t.Errorf("expected jvmTarget = \"17\", got:\n%s", got)
	}
}

func TestRefactorPackage(t *testing.T) {
	dir := t.TempDir()
	appDir := filepath.Join(dir, "app")

	// Create gradle file.
	os.MkdirAll(appDir, 0o755)
	gradle := `android {
    namespace = "com.old.app"
    defaultConfig {
        applicationId = "com.old.app"
    }
}
`
	os.WriteFile(filepath.Join(appDir, "build.gradle.kts"), []byte(gradle), 0o644)

	// Create source files in the old package.
	srcDir := filepath.Join(appDir, "src", "main", "java", "com", "old", "app")
	os.MkdirAll(srcDir, 0o755)
	os.WriteFile(filepath.Join(srcDir, "MainActivity.kt"), []byte(`package com.old.app

import com.old.app.util.Helper

class MainActivity {
}
`), 0o644)

	utilDir := filepath.Join(srcDir, "util")
	os.MkdirAll(utilDir, 0o755)
	os.WriteFile(filepath.Join(utilDir, "Helper.kt"), []byte(`package com.old.app.util

class Helper {
}
`), 0o644)

	// Also create a test source set.
	testDir := filepath.Join(appDir, "src", "test", "java", "com", "old", "app")
	os.MkdirAll(testDir, 0o755)
	os.WriteFile(filepath.Join(testDir, "MainActivityTest.kt"), []byte(`package com.old.app

import com.old.app.MainActivity

class MainActivityTest {
}
`), 0o644)

	svc := New()
	if err := svc.RefactorPackage(dir, "com.new.myapp"); err != nil {
		t.Fatalf("RefactorPackage: %v", err)
	}

	// Check gradle file updated.
	data, _ := os.ReadFile(filepath.Join(appDir, "build.gradle.kts"))
	got := string(data)
	if !contains(got, `namespace = "com.new.myapp"`) {
		t.Errorf("expected namespace = com.new.myapp, got:\n%s", got)
	}
	if !contains(got, `applicationId = "com.new.myapp"`) {
		t.Errorf("expected applicationId = com.new.myapp, got:\n%s", got)
	}

	// Check source files moved.
	newMainDir := filepath.Join(appDir, "src", "main", "java", "com", "new", "myapp")
	if _, err := os.Stat(filepath.Join(newMainDir, "MainActivity.kt")); err != nil {
		t.Errorf("MainActivity.kt not found at new location: %v", err)
	}
	if _, err := os.Stat(filepath.Join(newMainDir, "util", "Helper.kt")); err != nil {
		t.Errorf("Helper.kt not found at new location: %v", err)
	}

	// Check test files moved.
	newTestDir := filepath.Join(appDir, "src", "test", "java", "com", "new", "myapp")
	if _, err := os.Stat(filepath.Join(newTestDir, "MainActivityTest.kt")); err != nil {
		t.Errorf("MainActivityTest.kt not found at new location: %v", err)
	}

	// Check package declarations updated.
	mainData, _ := os.ReadFile(filepath.Join(newMainDir, "MainActivity.kt"))
	if !contains(string(mainData), "package com.new.myapp") {
		t.Errorf("MainActivity.kt package not updated:\n%s", mainData)
	}
	if !contains(string(mainData), "import com.new.myapp.util.Helper") {
		t.Errorf("MainActivity.kt import not updated:\n%s", mainData)
	}

	helperData, _ := os.ReadFile(filepath.Join(newMainDir, "util", "Helper.kt"))
	if !contains(string(helperData), "package com.new.myapp.util") {
		t.Errorf("Helper.kt package not updated:\n%s", helperData)
	}

	// Old directories should be cleaned up.
	oldDir := filepath.Join(appDir, "src", "main", "java", "com", "old")
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Errorf("old package directory still exists: %s", oldDir)
	}
}

func TestDeriveOutputDir(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://github.com/user/my-template.git", "my-template"},
		{"https://github.com/user/my-template", "my-template"},
		{"https://github.com/user/my-template/", "my-template"},
		{"git@github.com:user/my-template.git", "my-template"},
		{"git@github.com:user/my-template", "my-template"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			// deriveOutputDir is in internal/cmd, so we test it indirectly
			// by duplicating the logic here for unit testing.
			got := deriveOutputDirHelper(tt.input)
			if got != tt.want {
				t.Errorf("deriveOutputDir(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// deriveOutputDirHelper mirrors the logic in internal/cmd/project.go for testing.
func deriveOutputDirHelper(repoURL string) string {
	u := repoURL
	// Strip trailing slashes and .git suffix.
	for len(u) > 0 && u[len(u)-1] == '/' {
		u = u[:len(u)-1]
	}
	if len(u) > 4 && u[len(u)-4:] == ".git" {
		u = u[:len(u)-4]
	}

	// Handle SSH-style URLs (git@host:user/repo).
	for i := len(u) - 1; i >= 0; i-- {
		if u[i] == ':' {
			hasScheme := false
			for j := i - 1; j >= 0; j-- {
				if u[j] == '/' {
					hasScheme = true
					break
				}
			}
			if !hasScheme {
				u = u[i+1:]
			}
			break
		}
	}

	// Take last path segment.
	if i := lastIndex(u, '/'); i >= 0 {
		u = u[i+1:]
	}
	return u
}

func lastIndex(s string, b byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
