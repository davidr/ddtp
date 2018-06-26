package cmd

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	cpuFlag     int
	verboseFlag bool
	debugFlag   bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "ddtp",
	Short: "utilities for power management on laptops",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		log.SetOutput(os.Stdout)

		if debugFlag {
			log.SetLevel(log.DebugLevel)
			log.Debug("debug logging enabled")
		} else if verboseFlag {
			log.SetLevel(log.InfoLevel)
			log.Info("verbose logging enabled")
		} else {
			log.SetLevel(log.WarnLevel)
		}
	},
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
	cpuDefault := 0

	rootCmd.PersistentFlags().IntVarP(&cpuFlag, "cpu", "c", cpuDefault, "CPU Number (Default: 0)")
	rootCmd.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVarP(&debugFlag, "debug", "d", false, "Debug output")
}
