package cmd

import (
	"fmt"

	"github.com/android-cli/acli/pkg/config"
	"github.com/android-cli/acli/pkg/output"
	"github.com/android-cli/acli/pkg/update"
	"github.com/spf13/cobra"
)

// Version and commit are injected at build time via -ldflags.
var (
	Version = "dev"
	Commit  = "none"
)

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Check for and install acli updates",
	}
	cmd.AddCommand(
		newUpdateCheckCmd(),
		newUpdateInstallCmd(),
	)
	return cmd
}

func newUpdateCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Check for a newer version of acli",
		RunE: func(cmd *cobra.Command, _ []string) error {
			output.Info("Current version: %s (%s)", Version, Commit)
			output.Info("Checking for updates…")

			repo := config.Get().GithubRepo
			rel, err := update.LatestRelease(repo)
			if err != nil {
				return handleErr(err)
			}

			if rel.TagName == Version || rel.TagName == "v"+Version {
				output.Success("acli is up to date (%s).", Version)
				return nil
			}

			output.Println("")
			output.Println("  New version available: %s", rel.TagName)
			output.Println("  Run: acli update install")
			return nil
		},
	}
}

func newUpdateInstallCmd() *cobra.Command {
	var flagVersion string

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Download and install the latest (or specified) version of acli",
		RunE: func(cmd *cobra.Command, _ []string) error {
			repo := config.Get().GithubRepo

			var rel *update.Release
			var err error

			if flagVersion != "" {
				output.Info("Fetching release %s…", flagVersion)
				// We fetch latest and compare; for exact version we'd need the GitHub tags API.
				// For simplicity, fetch latest and validate.
				rel, err = update.LatestRelease(repo)
				if err != nil {
					return handleErr(err)
				}
				if rel.TagName != flagVersion && rel.TagName != "v"+flagVersion {
					return handleErr(fmt.Errorf("version %q not available; latest is %s", flagVersion, rel.TagName))
				}
			} else {
				output.Info("Fetching latest release…")
				rel, err = update.LatestRelease(repo)
				if err != nil {
					return handleErr(err)
				}
			}

			if rel.TagName == Version || rel.TagName == "v"+Version {
				output.Success("acli is already at %s.", Version)
				return nil
			}

			output.Info("Installing %s…", rel.TagName)
			if err := update.Install(rel); err != nil {
				return handleErr(err)
			}
			output.Success("Updated to %s. Restart acli to use the new version.", rel.TagName)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagVersion, "version", "", "Specific version to install, e.g. v1.2.0")
	return cmd
}
