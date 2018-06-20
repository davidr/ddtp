package main

import (
	"fmt"
	"os"
	"io/ioutil"
	"strconv"
	"strings"
)


func main() {
	// TODO(davidr) - unless dry run is enabled, this should cause the program to fail
	fmt.Printf("EUID == 0: %t\n", isRoot())

	// doUndervolt()
	fmt.Println(readTempTarget(0))

}

// A convenience function to tell whether or not we're running as root. Presumably this will
// get a bit more complicated later on.
func isRoot() bool {
	return os.Geteuid() == 0
}

func OnBatteryPower() (bool, error) {
	var ACOnlineSysFile string = "/sys/class/power_supply/AC/online"

	ACOnlineContents, err := ioutil.ReadFile(ACOnlineSysFile)
	if err != nil {
		fmt.Println("a")
		return false, err
	}

	ACOnlineString := strings.TrimSuffix(string(ACOnlineContents), "\n")
	IsOnline, err := strconv.Atoi(ACOnlineString)
	if err != nil {
		return false, err
	}

	return IsOnline == 0, nil
}
