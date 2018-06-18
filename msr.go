package main

import (
	"math"
	"fmt"
	"os"
	"bytes"
	"encoding/binary"
	"path/filepath"
)

type VoltagePlane int
const (
	cpuCorePlane  = 0
	gpuPlane      = 1
	cachePlane    = 2
	uncorePlane   = 3
	analogioPlane = 4
)


// https://software.intel.com/sites/default/files/managed/22/0d/335592-sdm-vol-4.pdf
type MSROffset int64
const (
	underVoltOffset = 0x150 // I cannot for the life of me find any docs from Intel on this
	                        // MSR. I must be searching for the wrong thing, but nothing seems
	                        // to mention register address 0x150... ANYWHERE. Thanks Obama.
	tempOffset      = 0x1a2 // Temperature Target
	powerLimit      = 0x610 // PKG RAPL Power Limit Control (R/W)
)


// Convenience funciton to be replaced by proper CLI command
func doUndervolt() error {
	err := setUndervolt(cpuCorePlane, -115, -1)
	if err != nil {
		return err
	}

	err = setUndervolt(cachePlane, -115, -1)
	if err != nil {
		return err
	}

	err = setUndervolt(uncorePlane, -115, -1)
	if err != nil {
		return err
	}

	err = setUndervolt(gpuPlane, -50, -1)
	if err != nil {
		return err
	}

	return nil
}


// setUndervolt sets the setPlane plane on cpu cpu to mVolts mV. If cpu is -1,
// setUndervolt sets the value on all CPUs in the system.
func setUndervolt(setPlane VoltagePlane, mVolts int, cpu int) error {
	MSRFiles, err := GetMsrFiles(cpu)
	if err != nil {
		return err
	}

	OffsetValue := calcUndervoltValue(setPlane, mVolts)
	fmt.Printf("OffsetValue: %#x\n", OffsetValue)
	for _, MSRFile := range MSRFiles {
		err := WriteMSRValue(MSRFile, underVoltOffset, OffsetValue)
		if err != nil {
			fmt.Println("WriteMSRValue returned error: ", err)
			return err
		}
	}

	return nil
}

func packOffset(offset uint32, plane VoltagePlane) uint64 {
	// I've deconstructed this calculation a little bit so I can remember how this magic
	// int64 was created. This is... not intuitive to me.

	// I don't actually know what this bit does, but it seems to be needed
	unknownBit := uint64(1 << 36)

	writeBit := uint64(1 << 32)
	planeBits := uint64(plane) << 40

	return (1 << 63) | planeBits | writeBit | unknownBit | uint64(offset)
}

func calcUndervoltValue(plane VoltagePlane, offsetMv int) uint64 {
	// TODO(davidr) assert offset_mv < 0

	offset := uint32(math.Round(float64(offsetMv) * 1.024))

	// The actual value is only an 11 bit number, so we left-shift by 21
	offsetValue := 0xffe00000 & ((offset & 0xfff) << 21)
	return packOffset(offsetValue, plane)
}

func WriteMSRValue(msr_file string, msr_offset int64, value uint64) error {
	fmt.Printf("writemsr: %#x\tval: %#x\n", msr_offset, value)
	file, err := os.OpenFile(msr_file, os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	// Seek msr_offset bytes from beginning of file
	_, err = file.Seek(msr_offset, 0)
	if err != nil {
		// Something's gone seriously wrong. We've opened the MSR file WRONLY, but somehow can't
		// seek in it?
		return err
	}

	buff := new(bytes.Buffer)
	err = binary.Write(buff, binary.LittleEndian, value)
	_, err = file.Write(buff.Bytes())
	if err != nil {
		return err
	}

	file.Close()
	return nil
}

// Return a slice of strings of all of the model specific register (msr) files associated
// with the CPUs on the system. In case of error, returns an empty slice.
func GetMsrFiles(cpu int) ([]string, error) {
	var msrFiles []string
	var err error

	if cpu == -1 {
		msrFiles, err = filepath.Glob("/dev/cpu/*/msr")
	} else {
		cpumsrFile := fmt.Sprintf("/dev/cpu/%d/msr", cpu)
		_, err = os.Stat(cpumsrFile)
		msrFiles = append(msrFiles, cpumsrFile)
	}

	if err != nil {
		fmt.Println("Error getting MSR file(s): ", err)
		var emptylist []string
		return emptylist, err
	}
	return msrFiles, nil
}

