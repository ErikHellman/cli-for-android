package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/android-cli/acli/pkg/android"
	"github.com/android-cli/acli/pkg/output"
	"github.com/android-cli/acli/pkg/runner"
	"github.com/spf13/cobra"
)

func newAppCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "app",
		Short: "Manage installed applications on a device",
	}
	cmd.AddCommand(
		newAppListCmd(),
		newAppClearCmd(),
		newAppGrantCmd(),
		newAppRevokeCmd(),
		newAppLaunchCmd(),
		newAppStopCmd(),
		newAppDeepLinkCmd(),
	)
	return cmd
}

func newAppListCmd() *cobra.Command {
	var flagSystem, flagThirdParty bool
	var flagFilter string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed apps on the device",
		RunE: func(cmd *cobra.Command, _ []string) error {
			pkgs, err := pmList(cmd.Context(), flagSystem, flagThirdParty, flagFilter)
			if err != nil {
				return handleErr(err)
			}
			if len(pkgs) == 0 {
				output.Info("No packages found.")
				return nil
			}
			headers := []string{"Package"}
			var rows [][]string
			for _, p := range pkgs {
				rows = append(rows, []string{p})
			}
			output.Table(headers, rows)
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagSystem, "system", false, "Include system packages")
	cmd.Flags().BoolVar(&flagThirdParty, "third-party", false, "Show only third-party packages")
	cmd.Flags().StringVar(&flagFilter, "filter", "", "Regex filter on package name")
	return cmd
}

func newAppClearCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear <package>",
		Short: "Clear app data and cache",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := shell(cmd.Context(), "pm", "clear", args[0]); err != nil {
				return handleErr(err)
			}
			output.Success("Cleared data for %s.", args[0])
			return nil
		},
	}
}

func newAppGrantCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "grant <package> <permission>",
		Short: "Grant a runtime permission to an app",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := shell(cmd.Context(), "pm", "grant", args[0], args[1]); err != nil {
				return handleErr(err)
			}
			output.Success("Granted %s to %s.", args[1], args[0])
			return nil
		},
	}
}

func newAppRevokeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "revoke <package> <permission>",
		Short: "Revoke a runtime permission from an app",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := shell(cmd.Context(), "pm", "revoke", args[0], args[1]); err != nil {
				return handleErr(err)
			}
			output.Success("Revoked %s from %s.", args[1], args[0])
			return nil
		},
	}
}

func newAppLaunchCmd() *cobra.Command {
	var flagActivity string
	var flagWait bool

	cmd := &cobra.Command{
		Use:   "launch <package>",
		Short: "Launch an app by package name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			amArgs := []string{"start", "-n"}
			activity := flagActivity
			if activity == "" {
				// Query the default launcher activity
				mainActivity, err := getMainActivity(cmd.Context(), args[0])
				if err != nil || mainActivity == "" {
					// Fall back to monkey
					return handleErr(
						shell(cmd.Context(), "monkey", "-p", args[0], "-c", "android.intent.category.LAUNCHER", "1"),
					)
				}
				activity = mainActivity
			}
			amArgs = append(amArgs, args[0]+"/"+activity)
			if flagWait {
				amArgs = append(amArgs, "-W")
			}
			if err := shell(cmd.Context(), append([]string{"am"}, amArgs...)...); err != nil {
				return handleErr(err)
			}
			output.Success("Launched %s.", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&flagActivity, "activity", "", "Activity class name (default: main launcher activity)")
	cmd.Flags().BoolVar(&flagWait, "wait", false, "Wait for the activity to start")
	return cmd
}

func newAppStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <package>",
		Short: "Force-stop an app",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := shell(cmd.Context(), "am", "force-stop", args[0]); err != nil {
				return handleErr(err)
			}
			output.Success("Stopped %s.", args[0])
			return nil
		},
	}
}

func newAppDeepLinkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "deep-link <uri>",
		Short: "Open a deep link URI on the device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := shell(cmd.Context(), "am", "start", "-a", "android.intent.action.VIEW", "-d", args[0]); err != nil {
				return handleErr(err)
			}
			output.Success("Opened: %s", args[0])
			return nil
		},
	}
}

// ── helpers ───────────────────────────────────────────────────────────────

// shell runs an adb shell command on the target device.
func shell(ctx context.Context, args ...string) error {
	loc, err := android.New()
	if err != nil {
		return err
	}
	adb, err := loc.Binary("adb")
	if err != nil {
		return err
	}

	cmdArgs := buildADBArgs(deviceSerial(), "shell")
	cmdArgs = append(cmdArgs, args...)
	res, err := runner.RunCapture(ctx, adb, cmdArgs)
	if err != nil {
		return fmt.Errorf("adb shell: %w", err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("adb shell: %s", res.Stderr)
	}
	return nil
}

func buildADBArgs(serial, subcmd string) []string {
	if serial != "" {
		return []string{"-s", serial, subcmd}
	}
	return []string{subcmd}
}

func pmList(ctx context.Context, system, thirdParty bool, filter string) ([]string, error) {
	loc, err := android.New()
	if err != nil {
		return nil, err
	}
	adb, err := loc.Binary("adb")
	if err != nil {
		return nil, err
	}

	args := buildADBArgs(deviceSerial(), "shell")
	pmArgs := []string{"pm", "list", "packages"}
	if thirdParty {
		pmArgs = append(pmArgs, "-3")
	}
	if !system && !thirdParty {
		// default: no flag = all packages
	}
	args = append(args, pmArgs...)

	res, err := runner.RunCapture(ctx, adb, args)
	if err != nil {
		return nil, err
	}

	var pkgs []string
	for _, line := range strings.Split(res.Stdout, "\n") {
		pkg := strings.TrimPrefix(strings.TrimSpace(line), "package:")
		if pkg == "" {
			continue
		}
		if filter != "" && !strings.Contains(pkg, filter) {
			continue
		}
		pkgs = append(pkgs, pkg)
	}
	return pkgs, nil
}

func getMainActivity(ctx context.Context, pkg string) (string, error) {
	loc, err := android.New()
	if err != nil {
		return "", err
	}
	adb, err := loc.Binary("adb")
	if err != nil {
		return "", err
	}

	args := buildADBArgs(deviceSerial(), "shell")
	args = append(args, "cmd", "package", "resolve-activity", "--brief", pkg)
	res, err := runner.RunCapture(ctx, adb, args)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(res.Stdout, "\n") {
		if strings.Contains(line, "/") {
			parts := strings.SplitN(strings.TrimSpace(line), "/", 2)
			if len(parts) == 2 {
				return parts[1], nil
			}
		}
	}
	return "", fmt.Errorf("could not resolve main activity for %s", pkg)
}
