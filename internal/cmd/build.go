package cmd

import (
	"github.com/android-cli/acli/internal/build"
	"github.com/android-cli/acli/pkg/output"
	"github.com/spf13/cobra"
)

func newBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build, test, and lint your Android project (wraps ./gradlew)",
		Long: `Wraps the Gradle wrapper with ergonomic subcommands.
acli automatically walks up from the current directory to find the project root
(the directory containing settings.gradle or build.gradle).`,
	}
	cmd.AddCommand(
		newBuildAssembleCmd(),
		newBuildTestCmd(),
		newBuildCleanCmd(),
		newBuildLintCmd(),
		newBuildBundleCmd(),
		newBuildRunCmd(),
	)
	return cmd
}

func newBuildAssembleCmd() *cobra.Command {
	var flagVariant, flagModule string

	cmd := &cobra.Command{
		Use:   "assemble",
		Short: "Assemble the project (build APKs)",
		Example: `  acli build assemble
  acli build assemble --variant release
  acli build assemble --module app --variant debug`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc := build.New()
			output.Info("Assembling…")
			if err := svc.Assemble(cmd.Context(), flagVariant, flagModule); err != nil {
				return handleErr(err)
			}
			output.Success("Build complete.")
			return nil
		},
	}
	cmd.Flags().StringVar(&flagVariant, "variant", "debug", "Build variant: debug, release")
	cmd.Flags().StringVar(&flagModule, "module", "", "Gradle module, e.g. app or :feature:login")
	return cmd
}

func newBuildTestCmd() *cobra.Command {
	var flagUnit, flagInstrumented bool
	var flagModule string

	cmd := &cobra.Command{
		Use:   "test",
		Short: "Run tests",
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc := build.New()
			if err := svc.Test(cmd.Context(), flagUnit, flagInstrumented, flagModule); err != nil {
				return handleErr(err)
			}
			output.Success("Tests complete.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagUnit, "unit", false, "Run only unit tests")
	cmd.Flags().BoolVar(&flagInstrumented, "instrumented", false, "Run only instrumented (device) tests")
	cmd.Flags().StringVar(&flagModule, "module", "", "Gradle module to test")
	return cmd
}

func newBuildCleanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clean",
		Short: "Clean build outputs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc := build.New()
			output.Info("Cleaning…")
			if err := svc.Clean(cmd.Context()); err != nil {
				return handleErr(err)
			}
			output.Success("Clean complete.")
			return nil
		},
	}
}

func newBuildLintCmd() *cobra.Command {
	var flagModule string
	var flagFix bool

	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Run Android Lint",
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc := build.New()
			output.Info("Running lint…")
			if err := svc.Lint(cmd.Context(), flagModule, flagFix); err != nil {
				return handleErr(err)
			}
			output.Success("Lint complete.")
			return nil
		},
	}
	cmd.Flags().StringVar(&flagModule, "module", "", "Gradle module")
	cmd.Flags().BoolVar(&flagFix, "fix", false, "Auto-fix lint issues where possible")
	return cmd
}

func newBuildBundleCmd() *cobra.Command {
	var flagVariant, flagModule string

	cmd := &cobra.Command{
		Use:   "bundle",
		Short: "Build an Android App Bundle (AAB)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc := build.New()
			output.Info("Bundling…")
			if err := svc.Bundle(cmd.Context(), flagVariant, flagModule); err != nil {
				return handleErr(err)
			}
			output.Success("Bundle complete.")
			return nil
		},
	}
	cmd.Flags().StringVar(&flagVariant, "variant", "release", "Build variant")
	cmd.Flags().StringVar(&flagModule, "module", "", "Gradle module")
	return cmd
}

func newBuildRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run <task> [-- gradle-args...]",
		Short: "Run an arbitrary Gradle task",
		Example: `  acli build run dependencies
  acli build run :app:generateDebugSources
  acli build run tasks -- --all`,
		Args:               cobra.MinimumNArgs(1),
		DisableFlagParsing: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := build.New()
			task := args[0]
			var extra []string
			if len(args) > 1 {
				extra = args[1:]
			}
			return handleErr(svc.RunTask(cmd.Context(), task, extra))
		},
	}
}
