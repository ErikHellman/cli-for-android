package main

import (
	"os"

	"github.com/android-cli/acli/internal/cmd"
)

// version and commit are injected by the Makefile via -ldflags.
var (
	version = "dev"
	commit  = "none"
)

func main() {
	// Propagate build-time values to the update command
	cmd.Version = version
	cmd.Commit = commit

	if err := cmd.RootCmd.Execute(); err != nil {
		os.Exit(cmd.ExitCode(err))
	}
}
