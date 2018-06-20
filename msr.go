package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"os"
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
//
// I cannot for the life of me find any docs from Intel on the voltage MSR. I must be
// searching for the wrong thing, but nothing seems to mention register address 0x150...
// ANYWHERE.
//
// As best I can tell, the voltage values are just reverse-engineered from Intel's
// tuning utilities.
type MSROffset int64

const (
	underVoltOffset = 0x150
	tempOffset      = 0x1a2 // b29:24 Temperature Target
	powerLimit      = 0x610 // PKG RAPL Power Limit Control (R/W)
)

// Convenience function to be replaced by proper CLI command
func doAllUndervolt() error {

	MSRFiles, err := GetMsrFiles(-1)
	if err != nil {
		return err
	}

	// TODO(davidr) shame on you. fix this
	for cpu, _ := range MSRFiles {
		err := setVoltage(cpuCorePlane, -115, cpu)
		if err != nil {
			fmt.Println(err)
			return err
		}

		err = setVoltage(cachePlane, -115, cpu)
		if err != nil {
			fmt.Println(err)
			return err
		}

		err = setVoltage(uncorePlane, -115, cpu)
		if err != nil {
			fmt.Println(err)
			return err
		}

		err = setVoltage(gpuPlane, -50, cpu)
		if err != nil {
			fmt.Println(err)
			return err
		}
	}

	return nil
}

// TemperatureTarget is a struct corresponding to the TEMPERATURE_TARGET MSR for
// an individual CPU
type TemperatureTarget struct {
	cpu    int   // CPU number
	target int64 // default thermal throttling activation temperature in degrees C
	offset int64 // offset from the default in degrees C at which to start throttling
}

// setThrottleTemp sets the throttle temperature for the CPU to temp by way of an offset
// from TemperatureTarget.target (e.g. if t.target == 100, then setThrottleTemp(90)
// will set t.offset to 10)
func (t *TemperatureTarget) setThrottleTemp(throttleTemp int64) error {
	// I think it's unlikely we'd ever want to set the offset higher than the default
	if throttleTemp > t.target {
		return fmt.Errorf("CPU throttling temperature cannot be higher than %d", t.target)
	}

	newOffset := t.target - throttleTemp
	if newOffset == t.offset {
		return nil
	}

	// we have a new value, now set it
	MSRFiles, err := GetMsrFiles(t.cpu)
	if err != nil {
		return fmt.Errorf("could not set temperature target for CPU %d: %s", t.cpu, err)
	}

	err = WriteMSRIntValue(MSRFiles[0], tempOffset, uint64(newOffset<<24))
	if err != nil {
		return fmt.Errorf("could not set new offset for CPU %d: %s", t.cpu, err)
	} else {
		t.offset = newOffset
	}

	return nil
}

// readTempTarget returns a TemperatureTarget struct for the cpu given in cpu
func readTempTarget(cpu int) (TemperatureTarget, error) {
	tempTarget := TemperatureTarget{cpu: cpu}

	// Temp target offset calculation:
	// Only the 29th-24th bits are relevant. Mask out 63rd-30th and shift right 24 bits
	// It's been a while since I've done this, so just for reference when I'm doing more
	// bit twiddling later on:
	//
	// 63    56 55    48 47    40 39    32 31    24 23    16 15     8 7      0
	// 00000000 00000000 00000000 00000000 00010100 01100100 00000000 00000000
	// mask: 00       00       00       00       7F       FF       FF       FF
	var tempOffsetMask int64 = 0x000000007fffffff

	// Same thing with bits 23:16 for the temperature target (right shift 16)
	var tempTargetMask int64 = 0x0000000000ffffff

	MSRFiles, err := GetMsrFiles(cpu)
	if err != nil {
		return tempTarget, err
	}

	buf, err := ReadMSRIntValue(MSRFiles[0], tempOffset)
	if err != nil {
		return tempTarget, err
	}

	tempTarget.offset = (buf & tempOffsetMask) >> 24
	tempTarget.target = (buf & tempTargetMask) >> 16
	return tempTarget, nil
}

