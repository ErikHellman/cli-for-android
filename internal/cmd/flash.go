package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/android-cli/acli/internal/flash"
	"github.com/android-cli/acli/pkg/android"
	"github.com/android-cli/acli/pkg/output"
	"github.com/spf13/cobra"
)

func newFlashCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "flash",
		Short: "Flash device partitions via fastboot",
	}
	cmd.AddCommand(
		newFlashListCmd(),
		newFlashRebootCmd(),
		newFlashImageCmd(),
		newFlashFactoryCmd(),
		newFlashUnlockCmd(),
	)
	return cmd
}

func newFlashListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List devices in fastboot mode",
		RunE: func(cmd *cobra.Command, _ []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := flash.New(loc)
			devices, err := svc.List(cmd.Context())
			if err != nil {
				return handleErr(err)
			}
			if len(devices) == 0 {
				output.Info("No fastboot devices. Reboot to bootloader with: acli device reboot --bootloader")
				return nil
			}
			headers := []string{"Serial", "State"}
			var rows [][]string
			for _, d := range devices {
				rows = append(rows, []string{d.Serial, d.State})
			}
			output.Table(headers, rows)
			return nil
		},
	}
}

func newFlashRebootCmd() *cobra.Command {
	var flagBootloader bool

	cmd := &cobra.Command{
		Use:   "reboot",
		Short: "Reboot from fastboot mode",
		RunE: func(cmd *cobra.Command, _ []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := flash.New(loc)
			return handleErr(svc.Reboot(cmd.Context(), deviceSerial(), !flagBootloader, flagBootloader))
		},
	}
	cmd.Flags().BoolVar(&flagBootloader, "bootloader", false, "Reboot back into bootloader")
	return cmd
}

func newFlashImageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "image <partition> <file>",
		Short: "Flash a single partition image",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := flash.New(loc)
			output.Info("Flashing %s → %s…", args[1], args[0])
			if err := svc.FlashImage(cmd.Context(), deviceSerial(), args[0], args[1]); err != nil {
				return handleErr(err)
			}
			output.Success("Flashed %s.", args[0])
			return nil
		},
	}
}

func newFlashFactoryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "factory <factory-image.zip>",
		Short: "Flash a complete factory image",
		Long: `Flashes all partitions from a factory image zip file.
WARNING: This will erase all data on the device.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !confirmDangerous("This will ERASE ALL DATA on the device. Continue?") {
				output.Info("Aborted.")
				return nil
			}
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := flash.New(loc)
			output.Info("Flashing factory image %s…", args[0])
			if err := svc.FlashFactory(cmd.Context(), deviceSerial(), args[0]); err != nil {
				return handleErr(err)
			}
			output.Success("Factory image flashed.")
			return nil
		},
	}
}

func newFlashUnlockCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unlock",
		Short: "Unlock the bootloader (OEM unlock)",
		Long: `Sends the OEM bootloader unlock command via fastboot.
WARNING: This will factory reset the device and may void your warranty.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !confirmDangerous("Bootloader unlock will FACTORY RESET the device. Continue?") {
				output.Info("Aborted.")
				return nil
			}
			loc, err := android.New()
			if err != nil {
				return handleErr(err)
			}
			svc := flash.New(loc)
			if err := svc.Unlock(cmd.Context(), deviceSerial()); err != nil {
				return handleErr(err)
			}
			output.Success("Bootloader unlock command sent.")
			return nil
		},
	}
}

// confirmDangerous prompts the user for y/n confirmation on destructive ops.
// Returns true if the user confirms.
func confirmDangerous(prompt string) bool {
	if globalFlags.JSON {
		// In JSON/automated mode, do not proceed with destructive ops without explicit flag
		return false
	}
	fmt.Printf("\n⚠  %s [y/N] ", prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		resp := strings.TrimSpace(strings.ToLower(scanner.Text()))
		return resp == "y" || resp == "yes"
	}
	return false
}
