// Package cmd defines the acli command tree using Cobra.
package cmd

import (
	"os"

	"github.com/android-cli/acli/pkg/aclerr"
	"github.com/android-cli/acli/pkg/config"
	"github.com/android-cli/acli/pkg/output"
	"github.com/spf13/cobra"
)

// globalFlags holds values parsed from persistent root-level flags.
var globalFlags struct {
	JSON    bool
	Verbose bool
	NoColor bool
	Device  string
}

// RootCmd is the top-level cobra command.
var RootCmd = &cobra.Command{
	Use:   "acli",
	Short: "Unified Android development CLI",
	Long: `acli is a single, ergonomic interface for all Android development tasks.

It wraps sdkmanager, avdmanager, adb, fastboot, and Gradle so you never
have to memorize complex flags or package paths again.`,
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		// Skip initialization for completion commands
		if cmd.Name() == "completion" || cmd.Parent() != nil && cmd.Parent().Name() == "completion" {
			return nil
		}
		output.Init(globalFlags.JSON, globalFlags.Verbose, globalFlags.NoColor)
		return config.Load()
	},
}

func init() {
	pf := RootCmd.PersistentFlags()
	pf.BoolVar(&globalFlags.JSON, "json", false, "Output results as JSON (machine-readable)")
	pf.BoolVarP(&globalFlags.Verbose, "verbose", "v", false, "Show verbose output including underlying errors")
	pf.BoolVar(&globalFlags.NoColor, "no-color", false, "Disable color output")
	pf.StringVarP(&globalFlags.Device, "device", "d", "", "Target device serial (overrides $ACLI_DEVICE)")

	// Register all subcommands
	RootCmd.AddCommand(
		newSDKCmd(),
		newAVDCmd(),
		newDeviceCmd(),
		newAppCmd(),
		newBuildCmd(),
		newProjectCmd(),
		newFlashCmd(),
		newInstrumentCmd(),
		newSkillsCmd(),
		newDoctorCmd(),
		newUpdateCmd(),
		newCompletionCmd(),
	)
}

// deviceSerial resolves the target device serial from flag → env var → empty.
func deviceSerial() string {
	if globalFlags.Device != "" {
		return globalFlags.Device
	}
	if v := os.Getenv("ACLI_DEVICE"); v != "" {
		return v
	}
	return config.Get().DefaultDevice
}

// handleErr renders an error and returns it so the caller can return it to Cobra
// (which triggers a non-zero exit). Cobra is told to silence its own error
// printing via SilenceErrors=true on the root command.
func handleErr(err error) error {
	if err == nil {
		return nil
	}
	output.Error(err)
	return err
}

// exitCode extracts the appropriate POSIX exit code from an AcliError.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var ae *aclerr.AcliError
	if aclerr.As(err, &ae) {
		return ae.Code.ExitCode()
	}
	return 1
}

// newCompletionCmd adds shell completion subcommands.
func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion",
		Short: "Generate shell completion scripts",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "bash",
		Short: "Generate Bash completion script",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return RootCmd.GenBashCompletion(os.Stdout)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "zsh",
		Short: "Generate Zsh completion script",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return RootCmd.GenZshCompletion(os.Stdout)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "fish",
		Short: "Generate Fish completion script",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return RootCmd.GenFishCompletion(os.Stdout, true)
		},
	})
	return cmd
}
