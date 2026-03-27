package cmd

import (
	"github.com/android-cli/acli/internal/sdk"
	"github.com/android-cli/acli/pkg/android"
	"github.com/android-cli/acli/pkg/output"
	"github.com/spf13/cobra"
)

func newSDKCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sdk",
		Short: "Manage Android SDK packages (wraps sdkmanager)",
	}
	cmd.AddCommand(
		newSDKListCmd(),
		newSDKInstallCmd(),
		newSDKUninstallCmd(),
		newSDKUpdateCmd(),
		newSDKLicensesCmd(),
		newSDKBootstrapCmd(),
	)
	return cmd
}

// ── sdk list ──────────────────────────────────────────────────────────────

func newSDKListCmd() *cobra.Command {
	var (
		flagInstalled bool
		flagAvailable bool
		flagUpdates   bool
		flagChannel   string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List SDK packages",
		Example: `  acli sdk list
  acli sdk list --available
  acli sdk list --installed
  acli sdk list --updates
  acli sdk list --available --channel canary`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := sdk.New(loc)

			installed := flagInstalled || flagUpdates
			available := flagAvailable || flagUpdates

			pkgs, err := svc.List(cmd.Context(), installed, available, flagChannel)
			if err != nil {
				return handleErr(err)
			}

			if len(pkgs) == 0 {
				output.Info("No packages found matching the given filters.")
				return nil
			}

			headers := []string{"Path", "Version", "Description", "Installed"}
			var rows [][]string
			for _, p := range pkgs {
				installed := "no"
				if p.Installed {
					installed = "yes"
				}
				rows = append(rows, []string{p.Path, p.Version, p.Description, installed})
			}
			output.Table(headers, rows)
			return nil
		},
	}

	cmd.Flags().BoolVar(&flagInstalled, "installed", false, "Show only installed packages")
	cmd.Flags().BoolVar(&flagAvailable, "available", false, "Show only available (not installed) packages")
	cmd.Flags().BoolVar(&flagUpdates, "updates", false, "Show packages with available updates")
	cmd.Flags().StringVar(&flagChannel, "channel", "stable", "Channel: stable, beta, dev, canary")
	return cmd
}

// ── sdk install ───────────────────────────────────────────────────────────

func newSDKInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <package> [packages...]",
		Short: "Install one or more SDK packages",
		Long: `Install Android SDK packages by their path identifiers.

Examples:
  acli sdk install "platforms;android-35"
  acli sdk install "build-tools;35.0.0" "platform-tools"
  acli sdk install "ndk;26.1.10909125"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := sdk.New(loc)

			output.Info("Installing: %v", args)
			if err := svc.Install(cmd.Context(), args); err != nil {
				return handleErr(err)
			}
			output.Success("Installed %d package(s) successfully.", len(args))
			return nil
		},
	}
	return cmd
}

// ── sdk uninstall ─────────────────────────────────────────────────────────

func newSDKUninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall <package> [packages...]",
		Short: "Uninstall one or more SDK packages",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := sdk.New(loc)

			output.Info("Uninstalling: %v", args)
			if err := svc.Uninstall(cmd.Context(), args); err != nil {
				return handleErr(err)
			}
			output.Success("Uninstalled %d package(s).", len(args))
			return nil
		},
	}
	return cmd
}

// ── sdk update ────────────────────────────────────────────────────────────

func newSDKUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update all installed SDK packages",
		RunE: func(cmd *cobra.Command, _ []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := sdk.New(loc)

			output.Info("Updating all installed SDK packages…")
			if err := svc.Update(cmd.Context()); err != nil {
				return handleErr(err)
			}
			output.Success("All packages are up to date.")
			return nil
		},
	}
	return cmd
}

// ── sdk bootstrap ─────────────────────────────────────────────────────────

func newSDKBootstrapCmd() *cobra.Command {
	var (
		flagDir   string
		flagForce bool
	)
	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Install Android SDK command-line tools from scratch",
		Long: `Downloads the Android SDK command-line tools from Google and installs them
at the platform-default location (~/Library/Android/sdk on macOS,
~/Android/Sdk on Linux, %LOCALAPPDATA%\Android\Sdk on Windows).

No existing Android SDK installation is required.

After bootstrapping, run:
  export ANDROID_HOME=<sdk-root>
  acli sdk licenses
  acli doctor`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Do NOT call android.New() — this command works without an existing SDK.
			b := sdk.NewBootstrapper()
			sdkRoot, alreadyInstalled, err := b.Bootstrap(cmd.Context(), flagDir, flagForce)
			if err != nil {
				return handleErr(err)
			}
			if alreadyInstalled {
				output.Info("Android SDK command-line tools are already installed at %s", sdkRoot)
				output.Info("Use --force to reinstall.")
				return nil
			}
			output.Success("Command-line tools installed at %s", sdkRoot)
			output.Info("Next steps:")
			output.Info("  export ANDROID_HOME=%s", sdkRoot)
			output.Info("  acli sdk licenses")
			output.Info("  acli doctor")
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDir, "dir", "", "Override the SDK root install path")
	cmd.Flags().BoolVar(&flagForce, "force", false, "Reinstall even if already present")
	return cmd
}

// ── sdk licenses ──────────────────────────────────────────────────────────

func newSDKLicensesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "licenses",
		Short: "Accept all pending Android SDK licenses",
		Long: `Accepts all pending SDK licenses non-interactively.
Particularly useful in CI/CD environments where a GUI is unavailable.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := sdk.New(loc)

			output.Info("Accepting SDK licenses…")
			count, err := svc.AcceptLicenses(cmd.Context())
			if err != nil {
				return handleErr(err)
			}
			if count > 0 {
				output.Success("Accepted %d license(s).", count)
			} else {
				output.Success("All licenses already accepted.")
			}
			return nil
		},
	}
	return cmd
}
