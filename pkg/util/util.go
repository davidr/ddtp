package util

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
)

// GetAllCPUs returns a list of integers corresponding to all CPUs on the system (e.g. on
// a 4-core system, GetAllCPUs will return [0, 1, 2, 3]. This listing is obtained by globbing
// /dev/cpu/[0-9]*
func GetAllCPUs() ([]int, error) {
	var cpus []int

	cpuDirs, err := filepath.Glob("/dev/cpu/[0-9]*")
	if err != nil {
		return cpus, err
	}

	for _, cpuDir := range cpuDirs {
		cpuDirBase := path.Base(cpuDir)

		cpuID, _ := strconv.Atoi(cpuDirBase)
		cpus = append(cpus, cpuID)
	}

	// Just a sanity check to make sure we found *something*
	if len(cpus) == 0 {
		return cpus, fmt.Errorf("found no CPUs")
	}

	return cpus, nil
}

func IsValidCPU(cpu int) bool {

	// CPU must be a nonnegative integer
	if cpu < 0 {
		return false
	}

	// Does this CPU have a directory under /dev/cpu?
	cpuDir := fmt.Sprintf("/dev/cpu/%d", cpu)
	if _, err := os.Stat(cpuDir); os.IsNotExist(err) {
		return false
	}

	return true
}
