package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/android-cli/acli/pkg/output"
	"github.com/spf13/cobra"
)

// skillTemplate is the Claude Code SKILL.md content installed by `acli skills install`.
// It is also embedded in assets/skills/acli/SKILL.md for reference.
const skillTemplate = `---
name: acli
description: Control the Android development environment: manage SDK packages, start/stop emulators, install APKs, stream device logs, take screenshots, and run Gradle builds. Use when the user wants to do anything with Android devices, emulators, or SDK tools.
allowed-tools: Bash(acli *)
disable-model-invocation: false
---

You have access to the ` + "`acli`" + ` command, which provides a unified interface to all Android development tools.

## Device & Emulator Management

` + "```" + `bash
acli device list --json                          # list devices/emulators with serials
acli device -d <serial> info                     # model, Android version, API level, ABI
acli device -d <serial> install path/to/app.apk  # install APK
acli device -d <serial> logs --follow --level E  # stream error logs
acli device -d <serial> screenshot screen.png    # capture screen
acli avd list --json                             # list AVDs
acli avd start <name> --headless --wait-boot     # start emulator, wait for boot
acli avd stop <serial>                           # stop a running emulator
` + "```" + `

## SDK Management

` + "```" + `bash
acli sdk list --installed --json                 # list installed packages
acli sdk list --updates                          # list packages with updates
acli sdk install "platforms;android-35"          # install a package
acli sdk licenses                                # accept all pending licenses
` + "```" + `

## App Management

` + "```" + `bash
acli app list --json                             # list all installed apps
acli app launch <package>                        # launch an app
acli app stop <package>                          # force-stop an app
acli app clear <package>                         # clear app data
acli app deep-link <uri>                         # open a URI deep link
` + "```" + `

## Build

` + "```" + `bash
acli build assemble --variant debug              # build debug APK
acli build test --unit                           # run unit tests
acli build clean                                 # clean build outputs
` + "```" + `

## Device Instrumentation

` + "```" + `bash
acli instrument battery --level 10 --status discharging
acli instrument network --speed edge --latency gprs
acli instrument location --lat 37.7749 --lng -122.4194
acli instrument input text "Hello World"
acli instrument input tap 540 960
` + "```" + `

## Environment Health

` + "```" + `bash
acli doctor --json                               # full environment check (parseable)
` + "```" + `

## JSON Output

Always pass ` + "`--json`" + ` when you need to parse results programmatically.
All errors are written to stderr as:
` + "```" + `json
{"error": {"code": "...", "message": "...", "detail": "...", "fix": ["cmd1", "cmd2"]}}
` + "```" + `

## Device Targeting

Use ` + "`--device <serial>`" + ` or set ` + "`ACLI_DEVICE=<serial>`" + ` to target a specific device.
When multiple devices are connected, always specify a target to avoid ambiguity.
`

func newSkillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Install AI agent skills for Claude Code and other AI tools",
	}
	cmd.AddCommand(
		newSkillsInstallCmd(),
		newSkillsListCmd(),
	)
	return cmd
}

func newSkillsInstallCmd() *cobra.Command {
	var flagScope string

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the acli Claude Code skill",
		Long: `Installs a Claude Code skill file (SKILL.md) that teaches Claude how to
control the Android environment using acli commands.

Project scope: .claude/skills/acli/SKILL.md  (committed to the project)
User scope:    ~/.claude/skills/acli/SKILL.md (available in all your projects)`,
		RunE: func(_ *cobra.Command, _ []string) error {
			dir, err := skillDir(flagScope)
			if err != nil {
				return handleErr(err)
			}

			if err := os.MkdirAll(dir, 0o755); err != nil {
				return handleErr(fmt.Errorf("creating skill directory: %w", err))
			}

			dest := filepath.Join(dir, "SKILL.md")
			if err := os.WriteFile(dest, []byte(skillTemplate), 0o644); err != nil {
				return handleErr(fmt.Errorf("writing skill file: %w", err))
			}

			output.Success("Skill installed at %s", dest)
			output.Println("")
			output.Println("  Claude Code will now automatically use acli to control your Android environment.")
			output.Println("  You can also invoke it directly with /acli in Claude Code.")
			return nil
		},
	}
	cmd.Flags().StringVar(&flagScope, "scope", "project", "Installation scope: project or user")
	return cmd
}

func newSkillsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed acli skills",
		RunE: func(_ *cobra.Command, _ []string) error {
			locations := []struct{ scope, path string }{
				{"project", filepath.Join(".claude", "skills", "acli", "SKILL.md")},
			}
			home, _ := os.UserHomeDir()
			if home != "" {
				locations = append(locations, struct{ scope, path string }{
					"user", filepath.Join(home, ".claude", "skills", "acli", "SKILL.md"),
				})
			}

			headers := []string{"Scope", "Path", "Installed"}
			var rows [][]string
			for _, loc := range locations {
				installed := "no"
				if _, err := os.Stat(loc.path); err == nil {
					installed = "yes"
				}
				rows = append(rows, []string{loc.scope, loc.path, installed})
			}
			output.Table(headers, rows)
			return nil
		},
	}
}

// skillDir returns the directory where the SKILL.md should be installed.
func skillDir(scope string) (string, error) {
	switch scope {
	case "user":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".claude", "skills", "acli"), nil
	case "project", "":
		return filepath.Join(".claude", "skills", "acli"), nil
	default:
		return "", fmt.Errorf("unknown scope %q: use 'project' or 'user'", scope)
	}
}
