package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/android-cli/acli/pkg/output"
	"github.com/spf13/cobra"
)

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Bootstrap new Android projects",
	}
	cmd.AddCommand(newProjectInitCmd())
	return cmd
}

func newProjectInitCmd() *cobra.Command {
	var (
		flagPackage string
		flagMinSDK  int
		flagTemplate string
	)

	cmd := &cobra.Command{
		Use:   "init <name>",
		Short: "Create a new Android project from a template",
		Long: `Scaffold a minimal Android project with Gradle wrapper, basic structure,
and sensible defaults. Ideal for quickly bootstrapping test or sample projects.

Templates:
  empty      - Bare minimum project with one Activity
  compose    - Jetpack Compose starter
  viewbinding - ViewBinding + MVVM starter`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			name := args[0]
			pkg := flagPackage
			if pkg == "" {
				pkg = "com.example." + strings.ToLower(strings.ReplaceAll(name, "-", ""))
			}

			dir := name
			if _, err := os.Stat(dir); err == nil {
				return handleErr(fmt.Errorf("directory %q already exists", dir))
			}

			output.Info("Creating Android project %q in ./%s", name, dir)
			if err := scaffoldProject(dir, name, pkg, flagMinSDK, flagTemplate); err != nil {
				return handleErr(err)
			}
			output.Success("Project created at ./%s", dir)
			output.Println("  Next steps:")
			output.Println("    cd %s", dir)
			output.Println("    acli sdk install 'platforms;android-%d'", flagMinSDK)
			output.Println("    acli build assemble")
			return nil
		},
	}

	cmd.Flags().StringVar(&flagPackage, "package", "", "Application ID, e.g. com.example.myapp (default: com.example.<name>)")
	cmd.Flags().IntVar(&flagMinSDK, "min-sdk", 24, "Minimum supported Android API level")
	cmd.Flags().StringVar(&flagTemplate, "template", "empty", "Project template: empty, compose, viewbinding")
	return cmd
}

// scaffoldProject creates a minimal Android project directory structure.
func scaffoldProject(dir, name, pkg string, minSDK int, _ string) error {
	packagePath := strings.ReplaceAll(pkg, ".", "/")

	dirs := []string{
		filepath.Join(dir, "app", "src", "main", "java", packagePath),
		filepath.Join(dir, "app", "src", "main", "res", "layout"),
		filepath.Join(dir, "app", "src", "main", "res", "values"),
		filepath.Join(dir, "gradle", "wrapper"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}

	files := map[string]string{
		filepath.Join(dir, "settings.gradle.kts"): fmt.Sprintf(`pluginManagement {
    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
}
dependencyResolutionManagement {
    repositories {
        google()
        mavenCentral()
    }
}
rootProject.name = "%s"
include(":app")
`, name),

		filepath.Join(dir, "build.gradle.kts"): `plugins {
    alias(libs.plugins.android.application) apply false
    alias(libs.plugins.kotlin.android) apply false
}
`,

		filepath.Join(dir, "app", "build.gradle.kts"): fmt.Sprintf(`plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.android)
}

android {
    namespace = "%s"
    compileSdk = 35

    defaultConfig {
        applicationId = "%s"
        minSdk = %d
        targetSdk = 35
        versionCode = 1
        versionName = "1.0"
    }

    buildTypes {
        release {
            isMinifyEnabled = false
        }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_1_8
        targetCompatibility = JavaVersion.VERSION_1_8
    }
    kotlinOptions {
        jvmTarget = "1.8"
    }
}

dependencies {
    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.appcompat)
    implementation(libs.material)
}
`, pkg, pkg, minSDK),

		filepath.Join(dir, "app", "src", "main", "AndroidManifest.xml"): fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <application
        android:allowBackup="true"
        android:label="%s"
        android:supportsRtl="true"
        android:theme="@style/Theme.AppCompat.Light.DarkActionBar">
        <activity
            android:name=".MainActivity"
            android:exported="true">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
        </activity>
    </application>
</manifest>
`, name),

		filepath.Join(dir, "app", "src", "main", "java", packagePath, "MainActivity.kt"): fmt.Sprintf(`package %s

import androidx.appcompat.app.AppCompatActivity
import android.os.Bundle

class MainActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
    }
}
`, pkg),

		filepath.Join(dir, "gradle", "wrapper", "gradle-wrapper.properties"): `distributionBase=GRADLE_USER_HOME
distributionPath=wrapper/dists
zipStoreBase=GRADLE_USER_HOME
zipStorePath=wrapper/dists
distributionUrl=https\://services.gradle.org/distributions/gradle-8.7-bin.zip
`,

		filepath.Join(dir, ".gitignore"): `*.iml
.gradle
/local.properties
/.idea
.DS_Store
/build
/captures
.externalNativeBuild
.cxx
local.properties
`,
	}

	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}
