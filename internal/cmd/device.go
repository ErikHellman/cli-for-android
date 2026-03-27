package cmd

import (
	"github.com/android-cli/acli/internal/device"
	"github.com/android-cli/acli/pkg/android"
	"github.com/android-cli/acli/pkg/output"
	"github.com/spf13/cobra"
)

func newDeviceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "device",
		Short: "Manage connected devices and emulators (wraps adb)",
	}
	cmd.AddCommand(
		newDeviceListCmd(),
		newDeviceShellCmd(),
		newDeviceInstallCmd(),
		newDeviceUninstallCmd(),
		newDeviceLogsCmd(),
		newDevicePushCmd(),
		newDevicePullCmd(),
		newDeviceScreenshotCmd(),
		newDeviceRecordCmd(),
		newDeviceRebootCmd(),
		newDeviceForwardCmd(),
		newDeviceReverseCmd(),
		newDevicePairCmd(),
		newDeviceConnectCmd(),
		newDeviceInfoCmd(),
	)
	return cmd
}

func newDeviceListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List connected devices and emulators",
		RunE: func(cmd *cobra.Command, _ []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := device.New(loc)
			devices, err := svc.List(cmd.Context())
			if err != nil {
				return handleErr(err)
			}
			if len(devices) == 0 {
				output.Info("No devices connected. Start an emulator with: acli avd start <name>")
				return nil
			}
			headers := []string{"Serial", "State", "Model", "Product", "Type"}
			var rows [][]string
			for _, d := range devices {
				kind := "physical"
				if d.IsEmu {
					kind = "emulator"
				}
				rows = append(rows, []string{d.Serial, d.State, d.Model, d.Product, kind})
			}
			output.Table(headers, rows)
			return nil
		},
	}
}

func newDeviceShellCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "shell [command]",
		Short: "Open an interactive shell or run a one-shot command",
		Example: `  acli device shell
  acli device shell ls /sdcard
  acli -d emulator-5554 device shell pm list packages`,
		RunE: func(cmd *cobra.Command, args []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := device.New(loc)
			shellCmd := ""
			if len(args) > 0 {
				shellCmd = joinArgs(args)
			}
			return handleErr(svc.Shell(cmd.Context(), deviceSerial(), shellCmd))
		},
	}
}

func newDeviceInstallCmd() *cobra.Command {
	var flagGrantAll, flagReinstall, flagDowngrade bool

	cmd := &cobra.Command{
		Use:   "install <path/to/app.apk>",
		Short: "Install an APK on the target device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := device.New(loc)
			output.Info("Installing %s…", args[0])
			if err := svc.Install(cmd.Context(), deviceSerial(), args[0], flagGrantAll, flagReinstall, flagDowngrade); err != nil {
				return handleErr(err)
			}
			output.Success("Installed successfully.")
			return nil
		},
	}
	cmd.Flags().BoolVarP(&flagGrantAll, "grant-all", "g", false, "Grant all runtime permissions")
	cmd.Flags().BoolVarP(&flagReinstall, "reinstall", "r", false, "Reinstall, keeping data")
	cmd.Flags().BoolVarP(&flagDowngrade, "downgrade", "d", false, "Allow version downgrade")
	return cmd
}

func newDeviceUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall <package>",
		Short: "Uninstall an app by package name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := device.New(loc)
			output.Info("Uninstalling %s…", args[0])
			if err := svc.Uninstall(cmd.Context(), deviceSerial(), args[0]); err != nil {
				return handleErr(err)
			}
			output.Success("Uninstalled %s.", args[0])
			return nil
		},
	}
}

func newDeviceLogsCmd() *cobra.Command {
	var flagLevel, flagTag string
	var flagFollow, flagClear bool

	cmd := &cobra.Command{
		Use:   "logs [tag]",
		Short: "Stream or dump device logs (logcat)",
		Example: `  acli device logs
  acli device logs --follow --level E
  acli device logs MyApp --level D
  acli device logs --clear`,
		RunE: func(cmd *cobra.Command, args []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := device.New(loc)
			tag := ""
			if len(args) > 0 {
				tag = args[0]
			}
			return handleErr(svc.Logs(cmd.Context(), deviceSerial(), tag, flagLevel, flagFollow, flagClear))
		},
	}
	cmd.Flags().StringVarP(&flagLevel, "level", "l", "", "Log level filter: V, D, I, W, E, F")
	cmd.Flags().StringVar(&flagTag, "tag", "", "Filter by log tag")
	cmd.Flags().BoolVarP(&flagFollow, "follow", "f", false, "Stream logs continuously (like tail -f)")
	cmd.Flags().BoolVar(&flagClear, "clear", false, "Clear the log buffer before dumping")
	return cmd
}

func newDevicePushCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "push <local> <remote>",
		Short: "Copy a local file to the device",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := device.New(loc)
			if err := svc.Push(cmd.Context(), deviceSerial(), args[0], args[1]); err != nil {
				return handleErr(err)
			}
			output.Success("Pushed %s → %s", args[0], args[1])
			return nil
		},
	}
}

func newDevicePullCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pull <remote> [local]",
		Short: "Copy a file from the device to the host",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := device.New(loc)
			local := "."
			if len(args) > 1 {
				local = args[1]
			}
			if err := svc.Pull(cmd.Context(), deviceSerial(), args[0], local); err != nil {
				return handleErr(err)
			}
			output.Success("Pulled %s → %s", args[0], local)
			return nil
		},
	}
}

