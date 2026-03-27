# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make build       # compile → dist/acli (with version/commit ldflags)
make test        # go test ./... -v -count=1
make lint        # go vet ./...
make install     # go install to $GOPATH/bin
make release     # cross-compile for darwin/linux/windows (5 binaries in dist/)
make clean       # rm -rf dist/

# Run a single test package
go test ./pkg/aclerr/... -v -run TestClassify

# Run a single test function
go test ./pkg/runner/... -v -run TestRunCapture_Timeout

# Build and run locally
go build -o dist/acli ./cmd/acli && ./dist/acli doctor
```

## Architecture

The binary is `cmd/acli/main.go` → calls `internal/cmd` (Cobra tree) → delegates to service layers.

### Layer map

| Layer | Path | Role |
|---|---|---|
| CLI commands | `internal/cmd/*.go` | One file per subcommand group; registers with `RootCmd` in `root.go` |
| Domain services | `internal/{sdk,avd,device,build,flash,instrument}/service.go` | Business logic; calls `runner` and `android.SDKLocator` |
| Shared packages | `pkg/` | No internal imports; safe to use anywhere |

### Key packages

**`pkg/runner`** — all subprocess execution goes here. `Run()` returns `(*Result, error)`. Use `RunWith()` when you only care about the error (passthrough/fire-and-forget), `RunCapture()` for captured output, `RunWithStdin()` when stdin piping is needed. Context timeout check happens **before** ExitError check (critical ordering — killed processes also return ExitError(-1)).

**`pkg/aclerr`** — structured errors. Every user-facing error is an `*AcliError{Code, Message, Detail, FixCmds, DocsURL}`. Exit codes: device errors→3, SDK/env→4, build→5, emulator timeout→6, general→1. `catalog.go` has regex patterns that classify raw tool stderr into structured errors via `aclerr.Classify(tool, stderr)`.

**`pkg/output`** — call `output.Init(json, verbose, noColor)` once (done in `PersistentPreRunE`). Then use package-level `Success()`, `Info()`, `Warn()`, `Error()`, `Table()`, `CheckList()`. All functions branch internally on `Default.JSON` — never check the flag yourself in command code. Errors go to stderr; data goes to stdout.

**`pkg/android`** — `SDKLocator` finds the Android SDK root (checks `$ANDROID_HOME`, `$ANDROID_SDK_ROOT`, then OS-specific well-known paths). Every service that needs a binary calls `loc.Binary("adb")` etc.

**`pkg/config`** — Viper-based; reads `~/.acli/config.yaml`, env prefix `ACLI_`. `config.Get().DefaultDevice` is the fallback device serial.

### Adding a new subcommand

1. Create `internal/cmd/foo.go` with `newFooCmd() *cobra.Command`.
2. Register it in `root.go` `init()`: `RootCmd.AddCommand(newFooCmd())`.
3. Use `handleErr(err)` to render and return errors. Use `deviceSerial()` to resolve `--device`.
4. Create a service in `internal/foo/service.go` that accepts `*android.SDKLocator` and uses `runner` for subprocess calls.

### Adding a new error pattern

Add an `ErrorPattern` entry to the `catalog` slice in `pkg/aclerr/catalog.go`. Match by `Tool` name and a `*regexp.Regexp` on stderr text; `Build` returns a fully-populated `*AcliError` with `FixCmds`.

### Output duality (human vs JSON)

Every command must handle both modes. Human output uses lipgloss-styled terminal text. JSON output is newline-delimited JSON to stdout (or stderr for errors). Do not add conditional `if --json` checks in commands — use the `output.*` functions which handle both modes internally.