func isValidCPU(cpu int) bool {

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

// setVoltage sets the setPlane plane on cpu cpu to mVolts mV
func setVoltage(setPlane VoltagePlane, mVolts int, cpu int) error {
	if !isValidCPU(cpu) {
		return fmt.Errorf("msr: invalid CPU number %d", cpu)
	}

	MSRFiles, err := GetMsrFiles(cpu)
	if err != nil {
		return fmt.Errorf("msr: failed to set voltage on cpu %d: %s", cpu, err)
	}

	OffsetValue := calcUndervoltValue(setPlane, mVolts)
	fmt.Printf("OffsetValue: %#x\n", OffsetValue)
	err = WriteMSRIntValue(MSRFiles[0], underVoltOffset, OffsetValue)
	if err != nil {
		fmt.Println("WriteMSRIntValue returned error: ", err)
		return err
	}

	return nil
}

func packOffset(offset uint32, plane VoltagePlane) uint64 {
	// I've deconstructed this calculation a little bit so I can remember how this magic
	// int64 was created. This is... not intuitive to me.
	planeBits := uint64(plane) << 40

	// I don't actually know what this bit does, but it seems to be needed
	unknownBit := uint64(1 << 36)

	// Set to 1 if we intend to write the value
	writeBit := uint64(1 << 32)

	return (1 << 63) | planeBits | writeBit | unknownBit | uint64(offset)
}

func calcUndervoltValue(plane VoltagePlane, offsetMv int) uint64 {
	// TODO(davidr) assert offset_mv < 0

	offset := uint32(math.Round(float64(offsetMv) * 1.024))

	// The actual value is only an 11 bit number, so we left-shift by 21
	offsetValue := 0xffe00000 & ((offset & 0xfff) << 21)
	return packOffset(offsetValue, plane)
}

func ReadMSRIntValue(msr_file string, MSRRegAddr int64) (int64, error) {
	fmt.Printf("readmsr: %#x\n", MSRRegAddr)

	var ReturnValue int64
	bytesValue := make([]byte, 8)

	file, err := os.OpenFile(msr_file, os.O_RDONLY, 0600)
	if err != nil {
		return ReturnValue, err
	}

	_, err = file.Seek(MSRRegAddr, 0)
	if err != nil {
		log.Fatalf("Unable to seek to %0x in msr_file", MSRRegAddr)
	}

	_, err = file.Read(bytesValue)
	if err != nil {
		return ReturnValue, err
	}

	buf := bytes.NewReader(bytesValue)
	err = binary.Read(buf, binary.LittleEndian, &ReturnValue)
	if err != nil {
		return ReturnValue, err
	}

	return ReturnValue, err
}

// WriteMSRIntValue packs a uint64 into a byte array and writes said array to the MSR file
// msr_file (i.e. for one spcific CPU) at location MSRRegAddr
func WriteMSRIntValue(msr_file string, MSRRegAddr int64, value uint64) error {
	fmt.Printf("writemsr: %#x\tval: %#x\n", MSRRegAddr, value)
	file, err := os.OpenFile(msr_file, os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	// Seek MSRRegAddr bytes from beginning of file
	_, err = file.Seek(MSRRegAddr, 0)
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
// with the CPUs on the system. If -1 is given for cpu, GetMsrFiles returns a slice of all
// CPUs' MSR files on the system In case of error, returns an empty slice.
func GetMsrFiles(cpu int) ([]string, error) {
	var msrFiles []string
	var err error

	if cpu == -1 {
		msrFiles, err = filepath.Glob("/dev/cpu/*/msr")
	} else {
		if !isValidCPU(cpu) {
			return msrFiles, fmt.Errorf("msr: invalid CPU number %d", cpu)
		}

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
