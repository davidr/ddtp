package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	cpuFlag     int
	verboseFlag bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "ddtp",
	Short: "utilities for power management on laptops",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// config defaults
	cpuDefault := -1

	rootCmd.PersistentFlags().IntVarP(&cpuFlag, "cpu", "c", cpuDefault, "CPU Number (Default: ALL CPUs)")
	rootCmd.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "Verbose output")

}
