package main

import (
	"fmt"
	"os"
	"testing"
)

func TestInvalidCPU(t *testing.T) {
	t.Log("Testing invalid CPU detection")

	if !isValidCPU(0) {
		t.Errorf("Cpu 0 is valid")
	}

	if isValidCPU(-1) {
		t.Errorf("Negative CPU number is not valid")
	}

	// Find the first CPU number for which there is no /dev directory. Set the limit at 4096
	// cores. If you've got more than 4096 cores, what the hell are you doing running this?
	for i := 0; i < 4096; i++ {
		cpuDir := fmt.Sprintf("/dev/cpu/%d", i)
		if _, err := os.Stat(cpuDir); os.IsNotExist(err) {
			if isValidCPU(i) {
				t.Errorf("nonexistent CPU %d is not valid", i)
			} else {
				break
			}
		}
	}
}