func newDeviceScreenshotCmd() *cobra.Command {
	var flagOutput string

	cmd := &cobra.Command{
		Use:   "screenshot [output.png]",
		Short: "Capture a screenshot from the device",
		RunE: func(cmd *cobra.Command, args []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := device.New(loc)
			out := flagOutput
			if out == "" && len(args) > 0 {
				out = args[0]
			}
			if err := svc.Screenshot(cmd.Context(), deviceSerial(), out); err != nil {
				return handleErr(err)
			}
			if out == "" {
				output.Success("Screenshot saved.")
			} else {
				output.Success("Screenshot saved to %s", out)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&flagOutput, "output", "o", "", "Output file path (default: screenshot-<timestamp>.png)")
	return cmd
}

func newDeviceRecordCmd() *cobra.Command {
	var flagDuration int
	var flagSize, flagOutput string

	cmd := &cobra.Command{
		Use:   "record [output.mp4]",
		Short: "Record the device screen",
		RunE: func(cmd *cobra.Command, args []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := device.New(loc)
			out := flagOutput
			if out == "" && len(args) > 0 {
				out = args[0]
			}
			output.Info("Recording screen (press Ctrl+C to stop)…")
			if err := svc.Record(cmd.Context(), deviceSerial(), flagDuration, flagSize, out); err != nil {
				return handleErr(err)
			}
			output.Success("Recording saved.")
			return nil
		},
	}
	cmd.Flags().IntVar(&flagDuration, "duration", 0, "Recording duration in seconds (max 180)")
	cmd.Flags().StringVar(&flagSize, "size", "", "Video size, e.g. 1280x720 (default: device native)")
	cmd.Flags().StringVarP(&flagOutput, "output", "o", "", "Output file path")
	return cmd
}

func newDeviceRebootCmd() *cobra.Command {
	var flagBootloader, flagRecovery bool

	cmd := &cobra.Command{
		Use:   "reboot",
		Short: "Reboot the device",
		RunE: func(cmd *cobra.Command, _ []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := device.New(loc)
			output.Info("Rebooting device…")
			if err := svc.Reboot(cmd.Context(), deviceSerial(), flagBootloader, flagRecovery); err != nil {
				return handleErr(err)
			}
			output.Success("Reboot initiated.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagBootloader, "bootloader", false, "Reboot into bootloader mode")
	cmd.Flags().BoolVar(&flagRecovery, "recovery", false, "Reboot into recovery mode")
	return cmd
}

func newDeviceForwardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "forward <local-port> <remote-port>",
		Short: "Forward a host TCP port to a device TCP port",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := device.New(loc)
			if err := svc.Forward(cmd.Context(), deviceSerial(), args[0], args[1]); err != nil {
				return handleErr(err)
			}
			output.Success("Forwarding localhost:%s → device:%s", args[0], args[1])
			return nil
		},
	}
}

func newDeviceReverseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reverse <remote-port> <local-port>",
		Short: "Reverse-forward a device TCP port to a host TCP port",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := device.New(loc)
			if err := svc.Reverse(cmd.Context(), deviceSerial(), args[0], args[1]); err != nil {
				return handleErr(err)
			}
			output.Success("Reverse forward device:%s → localhost:%s", args[0], args[1])
			return nil
		},
	}
}

func newDevicePairCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pair <ip:port>",
		Short: "Pair a device over Wi-Fi (Android 11+)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := device.New(loc)
			return handleErr(svc.Pair(cmd.Context(), args[0]))
		},
	}
}

func newDeviceConnectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "connect <ip:port>",
		Short: "Connect to a device over Wi-Fi",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := device.New(loc)
			if err := svc.Connect(cmd.Context(), args[0]); err != nil {
				return handleErr(err)
			}
			output.Success("Connected to %s", args[0])
			return nil
		},
	}
}

func newDeviceInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show device information",
		RunE: func(cmd *cobra.Command, _ []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := device.New(loc)
			info, err := svc.Info(cmd.Context(), deviceSerial())
			if err != nil {
				return handleErr(err)
			}

			headers := []string{"Property", "Value"}
			var rows [][]string
			labels := map[string]string{
				"serial":                    "Serial",
				"ro.product.model":          "Model",
				"ro.build.version.release":  "Android Version",
				"ro.build.version.sdk":      "API Level",
				"ro.product.cpu.abi":        "ABI",
				"ro.product.manufacturer":   "Manufacturer",
			}
			order := []string{
				"serial",
				"ro.product.manufacturer",
				"ro.product.model",
				"ro.build.version.release",
				"ro.build.version.sdk",
				"ro.product.cpu.abi",
			}
			for _, k := range order {
				if v, ok := info[k]; ok && v != "" {
					label := labels[k]
					if label == "" {
						label = k
					}
					rows = append(rows, []string{label, v})
				}
			}
			output.Table(headers, rows)
			return nil
		},
	}
}

// joinArgs joins CLI args back into a single shell command string.
func joinArgs(args []string) string {
	result := ""
	for i, a := range args {
		if i > 0 {
			result += " "
		}
		result += a
	}
	return result
}
