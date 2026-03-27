package aclerr

import "regexp"

// ErrorPattern maps a stderr text pattern to a structured AcliError builder.
type ErrorPattern struct {
	Tool    string
	Pattern *regexp.Regexp
	Build   func(raw string) *AcliError
}

// catalog is the master list of known error patterns, checked in order.
var catalog = []ErrorPattern{
	// ── ADB ──────────────────────────────────────────────────────────────
	{
		Tool:    "adb",
		Pattern: regexp.MustCompile(`(?i)error: no devices/emulators found|no devices/emulators|no connected devices`),
		Build: func(_ string) *AcliError {
			return &AcliError{
				Code:    ErrDeviceNotFound,
				Message: "No Android device or emulator is connected.",
				Detail:  "ADB cannot find a target device. Make sure a device is plugged in with USB debugging enabled, or that an emulator is running.",
				FixCmds: []string{
					"acli device list",
					"acli avd start <avd-name>",
					"adb start-server",
				},
				DocsURL: "https://developer.android.com/tools/adb#devicestatus",
			}
		},
	},
	{
		Tool:    "adb",
		Pattern: regexp.MustCompile(`(?i)error: more than one device/emulator|multiple devices`),
		Build: func(_ string) *AcliError {
			return &AcliError{
				Code:    ErrMultipleDevices,
				Message: "Multiple devices are connected; target one explicitly.",
				Detail:  "ADB found more than one device. Use --device <serial> to specify which one to target.",
				FixCmds: []string{
					"acli device list",
					"acli --device <serial> <command>",
				},
				DocsURL: "https://developer.android.com/tools/adb#directingcommands",
			}
		},
	},
	{
		Tool:    "adb",
		Pattern: regexp.MustCompile(`(?i)unauthorized|RSA key|allow USB debugging`),
		Build: func(_ string) *AcliError {
			return &AcliError{
				Code:    ErrDeviceUnauthorized,
				Message: "Device has not authorized this computer.",
				Detail:  "Check the device screen and tap 'Allow' on the USB debugging dialog. If the dialog does not appear, try revoking USB debugging authorizations on the device and reconnecting.",
				FixCmds: []string{
					"adb kill-server && adb start-server",
				},
				DocsURL: "https://developer.android.com/tools/adb#Enabling",
			}
		},
	},
	{
		Tool:    "adb",
		Pattern: regexp.MustCompile(`(?i)device offline`),
		Build: func(_ string) *AcliError {
			return &AcliError{
				Code:    ErrDeviceOffline,
				Message: "Device is offline.",
				Detail:  "The device is listed but not responding. Unplug and reconnect the USB cable, or restart the ADB server.",
				FixCmds: []string{
					"adb kill-server && adb start-server",
					"acli device list",
				},
			}
		},
	},
	{
		Tool:    "adb",
		Pattern: regexp.MustCompile(`(?i)INSTALL_FAILED_ALREADY_EXISTS|INSTALL_FAILED_UPDATE_INCOMPATIBLE`),
		Build: func(raw string) *AcliError {
			return &AcliError{
				Code:    ErrApkInstallFailed,
				Message: "APK installation failed: version conflict.",
				Detail:  "The app is already installed with an incompatible signature or newer version. Uninstall it first.",
				FixCmds: []string{
					"acli app uninstall <package-name>",
					"acli device install --reinstall <path/to/app.apk>",
				},
			}
		},
	},
	{
		Tool:    "adb",
		Pattern: regexp.MustCompile(`(?i)INSTALL_FAILED_INSUFFICIENT_STORAGE`),
		Build: func(_ string) *AcliError {
			return &AcliError{
				Code:    ErrApkInstallFailed,
				Message: "APK installation failed: not enough storage on device.",
				Detail:  "The device does not have enough free space to install the APK.",
				FixCmds: []string{
					"acli app list --third-party",
					"acli app clear <large-package>",
				},
			}
		},
	},
	{
		Tool:    "adb",
		Pattern: regexp.MustCompile(`(?i)INSTALL_FAILED`),
		Build: func(raw string) *AcliError {
			return &AcliError{
				Code:    ErrApkInstallFailed,
				Message: "APK installation failed.",
				Detail:  "adb reported: " + raw,
				FixCmds: []string{
					"acli device install --verbose <path/to/app.apk>",
				},
				DocsURL: "https://developer.android.com/tools/adb#pm",
			}
		},
	},

	// ── sdkmanager ───────────────────────────────────────────────────────
	{
		Tool:    "sdkmanager",
		Pattern: regexp.MustCompile(`(?i)accept.*\(y/N\)|licenses? not accepted|review.*license`),
		Build: func(_ string) *AcliError {
			return &AcliError{
				Code:    ErrLicenseNotAccepted,
				Message: "SDK licenses have not been accepted.",
				Detail:  "sdkmanager requires license acceptance before installing packages. This is common in fresh installs and CI environments.",
				FixCmds: []string{"acli sdk licenses"},
				DocsURL: "https://developer.android.com/tools/sdkmanager",
			}
		},
	},
	{
		Tool:    "sdkmanager",
		Pattern: regexp.MustCompile(`(?i)failed to find package|no.*package.*matching`),
		Build: func(raw string) *AcliError {
			return &AcliError{
				Code:    ErrInvalidPackage,
				Message: "SDK package not found.",
				Detail:  "The requested package path does not exist in the configured channels. Check spelling and channel.",
				FixCmds: []string{
					"acli sdk list --available",
					"acli sdk list --available --channel canary",
				},
				DocsURL: "https://developer.android.com/tools/sdkmanager",
			}
		},
	},
	{
		Tool:    "sdkmanager",
		Pattern: regexp.MustCompile(`(?i)connection refused|unable to reach|could not resolve host|network`),
		Build: func(_ string) *AcliError {
			return &AcliError{
				Code:    ErrNetworkError,
				Message: "Network error while contacting the SDK repository.",
				Detail:  "sdkmanager could not reach the Google repository. Check your internet connection or proxy settings.",
				FixCmds: []string{
					"sdkmanager --list --no_https",
					"acli sdk install --no-https <package>",
				},
			}
		},
	},

	// ── emulator ─────────────────────────────────────────────────────────
	{
		Tool:    "emulator",
		Pattern: regexp.MustCompile(`(?i)no such avd|avd.*not found`),
		Build: func(_ string) *AcliError {
			return &AcliError{
				Code:    ErrAVDNotFound,
				Message: "AVD not found.",
				Detail:  "The requested AVD does not exist. List available AVDs or create a new one.",
				FixCmds: []string{
					"acli avd list",
					"acli avd create <name> --api 35",
				},
			}
		},
	},
	{
		Tool:    "emulator",
		Pattern: regexp.MustCompile(`(?i)port.*in use|address already in use`),
		Build: func(_ string) *AcliError {
			return &AcliError{
				Code:    ErrPortInUse,
				Message: "Emulator port is already in use.",
				Detail:  "Another emulator or process is occupying the default ADB emulator port. Use --port to specify a different port.",
				FixCmds: []string{
					"acli avd list --running",
					"acli avd start <name> --port 5556",
				},
			}
		},
	},

	// ── Gradle ───────────────────────────────────────────────────────────
	{
		Tool:    "gradle",
		Pattern: regexp.MustCompile(`(?i)gradlew.*not found|could not find.*gradlew|no such file.*gradlew`),
		Build: func(_ string) *AcliError {
			return &AcliError{
				Code:    ErrGradleNotFound,
				Message: "Gradle wrapper not found.",
				Detail:  "No gradlew script was found in the current directory or any parent. Make sure you are inside an Android project.",
				FixCmds: []string{
					"acli project init <name>",
					"cd /path/to/your/android/project",
				},
			}
		},
	},
	{
		Tool:    "gradle",
		Pattern: regexp.MustCompile(`(?i)java.*heap space|OutOfMemoryError|GC overhead`),
		Build: func(_ string) *AcliError {
			return &AcliError{
				Code:    ErrOutOfMemory,
				Message: "Gradle ran out of memory.",
				Detail:  "The JVM heap was exhausted during the build. Increase the heap size in gradle.properties.",
				FixCmds: []string{
					`echo 'org.gradle.jvmargs=-Xmx4g -XX:MaxMetaspaceSize=512m' >> gradle.properties`,
					"acli build assemble",
				},
			}
		},
	},
	{
		Tool:    "gradle",
		Pattern: regexp.MustCompile(`(?i)build failed|compilation failed|task.*FAILED`),
		Build: func(_ string) *AcliError {
			return &AcliError{
				Code:    ErrBuildFailed,
				Message: "Gradle build failed.",
				Detail:  "See the build output above for the specific compilation or task error.",
				FixCmds: []string{
					"acli build assemble --verbose",
				},
			}
		},
	},
}

// Classify matches tool + stderr text against the catalog and returns a
// structured AcliError, or nil if the error is not recognized.
func Classify(tool, stderr string) *AcliError {
	for _, p := range catalog {
		if p.Tool == tool && p.Pattern.MatchString(stderr) {
			return p.Build(stderr)
		}
	}
	return nil
}
