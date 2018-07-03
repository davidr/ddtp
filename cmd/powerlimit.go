package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/davidr/ddtp/pkg/msr"
)

var powerlimitCmd = &cobra.Command{
	Use:   "powerlimit",
	Short: "Under/Overpowerlimit Interface",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("powerlimit command...")
		fmt.Println(cpuFlag, verboseFlag)
	},
}

var powerlimitListCmd = &cobra.Command{
	Use:   "list",
	Short: "List Package Running Average Power Limit (RAPL)",
	Args:  cobra.MaximumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("powerlimit list...")
		powerlimit, err := msr.GetRAPLPowerLimit(cpuFlag)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(powerlimit)
	},
}

var powerlimitSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set plane powerlimitage value",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("powerlimit set...")

	},
}

func init() {
	powerlimitCmd.AddCommand(powerlimitListCmd)
	powerlimitCmd.AddCommand(powerlimitSetCmd)
	rootCmd.AddCommand(powerlimitCmd)
}
