package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/android-cli/acli/pkg/android"
	"github.com/android-cli/acli/pkg/output"
	"github.com/android-cli/acli/pkg/runner"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check the Android development environment",
		Long: `Validates that all required tools and environment variables are correctly
configured. Use this to diagnose setup problems.

Checks:
  • ANDROID_HOME / ANDROID_SDK_ROOT environment variable
  • Java (required by Gradle and sdkmanager)
  • adb, sdkmanager, avdmanager, emulator binaries
  • SDK license acceptance status
  • Connected devices`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			items := runChecks(cmd.Context())
			output.CheckList(items)

			// Exit with code 4 if any critical check failed
			for _, it := range items {
				if !it.OK && it.Label != "Connected devices" {
					return fmt.Errorf("one or more environment checks failed")
				}
			}
			return nil
		},
	}
}

func runChecks(ctx context.Context) []output.CheckItem {
	var checks []output.CheckItem

	// ── 1. ANDROID_HOME ──────────────────────────────────────────────────
	androidHome := os.Getenv("ANDROID_HOME")
	if androidHome == "" {
		androidHome = os.Getenv("ANDROID_SDK_ROOT")
	}
	if androidHome != "" {
		if fi, err := os.Stat(androidHome); err == nil && fi.IsDir() {
			checks = append(checks, output.CheckItem{
				Label:  "ANDROID_HOME is set",
				OK:     true,
				Detail: androidHome,
			})
		} else {
			checks = append(checks, output.CheckItem{
				Label:   "ANDROID_HOME is set but directory does not exist",
				OK:      false,
				Detail:  androidHome,
				FixCmds: []string{"export ANDROID_HOME=/path/to/your/android/sdk"},
			})
		}
	} else {
		checks = append(checks, output.CheckItem{
			Label: "$ANDROID_HOME is not set",
			OK:    false,
			Detail: "The Android SDK root directory is not configured. " +
				"On macOS it is usually ~/Library/Android/sdk",
			FixCmds: []string{
				`export ANDROID_HOME=~/Library/Android/sdk`,
				`echo 'export ANDROID_HOME=~/Library/Android/sdk' >> ~/.zshrc`,
			},
		})
	}

	// ── 2. Java ──────────────────────────────────────────────────────────
	javaPath, javaErr := exec.LookPath("java")
	if javaErr == nil {
		javaVer := "unknown"
		if res, err := runner.RunCapture(ctx, javaPath, []string{"-version"}); err == nil {
			// java -version outputs to stderr
			combined := res.Stdout + res.Stderr
			for _, line := range strings.Split(combined, "\n") {
				if strings.Contains(line, "version") {
					javaVer = strings.TrimSpace(line)
					break
				}
			}
		}
		checks = append(checks, output.CheckItem{
			Label:  "Java is installed",
			OK:     true,
			Detail: javaVer,
		})
	} else {
		checks = append(checks, output.CheckItem{
			Label:  "Java not found in PATH",
			OK:     false,
			Detail: "Java is required by Gradle and sdkmanager.",
			FixCmds: []string{
				"brew install --cask zulu@17",
				"# or install from https://adoptium.net",
			},
		})
	}

	// ── 3. SDK binaries ──────────────────────────────────────────────────
	loc, locErr := android.New()

	binaries := []struct {
		name    string
		fix     []string
		isFatal bool
	}{
		{"adb", []string{"acli sdk install platform-tools"}, true},
		{"sdkmanager", []string{"# Download cmdline-tools from https://developer.android.com/tools"}, true},
		{"avdmanager", []string{"acli sdk install cmdline-tools"}, false},
		{"emulator", []string{"acli sdk install emulator"}, false},
		{"fastboot", []string{"acli sdk install platform-tools"}, false},
	}

	for _, b := range binaries {
		if locErr != nil {
			checks = append(checks, output.CheckItem{
				Label:   fmt.Sprintf("%s binary", b.name),
				OK:      false,
				Detail:  "SDK not found; cannot locate binaries.",
				FixCmds: []string{"acli doctor (fix ANDROID_HOME first)"},
			})
			continue
		}
		path, err := loc.Binary(b.name)
		if err == nil {
			checks = append(checks, output.CheckItem{
				Label:  fmt.Sprintf("%s found", b.name),
				OK:     true,
				Detail: path,
			})
		} else {
			checks = append(checks, output.CheckItem{
				Label:   fmt.Sprintf("%s not found", b.name),
				OK:      false,
				FixCmds: b.fix,
			})
		}
	}

	// ── 4. SDK licenses ──────────────────────────────────────────────────
	if loc != nil {
		sdkMgr, err := loc.Binary("sdkmanager")
		if err == nil {
			res, _ := runner.RunCapture(ctx, sdkMgr, []string{"--licenses"})
			combined := res.Stdout + res.Stderr
			if strings.Contains(strings.ToLower(combined), "all sdk package licenses accepted") ||
				!strings.Contains(strings.ToLower(combined), "accept") {
				checks = append(checks, output.CheckItem{
					Label: "SDK licenses accepted",
					OK:    true,
				})
			} else {
				checks = append(checks, output.CheckItem{
					Label:   "SDK licenses not fully accepted",
					OK:      false,
					FixCmds: []string{"acli sdk licenses"},
				})
			}
		}
	}

	// ── 5. ADB server + connected devices ────────────────────────────────
	if loc != nil {
		adb, err := loc.Binary("adb")
		if err == nil {
			res, err := runner.RunCapture(ctx, adb, []string{"devices", "-l"})
			if err != nil || res.ExitCode != 0 {
				checks = append(checks, output.CheckItem{
					Label:   "ADB server",
					OK:      false,
					FixCmds: []string{"adb start-server"},
				})
			} else {
				checks = append(checks, output.CheckItem{
					Label: "ADB server running",
					OK:    true,
				})

				// Count connected devices (lines after "List of devices attached")
				count := 0
				lines := strings.Split(res.Stdout, "\n")
				for _, line := range lines[1:] {
					if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "*") {
						count++
					}
				}
				checks = append(checks, output.CheckItem{
					Label:  "Connected devices",
					OK:     count > 0,
					Detail: fmt.Sprintf("%d device(s) connected", count),
					FixCmds: func() []string {
						if count == 0 {
							return []string{"acli avd start <avd-name>", "# or plug in a physical device"}
						}
						return nil
					}(),
				})
			}
		}
	}

	return checks
}
