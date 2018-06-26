package cmd

import (
	"fmt"
	"os"
	"sort"
	"strconv"

	"github.com/davidr/ddtp/pkg/msr"
	"github.com/olekukonko/tablewriter"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var voltCmd = &cobra.Command{
	Use:   "volt",
	Short: "Under/Overvolt Interface",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("volt command...")
		fmt.Println(cpuFlag, verboseFlag)
	},
}

var voltListCmd = &cobra.Command{
	Use:   "list",
	Short: "List plane voltage offset value(s) (in mV)",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// with no arguments, get the voltage offset for all planes and display in a table
		if len(args) == 0 {
			log.Info("displaying all voltage planes")
			listAllPlaneVoltage()
			return
		}

		plane, ok := msr.VoltagePlanes[args[0]]
		log.Infof("requesting info for plane %s", args[0])
		if !ok {
			log.Fatalf("Invalid plane '%s'", args[0])
		}

		listPlaneVoltage(plane)

	},
}

var voltSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set plane voltage value",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("volt set...")

	},
}

func init() {
	voltCmd.AddCommand(voltListCmd)
	voltCmd.AddCommand(voltSetCmd)
	rootCmd.AddCommand(voltCmd)
}

func listPlaneVoltage(plane int) error {
	voltageOffset, err := msr.GetVoltage(plane, cpuFlag)
	if err != nil {
		log.Fatalf("could not retreive voltage offset for plane %d on cpu %d: %s", plane, cpuFlag, err)
	}
	fmt.Println(voltageOffset)
	return nil
}

// For convenience, this table should be sorted by plane integer number, but the map is
// unsorted. Generate a list of map keys sorted by map value. There's for sure more
// efficient ways to do this, but it's such a miniscule map, I'm not going to bother to
// do it any other way
func getPlaneListSortedByPlane() []string {
	flippedMap := map[int]string{}
	for k, v := range msr.VoltagePlanes {
		flippedMap[v] = k
	}

	var planes []int
	for k := range flippedMap {
		planes = append(planes, k)
	}

	sort.Ints(planes)
	var planesByInt []string
	for k := range planes {
		planesByInt = append(planesByInt, flippedMap[k])
	}

	return planesByInt
}

func listAllPlaneVoltage() error {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"plane", "offset in millivolts"})
	table.SetBorder(false)

	planeNames := getPlaneListSortedByPlane()
	for _, planeName := range planeNames {
		voltageOffset, err := msr.GetVoltage(msr.VoltagePlanes[planeName], cpuFlag)
		if err != nil {
			log.Fatalf("could not get data from voltage planes: %s", err)
		}

		table.Append([]string{planeName, strconv.Itoa(voltageOffset)})
	}

	table.Render()
	return nil
}
