package cmd

import (
	"fmt"
	"strconv"

	"github.com/android-cli/acli/internal/instrument"
	"github.com/android-cli/acli/pkg/android"
	"github.com/android-cli/acli/pkg/output"
	"github.com/spf13/cobra"
)

func newInstrumentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "instrument",
		Short: "Control device state for testing (battery, network, location, input)",
	}
	cmd.AddCommand(
		newInstrumentBatteryCmd(),
		newInstrumentNetworkCmd(),
		newInstrumentLocationCmd(),
		newInstrumentInputCmd(),
	)
	return cmd
}

func newInstrumentBatteryCmd() *cobra.Command {
	var flagLevel int
	var flagStatus string
	var flagReset bool

	cmd := &cobra.Command{
		Use:   "battery",
		Short: "Simulate battery level and charging state",
		Example: `  acli instrument battery --level 15
  acli instrument battery --status discharging
  acli instrument battery --level 5 --status discharging
  acli instrument battery --reset`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := instrument.New(loc)

			if flagReset {
				if err := svc.BatteryReset(cmd.Context(), deviceSerial()); err != nil {
					return handleErr(err)
				}
				output.Success("Battery simulation reset to real values.")
				return nil
			}

			if err := svc.Battery(cmd.Context(), deviceSerial(), flagLevel, flagStatus); err != nil {
				return handleErr(err)
			}
			if flagLevel >= 0 {
				output.Success("Battery level set to %d%%.", flagLevel)
			}
			if flagStatus != "" {
				output.Success("Battery status set to %s.", flagStatus)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&flagLevel, "level", -1, "Battery level 0-100")
	cmd.Flags().StringVar(&flagStatus, "status", "", "Battery status: charging, discharging, full, not-charging")
	cmd.Flags().BoolVar(&flagReset, "reset", false, "Reset battery simulation to real device values")
	return cmd
}

func newInstrumentNetworkCmd() *cobra.Command {
	var flagSpeed, flagLatency string

	cmd := &cobra.Command{
		Use:   "network",
		Short: "Simulate network speed and latency (emulators only)",
		Example: `  acli instrument network --speed edge
  acli instrument network --speed gprs --latency edge`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := instrument.New(loc)

			if err := svc.Network(cmd.Context(), deviceSerial(), flagSpeed, flagLatency); err != nil {
				return handleErr(err)
			}
			if flagSpeed != "" {
				output.Success("Network speed set to %s.", flagSpeed)
			}
			if flagLatency != "" {
				output.Success("Network latency set to %s.", flagLatency)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSpeed, "speed", "", "Network speed: full, hsdpa, edge, gprs, evdo, 1x, gsm")
	cmd.Flags().StringVar(&flagLatency, "latency", "", "Network latency: none, umts, edge, gprs")
	return cmd
}

func newInstrumentLocationCmd() *cobra.Command {
	var flagLat, flagLng float64

	cmd := &cobra.Command{
		Use:   "location",
		Short: "Set a mock GPS location",
		Example: `  acli instrument location --lat 37.7749 --lng -122.4194
  acli instrument location --lat 51.5074 --lng -0.1278`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := instrument.New(loc)

			if err := svc.Location(cmd.Context(), deviceSerial(), flagLat, flagLng); err != nil {
				return handleErr(err)
			}
			output.Success("Location set to %.4f, %.4f.", flagLat, flagLng)
			return nil
		},
	}
	cmd.Flags().Float64Var(&flagLat, "lat", 0, "Latitude (required)")
	cmd.Flags().Float64Var(&flagLng, "lng", 0, "Longitude (required)")
	_ = cmd.MarkFlagRequired("lat")
	_ = cmd.MarkFlagRequired("lng")
	return cmd
}

func newInstrumentInputCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "input",
		Short: "Send input events to the device",
	}

	textCmd := &cobra.Command{
		Use:   "text <text>",
		Short: "Type text on the device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := instrument.New(loc)
			if err := svc.InputText(cmd.Context(), deviceSerial(), args[0]); err != nil {
				return handleErr(err)
			}
			output.Success("Typed: %s", args[0])
			return nil
		},
	}

	tapCmd := &cobra.Command{
		Use:   "tap <x> <y>",
		Short: "Tap at screen coordinates",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			x, err := strconv.Atoi(args[0])
			if err != nil {
				return handleErr(fmt.Errorf("invalid x coordinate: %s", args[0]))
			}
			y, err := strconv.Atoi(args[1])
			if err != nil {
				return handleErr(fmt.Errorf("invalid y coordinate: %s", args[1]))
			}
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := instrument.New(loc)
			if err := svc.InputTap(cmd.Context(), deviceSerial(), x, y); err != nil {
				return handleErr(err)
			}
			output.Success("Tapped (%d, %d).", x, y)
			return nil
		},
	}

	keyCmd := &cobra.Command{
		Use:   "key <keycode>",
		Short: "Send a key event",
		Long: `Send an Android KeyEvent keycode. Common codes:
  KEYCODE_HOME  KEYCODE_BACK  KEYCODE_MENU  KEYCODE_POWER
  KEYCODE_ENTER KEYCODE_DEL   KEYCODE_VOLUME_UP KEYCODE_VOLUME_DOWN`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := instrument.New(loc)
			if err := svc.InputKey(cmd.Context(), deviceSerial(), args[0]); err != nil {
				return handleErr(err)
			}
			output.Success("Sent keycode: %s", args[0])
			return nil
		},
	}

	cmd.AddCommand(textCmd, tapCmd, keyCmd)
	return cmd
}
