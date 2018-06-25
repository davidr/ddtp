package cmd

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/davidr/ddtp/pkg/msr"
	"github.com/davidr/ddtp/pkg/util"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var tempCmd = &cobra.Command{
	Use:   "temp",
	Short: "Package Temperature Target Interface",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("temp command...")
		fmt.Println(msr.GetAllMsrFiles())
		fmt.Println(cpuFlag, verboseFlag)
	},
}

var tempListCmd = &cobra.Command{
	Use:   "list",
	Short: "List throttle temperature value(s)",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		// TODO(davidr) - check for error
		listTemp(cpuFlag)
	},
}

var tempSetCmd = &cobra.Command{
	Use:   "set TEMPERATURE",
	Short: "Set throttle temperature value(s)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Read temperature into int64 to process change
		throttleTemp, err := strconv.Atoi(args[0])
		if err != nil {
			log.Fatal("Could not parse argument into temperature: ", err)
		}

		setTemp(cpuFlag, throttleTemp)
	},
}

func init() {
	tempCmd.AddCommand(tempListCmd)
	tempCmd.AddCommand(tempSetCmd)
	rootCmd.AddCommand(tempCmd)
}

func setTemp(cpu int, throttleTemp int) error {
	if cpu == -1 {
		return setAllTemps(throttleTemp)
	}

	fmt.Println("setting CPU", cpu, "to", throttleTemp)

	tt, err := msr.GetTempTarget(cpu)
	if err != nil {
		return fmt.Errorf("could not read temperature target data: %s", err)
	}

	err = tt.SetThrottleTemp(throttleTemp)
	if err != nil {
		return fmt.Errorf("unable to set throttling temperature: %s", err)
	}

	return nil
}

func setAllTemps(throttleTemp int) error {
	// Set a counter for the number of throttle temps we've changed so that we know if we've
	// left the cpu temperature limits in an inconsistent state because we've died halfway through
	tempChangeCounter := 0

	cpus, err := util.GetAllCPUs()
	if err != nil {
		log.Fatal("Could not get list of CPUs: ", err)
		return err
	}

	for _, cpu := range cpus {
		err := setTemp(cpu, throttleTemp)
		if err != nil {
			if tempChangeCounter > 0 {
				log.Printf("WARNING: inconsistent state. %d CPUs' limits were altered before error\n", tempChangeCounter)
			}
			log.Fatal("unable to set throttling temperature: ", err)
		}

		tempChangeCounter++
	}

	return nil
}

func listTemp(cpu int) error {
	// pretty-print out a table of the values if we need to do this for all CPUs
	if cpu == -1 {
		return listAllTemps()
	}

	tt, err := msr.GetTempTarget(cpu)
	if err != nil {
		log.Fatal("could not retreive temperature information: ", err)
	}

	fmt.Println(tt.GetThrottleTemp())
	return nil
}

func listAllTemps() error {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"CPU", "Throttle Temp"})
	table.SetBorder(false)

	cpus, err := util.GetAllCPUs()
	if err != nil {
		log.Fatal("Could not get list of CPUs: ", err)
		return err
	}

	for _, cpu := range cpus {
		tt, err := msr.GetTempTarget(cpu)
		if err != nil {
			log.Fatal("Could not read temperature target data: ", err)
		}

		table.Append([]string{strconv.Itoa(cpu), strconv.Itoa(int(tt.GetThrottleTemp()))})
	}

	table.Render()
	return nil
}
