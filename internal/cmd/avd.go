package cmd

import (
	"fmt"

	"github.com/android-cli/acli/internal/avd"
	"github.com/android-cli/acli/pkg/android"
	"github.com/android-cli/acli/pkg/output"
	"github.com/spf13/cobra"
)

func newAVDCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "avd",
		Short: "Manage Android Virtual Devices (wraps avdmanager + emulator)",
	}
	cmd.AddCommand(
		newAVDListCmd(),
		newAVDCreateCmd(),
		newAVDDeleteCmd(),
		newAVDStartCmd(),
		newAVDStopCmd(),
		newAVDImagesCmd(),
	)
	return cmd
}

// ── avd list ──────────────────────────────────────────────────────────────

func newAVDListCmd() *cobra.Command {
	var flagRunning bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Android Virtual Devices",
		RunE: func(cmd *cobra.Command, _ []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := avd.New(loc)

			avds, err := svc.List(cmd.Context(), flagRunning)
			if err != nil {
				return handleErr(err)
			}

			if len(avds) == 0 {
				output.Info("No AVDs found. Create one with: acli avd create <name> --api 35")
				return nil
			}

			headers := []string{"Name", "Target", "Tag/ABI", "Running"}
			var rows [][]string
			for _, a := range avds {
				running := "no"
				if a.Running {
					running = "yes"
				}
				rows = append(rows, []string{a.Name, a.Target, a.TagABI, running})
			}
			output.Table(headers, rows)
			return nil
		},
	}

	cmd.Flags().BoolVar(&flagRunning, "running", false, "Show only currently running AVDs")
	return cmd
}

// ── avd create ────────────────────────────────────────────────────────────

func newAVDCreateCmd() *cobra.Command {
	var (
		flagAPI    string
		flagDevice string
		flagABI    string
		flagSDCard string
		flagForce  bool
	)

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new Android Virtual Device",
		Long: `Create a new AVD. The system image must be installed first.

Examples:
  acli avd create Pixel9 --api 35
  acli avd create MyPhone --api 34 --device "pixel_7" --abi arm64-v8a
  acli avd create TestPhone --api 35 --sdcard 512M`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagAPI == "" {
				return handleErr(fmt.Errorf("--api is required"))
			}
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := avd.New(loc)

			name := args[0]
			output.Info("Creating AVD %q (API %s)…", name, flagAPI)
			if err := svc.Create(cmd.Context(), name, flagAPI, flagDevice, flagABI, flagSDCard, flagForce); err != nil {
				return handleErr(err)
			}
			output.Success("AVD %q created. Start it with: acli avd start %s", name, name)
			return nil
		},
	}

	cmd.Flags().StringVar(&flagAPI, "api", "", "Android API level (required), e.g. 35")
	cmd.Flags().StringVar(&flagDevice, "device", "", "Hardware device definition, e.g. pixel_7")
	cmd.Flags().StringVar(&flagABI, "abi", "x86_64", "System image ABI: x86_64, arm64-v8a")
	cmd.Flags().StringVar(&flagSDCard, "sdcard", "", "SD card size, e.g. 512M")
	cmd.Flags().BoolVarP(&flagForce, "force", "f", false, "Overwrite an existing AVD with the same name")
	_ = cmd.MarkFlagRequired("api")
	return cmd
}

// ── avd delete ────────────────────────────────────────────────────────────

func newAVDDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete an Android Virtual Device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := avd.New(loc)

			name := args[0]
			output.Info("Deleting AVD %q…", name)
			if err := svc.Delete(cmd.Context(), name); err != nil {
				return handleErr(err)
			}
			output.Success("AVD %q deleted.", name)
			return nil
		},
	}
	return cmd
}

// ── avd start ─────────────────────────────────────────────────────────────

func newAVDStartCmd() *cobra.Command {
	var (
		flagHeadless  bool
		flagPort      int
		flagWaitBoot  bool
	)

	cmd := &cobra.Command{
		Use:   "start <name>",
		Short: "Start an Android emulator",
		Long: `Launch an emulator for the named AVD.

Examples:
  acli avd start Pixel9
  acli avd start Pixel9 --headless --wait-boot
  acli avd start Pixel9 --port 5556`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := avd.New(loc)

			name := args[0]
			if flagHeadless {
				output.Info("Starting emulator %q in headless mode…", name)
			} else {
				output.Info("Starting emulator %q…", name)
			}

			if err := svc.Start(cmd.Context(), name, flagHeadless, flagPort, flagWaitBoot); err != nil {
				return handleErr(err)
			}
			if flagWaitBoot {
				output.Success("Emulator %q is ready.", name)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&flagHeadless, "headless", false, "Run without window/audio (ideal for CI)")
	cmd.Flags().IntVar(&flagPort, "port", 0, "ADB port for the emulator (default: 5554)")
	cmd.Flags().BoolVar(&flagWaitBoot, "wait-boot", false, "Block until the emulator has finished booting")
	return cmd
}

// ── avd stop ──────────────────────────────────────────────────────────────

func newAVDStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop <serial>",
		Short: "Stop a running emulator",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := avd.New(loc)

			serial := args[0]
			output.Info("Stopping emulator %q…", serial)
			if err := svc.Stop(cmd.Context(), serial); err != nil {
				return handleErr(err)
			}
			output.Success("Emulator stopped.")
			return nil
		},
	}
	return cmd
}

// ── avd images ────────────────────────────────────────────────────────────

func newAVDImagesCmd() *cobra.Command {
	var flagAPI string

	cmd := &cobra.Command{
		Use:   "images",
		Short: "List installable system images",
		RunE: func(cmd *cobra.Command, _ []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := avd.New(loc)

			images, err := svc.ListImages(cmd.Context(), flagAPI)
			if err != nil {
				return handleErr(err)
			}

			if len(images) == 0 {
				output.Info("No system images found. Install one with: acli sdk install \"system-images;android-35;google_apis;x86_64\"")
				return nil
			}

			headers := []string{"Path", "Version", "Description"}
			var rows [][]string
			for _, img := range images {
				rows = append(rows, []string{img.Path, img.Version, img.Description})
			}
			output.Table(headers, rows)
			return nil
		},
	}

	cmd.Flags().StringVar(&flagAPI, "api", "", "Filter by API level, e.g. 35")
	return cmd
}
