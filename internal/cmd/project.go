package cmd

import (
	"path"
	"strings"

	"github.com/ErikHellman/android-cli/internal/project"
	"github.com/ErikHellman/android-cli/pkg/output"
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
		flagOutput      string
		flagPackage     string
		flagMinSDK      int
		flagTargetSDK   int
		flagJavaVersion string
	)

	cmd := &cobra.Command{
		Use:   "init <repo-url>",
		Short: "Create a new Android project from a Git template repository",
		Long: `Clone a Git repository as a starting point for a new Android project.
The repository must contain a valid Android project (settings.gradle or settings.gradle.kts).

Optionally customize the cloned project by changing the package name, SDK versions,
or Java version.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoURL := args[0]
			outDir := flagOutput
			if outDir == "" {
				outDir = deriveOutputDir(repoURL)
			}

			svc := project.New()

			output.Info("Downloading template from %s ...", repoURL)
			if err := svc.Download(cmd.Context(), repoURL, outDir); err != nil {
				return handleErr(err)
			}

			if flagPackage != "" {
				output.Info("Refactoring package to %s ...", flagPackage)
				if err := svc.RefactorPackage(outDir, flagPackage); err != nil {
					return handleErr(err)
				}
			}

			if flagMinSDK > 0 {
				output.Info("Setting minSdk to %d ...", flagMinSDK)
				if err := svc.UpdateMinSdk(outDir, flagMinSDK); err != nil {
					return handleErr(err)
				}
			}

			if flagTargetSDK > 0 {
				output.Info("Setting targetSdk to %d ...", flagTargetSDK)
				if err := svc.UpdateTargetSdk(outDir, flagTargetSDK); err != nil {
					return handleErr(err)
				}
			}

			if flagJavaVersion != "" {
				output.Info("Setting Java version to %s ...", flagJavaVersion)
				if err := svc.UpdateJavaVersion(outDir, flagJavaVersion); err != nil {
					return handleErr(err)
				}
			}

			output.Info("Initializing Git repository ...")
			if err := svc.InitRepo(cmd.Context(), outDir, repoURL); err != nil {
				return handleErr(err)
			}

			output.Success("Project created at ./%s", outDir)
			output.Println("  cd %s", outDir)
			output.Println("  acli build assemble")
			return nil
		},
	}

	cmd.Flags().StringVarP(&flagOutput, "output", "o", "", "Output directory (default: derived from repo name)")
	cmd.Flags().StringVar(&flagPackage, "package", "", "Application ID / package name, e.g. com.example.myapp")
	cmd.Flags().IntVar(&flagMinSDK, "min-sdk", 0, "Override minSdk in build.gradle files")
	cmd.Flags().IntVar(&flagTargetSDK, "target-sdk", 0, "Override targetSdk in build.gradle files")
	cmd.Flags().StringVar(&flagJavaVersion, "java-version", "", "Override Java version (e.g. 17, 11, 1.8)")
	return cmd
}

// deriveOutputDir extracts a directory name from a Git repository URL.
// "https://github.com/user/repo.git" → "repo"
// "git@github.com:user/repo.git"     → "repo"
func deriveOutputDir(repoURL string) string {
	// Strip trailing slashes and .git suffix.
	u := strings.TrimRight(repoURL, "/")
	u = strings.TrimSuffix(u, ".git")

	// Handle SSH-style URLs (git@host:user/repo).
	if i := strings.LastIndex(u, ":"); i > 0 && !strings.Contains(u, "://") {
		u = u[i+1:]
	}

	return path.Base(u)
}
