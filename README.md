# acli — Unified Android CLI

[![CI](https://github.com/ErikHellman/android-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/ErikHellman/android-cli/actions/workflows/ci.yml)

A single, ergonomic command-line interface for all Android development tasks. `acli` wraps `sdkmanager`, `avdmanager`, `adb`, `fastboot`, and Gradle so you never have to memorize package paths, flag syntax, or which binary lives where.

```
$ acli doctor
✓  ANDROID_HOME is set (/Users/you/Library/Android/sdk)
✓  Java is installed (openjdk version "21.0.10")
✓  adb found
✓  sdkmanager found
✓  avdmanager found
✓  emulator found
✓  fastboot found
✗  SDK licenses not fully accepted
   Run: acli sdk licenses
✓  ADB server running
✓  Connected devices (1 device)
```

---

## Table of Contents

- [Why acli](#why-acli)
- [Runtime Dependencies](#runtime-dependencies)
- [Build Dependencies](#build-dependencies)
- [Installation](#installation)
- [Environment Setup](#environment-setup)
- [Command Reference](#command-reference)
- [Global Flags](#global-flags)
- [JSON Output and Automation](#json-output-and-automation)
- [AI Agent Integration (Claude Code)](#ai-agent-integration-claude-code)
- [Development Guide](#development-guide)
- [Creating a Release](#creating-a-release)
- [Project Structure](#project-structure)
- [Configuration](#configuration)
- [Shell Completion](#shell-completion)
- [Self-Update](#self-update)
- [Error Handling](#error-handling)

---

## Why acli

Android's command-line tooling is fragmented across six separate binaries with inconsistent interfaces:

| Tool | Location | Problem |
|---|---|---|
| `sdkmanager` | `cmdline-tools/latest/bin/` | Package paths like `"system-images;android-35;google_apis;x86_64"` |
| `avdmanager` | `cmdline-tools/latest/bin/` | Requires knowing exact image IDs |
| `adb` | `platform-tools/` | `error: more than one device/emulator` with no guidance |
| `fastboot` | `platform-tools/` | Only works in bootloader mode; easy to brick devices |
| `emulator` | `emulator/` | Dozens of flags, no wait-for-boot |
| `gradlew` | project root | Must `cd` to project root; opaque error messages |

`acli` solves this with:

- **One binary** — no PATH juggling across SDK subdirectories
- **Ergonomic subcommands** — `acli sdk install "platforms;android-35"` instead of searching for the right path
- **Contextual error messages** — instead of passing raw Java stack traces, `acli` maps 15+ known error patterns to human-readable explanations with exact fix commands
- **`--json` flag** — every command emits structured JSON for use in CI pipelines and AI agents
- **`acli doctor`** — instant environment health check

---

## Runtime Dependencies

`acli` is a thin wrapper around Android's tooling. The SDK command-line tools can be installed automatically via `acli sdk bootstrap`; everything else must be installed separately.

### Required

| Dependency | Version | Purpose | Install |
|---|---|---|---|
| **Android SDK Command-Line Tools** (`sdkmanager`, `avdmanager`) | Any recent | `acli sdk`, `acli avd`; also required for all other SDK-dependent commands | `acli sdk bootstrap` ← start here |
| **Android Platform Tools** (`adb`, `fastboot`) | Any recent | `acli device`, `acli flash`, `acli instrument` | `acli sdk install platform-tools` |
| **Java (JDK)** | 17 or newer (21 recommended) | Required by `sdkmanager`, `avdmanager`, and Gradle | [SDKMAN](https://sdkman.io) — see below |

### Optional

| Dependency | Purpose |
|---|---|
| **Android Emulator** (`emulator` binary) | `acli avd start` / `acli avd stop` — install with `acli sdk install emulator` |
| **Gradle wrapper** (`gradlew` in project root) | All `acli build` commands |

### Installing Java with SDKMAN

[SDKMAN](https://sdkman.io) is the recommended way to install and manage Java versions on macOS and Linux. It handles download, extraction, and shell configuration automatically and makes switching between Java versions trivial.

```bash
# 1. Install SDKMAN
curl -s "https://get.sdkman.io" | bash
source "$HOME/.sdkman/bin/sdkman-init.sh"

# 2. Install Java 21 (Temurin — recommended for Android development)
sdk install java 21-tem

# 3. Verify
java -version
```

To see all available JDK distributions and versions:

```bash
sdk list java
```

### SDK Auto-Discovery

`acli` finds your Android SDK root automatically in this order:

1. `$ANDROID_HOME` environment variable
2. `$ANDROID_SDK_ROOT` environment variable
3. Well-known platform paths:
   - macOS: `~/Library/Android/sdk`
   - Linux: `~/Android/Sdk`, `/opt/android-sdk`
   - Windows: `%LOCALAPPDATA%\Android\Sdk`

Run `acli doctor` to verify that everything is found correctly.

---

## Build Dependencies

You only need these if you are building `acli` from source.

| Dependency | Version | Purpose |
|---|---|---|
| **Go** | 1.22 or newer | Compiler and toolchain |
| **git** | Any | Version injection in the binary via `git describe` |
| **make** | Any (optional) | Convenience targets; `go build` works directly without it |

All Go library dependencies are declared in `go.mod` and downloaded automatically by `go mod download` or `go build`. No external package manager is required.

**Direct library dependencies** (see `go.mod` for pinned versions):

| Library | Purpose |
|---|---|
| `github.com/spf13/cobra` | CLI framework: command tree, `--help`, shell completion |
| `github.com/spf13/viper` | Config file and environment variable management |
| `github.com/charmbracelet/lipgloss` | Color-coded terminal output panels |
| `golang.org/x/term` | TTY detection (switches between human and JSON output) |

---

## Installation

### Download a pre-built binary (recommended)

Run this on macOS or Linux — it detects your OS and architecture automatically:

```bash
curl -fsSL https://raw.githubusercontent.com/ErikHellman/android-cli/main/install.sh | bash
```

**Windows** (PowerShell):

```powershell
irm https://raw.githubusercontent.com/ErikHellman/android-cli/main/install.ps1 | iex
```

Installs to `%LOCALAPPDATA%\Programs\acli\` and adds it to your user `PATH` — no administrator rights required.

**Windows via WSL:** If you use Windows Subsystem for Linux, run the macOS/Linux command above inside your WSL terminal instead.

### Build from source

```bash
git clone https://github.com/android-cli/acli.git
cd acli
make install          # builds and installs to $GOPATH/bin
```

Or without `make`:

```bash
go install github.com/android-cli/acli/cmd/acli@latest
```

### Verify the installation

```bash
acli --help
acli doctor
```

---

## Environment Setup

Set `$ANDROID_HOME` so `acli` can find your SDK:

```bash
# macOS / Linux — add to ~/.zshrc or ~/.bashrc
export ANDROID_HOME=~/Library/Android/sdk          # macOS default
# export ANDROID_HOME=~/Android/Sdk               # Linux default

export PATH="$ANDROID_HOME/platform-tools:$PATH"   # adds adb, fastboot to PATH
```

Then reload your shell and confirm:

```bash
acli doctor
```

---

## Command Reference

### `acli sdk` — SDK Package Management

```bash
acli sdk list                          # all packages
acli sdk list --installed              # only installed packages
acli sdk list --available              # only packages available to install
acli sdk list --updates                # packages with available updates
acli sdk list --channel canary         # include canary channel

acli sdk install "platforms;android-35"
acli sdk install "build-tools;35.0.0" "platform-tools"
acli sdk install "system-images;android-35;google_apis;x86_64"
acli sdk install "ndk;26.1.10909125"

acli sdk uninstall "platforms;android-33"
acli sdk update                        # update all installed packages
acli sdk licenses                      # accept all pending licenses (CI-safe)
```

### `acli avd` — Virtual Device Management

```bash
acli avd list                          # all AVDs
acli avd list --running                # only running emulators

acli avd create Pixel9 --api 35                                    # arm64-v8a by default
acli avd create MyPhone --api 34 --device pixel_7 --abi x86_64
acli avd create TestPhone --api 35 --sdcard 512M --force
acli avd create MyAuto --api 35-ext15 --tag android-automotive-playstore --device automotive_1408p_landscape_with_google_apis

acli avd start Pixel9                  # launch emulator window
acli avd start Pixel9 --headless       # no window (CI mode)
acli avd start Pixel9 --headless --wait-boot  # block until boot completes
acli avd start Pixel9 --port 5556      # custom ADB port

acli avd stop emulator-5554
acli avd delete Pixel9
acli avd images                        # list installable system images
acli avd images --api 35               # filter by API level
```

### `acli device` — ADB Device Management

```bash
acli device list                       # all connected devices/emulators

# Target a specific device with -d / --device or $ACLI_DEVICE
acli -d emulator-5554 device shell             # interactive shell
acli -d emulator-5554 device shell dumpsys battery

acli device install app-debug.apk
acli device install app-debug.apk --grant-all --reinstall
acli device uninstall com.example.app

acli device logs                       # all logcat output
acli device logs --follow --level E    # stream errors only
acli device logs MyApp --level D       # filter to one tag
acli device logs --clear               # clear buffer first

acli device push ./data.json /sdcard/data.json
acli device pull /sdcard/screenshot.png ./local/

acli device screenshot                 # saves to screenshot-<timestamp>.png
acli device screenshot output.png
acli device record                     # records to screenrecord-<timestamp>.mp4
acli device record --duration 30 demo.mp4

acli device reboot
acli device reboot --bootloader        # into fastboot mode
acli device reboot --recovery

acli device forward 8080 8080          # host:8080 → device:8080
acli device reverse 3000 3000          # device:3000 → host:3000

acli device pair 192.168.1.5:37000     # Android 11+ wireless pairing
acli device connect 192.168.1.5:5555
acli device info                       # model, OS version, ABI, serial
```

### `acli app` — Application Management

```bash
acli app list                          # all packages
acli app list --third-party            # user-installed only
acli app list --filter myapp

acli app launch com.example.app
acli app launch com.example.app --activity .MainActivity --wait
acli app stop com.example.app
acli app clear com.example.app         # wipe data + cache

acli app grant  com.example.app android.permission.CAMERA
acli app revoke com.example.app android.permission.CAMERA

acli app deep-link "https://example.com/product/123"
```

### `acli build` — Gradle Wrapper

`acli` automatically walks up from the current directory to find the project root (the directory containing `settings.gradle` or `build.gradle`).

```bash
acli build assemble                    # debug APK
acli build assemble --variant release
acli build assemble --module :feature:login --variant debug

acli build test                        # unit + instrumented tests
acli build test --unit
acli build test --instrumented         # requires connected device

acli build clean
acli build lint
acli build lint --fix
acli build bundle --variant release    # Android App Bundle (.aab)
acli build run dependencies            # arbitrary Gradle task
acli build run :app:generateDebugSources
```

### `acli flash` — Fastboot Flashing

The device must be in fastboot/bootloader mode first (`acli device reboot --bootloader`).

```bash
acli flash list                        # devices in fastboot mode
acli flash image boot boot.img
acli flash factory ~/Downloads/shiba-factory.zip
acli flash reboot                      # back to Android
acli flash reboot --bootloader
acli flash unlock                      # OEM bootloader unlock (destructive — prompts for confirmation)
```

### `acli instrument` — Device Instrumentation

```bash
# Battery simulation
acli instrument battery --level 15
acli instrument battery --status discharging
acli instrument battery --level 5 --status discharging
acli instrument battery --reset        # restore real values

# Network simulation (emulators only)
acli instrument network --speed edge
acli instrument network --speed gprs --latency umts

# GPS mock location
acli instrument location --lat 37.7749 --lng -122.4194

# Input events
acli instrument input text "Hello World"
acli instrument input tap 540 960
acli instrument input key KEYCODE_HOME
```

### `acli skills` — AI Agent Integration

```bash
acli skills install                    # project scope (.claude/skills/acli/SKILL.md)
acli skills install --scope user       # user scope (~/.claude/skills/acli/SKILL.md)
acli skills list                       # show installation status
```

### `acli doctor` — Environment Health Check

```bash
acli doctor                            # human-readable checklist
acli doctor --json                     # machine-readable (for CI)
```

### `acli update` — Self-Update

```bash
acli update check                      # compare current vs latest
acli update install                    # download and replace binary
acli update install --version v1.2.0  # install a specific version
```

### `acli completion` — Shell Completion

```bash
acli completion bash > /etc/bash_completion.d/acli
acli completion zsh  > "${fpath[1]}/_acli"
acli completion fish > ~/.config/fish/completions/acli.fish
```

---

## Global Flags

These flags work with every command:

| Flag | Description |
|---|---|
| `-d, --device <serial>` | Target a specific device by ADB serial. Overrides `$ACLI_DEVICE` |
| `--json` | Emit all output as JSON to stdout; errors go to stderr |
| `-v, --verbose` | Show underlying error details and subprocess output |
| `--no-color` | Disable color output (auto-disabled when not a TTY) |

**Device targeting** is resolved in this order: `--device` flag → `$ACLI_DEVICE` env var → `default_device` in `~/.acli/config.yaml`.

---

## JSON Output and Automation

Pass `--json` to any command for machine-readable output. This is useful for CI pipelines and AI agents.

```bash
# List devices as JSON
acli device list --json
# [{"serial":"emulator-5554","state":"device","model":"sdk_gphone64_arm64",...}]

# Check environment health in CI
acli doctor --json
# {"checks":[{"label":"ANDROID_HOME is set","ok":true,"detail":"..."},{"label":"adb found","ok":true},...]}

# List installed SDK packages
acli sdk list --installed --json
```

**Error format** — all errors are written to stderr as structured JSON when `--json` is active:

```json
{
  "error": {
    "code": "device_not_found",
    "message": "No Android device or emulator is connected.",
    "detail": "ADB cannot find a target device...",
    "fix": ["acli device list", "acli avd start <avd-name>", "adb start-server"],
    "docs": "https://developer.android.com/tools/adb#devicestatus"
  }
}
```

**Exit codes** are POSIX-standard and consistent:

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | General error |
| 2 | Usage error (bad arguments or flags) |
| 3 | Device not found or ambiguous |
| 4 | SDK / environment not configured |
| 5 | Build failure |
| 6 | Process timeout |

---

## AI Agent Integration (Claude Code)

`acli` ships with a built-in Claude Code skill that gives AI agents native control over the Android environment.

```bash
# Install for the current project (committed to .claude/skills/)
acli skills install

# Or install globally for all your projects
acli skills install --scope user
```

Once installed, Claude Code will automatically use `acli` commands when you ask Android-related questions, or you can invoke it directly with `/acli`. The skill grants `Bash(acli *)` permission so the agent can run any `acli` subcommand without individual approval prompts.

The skill template is also available at [`assets/skills/acli/SKILL.md`](assets/skills/acli/SKILL.md).

---

## Development Guide

### Prerequisites

- Go 1.22 or newer (`go version`)
- `make` (optional but recommended)
- An Android SDK installation for manual testing

### Getting started

```bash
git clone https://github.com/android-cli/acli.git
cd acli

# Download dependencies (no network needed after this)
go mod download

# Build the binary into dist/
make build
# or: go build -o dist/acli ./cmd/acli

# Run all unit tests
make test
# or: go test ./... -v

# Install to $GOPATH/bin
make install
```

### Makefile targets

| Target | Description |
|---|---|
| `make build` | Build `dist/acli` with version info from `git describe` |
| `make install` | Build and install to `$GOPATH/bin` |
| `make test` | Run all tests with `-v -count=1` |
| `make lint` | Run `go vet ./...` |
| `make clean` | Remove `dist/` |
| `make release` | Cross-compile for all platforms into `dist/` |
| `make doctor` | Print Go version and module info |

### Running tests

```bash
# All tests
go test ./...

# Specific package
go test ./pkg/aclerr/... -v
go test ./pkg/runner/... -v
go test ./pkg/output/... -v

# With race detector
go test -race ./...
```

The unit tests in `pkg/` cover:

- **`pkg/aclerr`** — all 15 error catalog patterns, `AcliError` methods, exit code mapping
- **`pkg/runner`** — subprocess capture, passthrough, env, stdin, timeout, working directory, binary-not-found
- **`pkg/output`** — JSON error format, JSON table schema, JSON checklist, human error rendering, nil error safety

Integration tests (requiring a real Android SDK) are not automated but can be exercised manually with `acli doctor` after configuring `$ANDROID_HOME`.

### Making changes

**Adding a new command:**

1. Create a `new<Name>Cmd() *cobra.Command` function in a file under `internal/cmd/`.
2. Register it in `internal/cmd/root.go` inside `RootCmd.AddCommand(...)`.
3. If the command needs underlying Android tooling, add a service method in the appropriate `internal/<domain>/service.go`.

**Adding a new error pattern:**

1. Add a constant to `pkg/aclerr/errors.go` if a new `ErrorCode` is needed.
2. Add an `ErrorPattern` entry to the `catalog` slice in `pkg/aclerr/catalog.go`.
3. Add a test case to `pkg/aclerr/errors_test.go`.

**Changing output format:**

All rendering goes through `pkg/output`. The `Renderer` methods branch on `r.JSON` for machine vs. human output, so changing one path does not affect the other.

### Cross-compilation

```bash
make release
```

This produces binaries in `dist/` for:
- `acli-darwin-arm64` (macOS Apple Silicon)
- `acli-darwin-amd64` (macOS Intel)
- `acli-linux-amd64`
- `acli-linux-arm64`
- `acli-windows-amd64.exe`

### Version injection

The version string displayed by `acli --version` and used by `acli update check` is injected at build time via `-ldflags`:

```
-ldflags "-X main.version=$(git describe --tags) -X main.commit=$(git rev-parse --short HEAD)"
```

`make build` and `make release` handle this automatically. If built outside of `make` without `-ldflags`, the version will be reported as `dev`.

---

## Creating a Release

Releases are fully automated via `.github/workflows/release.yml`. The only manual step is tagging.

**1. Ensure `main` is green**

Confirm the CI workflow is passing on `main` before tagging.

**2. Create an annotated tag**

```bash
git tag -a v1.2.3 -m "Short description of what changed"
```

Use [semver](https://semver.org) with a `v` prefix. The tag message becomes the seed for the release notes (the workflow also appends the commit log since the previous tag).

**3. Push the tag**

```bash
git push origin v1.2.3
```

This is the trigger. Pushing the tag starts the release workflow automatically — no further action is required.

**What the workflow does:**

1. Runs `make test` — the release is aborted if any test fails
2. Runs `make release` — cross-compiles 5 platform binaries into `dist/`
3. Generates a `checksums.txt` (all binaries) and individual `<binary>.sha256` sidecars (used by `acli update install`)
4. Generates release notes from `git log <prev-tag>..HEAD --oneline --no-merges`
5. Creates a GitHub Release named after the tag and uploads all artifacts

**Verifying the release**

After the workflow completes (~2 minutes), check the [Releases page](https://github.com/ErikHellman/android-cli/releases) and confirm:
- All 5 binaries are present
- `checksums.txt` and `.sha256` sidecars are attached
- `acli update check` reports the new version

---

## Project Structure

```
android-cli/
├── cmd/
│   └── acli/
│       └── main.go              # Entry point; injects version/commit
├── internal/
│   ├── cmd/                     # Cobra command definitions (one file per domain)
│   │   ├── root.go              # Root command, global flags, PersistentPreRunE
│   │   ├── sdk.go               # acli sdk *
│   │   ├── avd.go               # acli avd *
│   │   ├── device.go            # acli device *
│   │   ├── app.go               # acli app *
│   │   ├── build.go             # acli build *
│   │   ├── project.go           # acli project init
│   │   ├── flash.go             # acli flash *
│   │   ├── instrument.go        # acli instrument *
│   │   ├── skills.go            # acli skills *
│   │   ├── doctor.go            # acli doctor
│   │   └── update.go            # acli update *
│   ├── sdk/service.go           # sdkmanager wrapper + output parser
│   ├── avd/service.go           # avdmanager + emulator wrapper
│   ├── device/service.go        # adb wrapper + device list parser
│   ├── build/service.go         # gradlew wrapper + project root discovery
│   ├── flash/service.go         # fastboot wrapper
│   └── instrument/service.go    # adb shell instrumentation commands
├── pkg/
│   ├── aclerr/
│   │   ├── errors.go            # AcliError type, ErrorCode constants, exit codes
│   │   ├── catalog.go           # 15+ regex patterns → structured errors
│   │   └── errors_test.go
│   ├── output/
│   │   ├── output.go            # Renderer: TTY detect, lipgloss panels, JSON mode
│   │   └── output_test.go
│   ├── runner/
│   │   ├── runner.go            # Subprocess manager: capture, passthrough, timeout
│   │   └── runner_test.go
│   ├── android/
│   │   └── locator.go           # SDK root discovery, binary path resolution
│   ├── config/
│   │   └── config.go            # Viper config (~/.acli/config.yaml)
│   └── update/
│       └── updater.go           # GitHub Releases API + atomic binary replacement
├── assets/
│   └── skills/
│       └── acli/
│           └── SKILL.md         # Claude Code skill template
├── dist/                        # Built binaries (gitignored)
├── go.mod
├── go.sum
└── Makefile
```

---

## Configuration

`acli` reads `~/.acli/config.yaml` and environment variables prefixed with `ACLI_`. Environment variables take precedence over the config file.

```yaml
# ~/.acli/config.yaml

# Default device serial to target when --device is not specified.
# Equivalent to setting $ACLI_DEVICE in your shell.
default_device: "emulator-5554"

# Override Android SDK root (normally auto-discovered).
# Equivalent to $ANDROID_HOME.
sdk_root: ""

# GitHub repository used for self-update checks.
github_repo: "android-cli/acli"
```

---

## Shell Completion

**Zsh:**
```bash
acli completion zsh > "${fpath[1]}/_acli"
# Restart your shell or: autoload -U compinit && compinit
```

**Bash:**
```bash
acli completion bash > /etc/bash_completion.d/acli
# or for a single user:
acli completion bash > ~/.bash_completion
```

**Fish:**
```bash
acli completion fish > ~/.config/fish/completions/acli.fish
```

---

## Self-Update

```bash
acli update check          # prints current version vs. latest GitHub release
acli update install        # downloads and atomically replaces the current binary
```

The update command:
1. Queries the GitHub Releases API for the latest release
2. Downloads the asset matching the current OS and architecture
3. Verifies the SHA256 checksum (if a `.sha256` asset is present)
4. Atomically replaces the running binary

---

## Error Handling

`acli` intercepts raw tool output and maps known failure modes to actionable messages. For example, when a Gradle build runs out of memory:

**Before (raw Gradle output):**
```
> Task :app:compileDebugKotlin FAILED
...
java.lang.OutOfMemoryError: Java heap space
	at ...50 lines of stack trace...
```

**After (acli):**
```
╭─ Error: out_of_memory ──────────────────────────────────╮
│                                                           │
│  Gradle ran out of memory.                               │
│                                                           │
│  The JVM heap was exhausted during the build. Increase   │
│  the heap size in gradle.properties.                     │
│                                                           │
│  Try:                                                     │
│    echo 'org.gradle.jvmargs=-Xmx4g' >> gradle.properties│
│    acli build assemble                                    │
│                                                           │
╰───────────────────────────────────────────────────────────╯
```

In `--json` mode the same error is emitted to stderr as structured JSON, making it trivially parseable in CI or by an AI agent.

The error catalog covers: device not found, multiple devices, unauthorized device, device offline, APK install failures (version conflict, insufficient storage, and others), SDK license not accepted, SDK package not found, network errors, AVD not found, emulator port in use, Gradle build failures, Gradle OOM, and Gradle wrapper not found.
