package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"os"
)

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

const (
	underVoltOffset = 0x150
	tempOffset      = 0x1a2 // b29:24 Temperature Target
	powerLimitUnits = 0x606 // Definition of units for 0x610
	powerLimit      = 0x610 // PKG RAPL Power Limit Control (R/W)
)

// Convenience function to be replaced by proper CLI command
func doAllUndervolt() error {

	MSRFiles, err := GetAllMsrFiles()
	if err != nil {
		return err
	}

	// TODO(davidr) shame on you. fix this
	for cpu := range MSRFiles {
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

// RAPLPowerLimit is a struct corresponding to the PKG RAPL Power Limit Control
// MSR for a CPU
type RAPLPowerLimit struct {
	cpu        int     // CPU Id
	powerLimit float64 // package power limit in W
	enabled    bool
	clamping   bool    // no idea
	timeWindow float64 // window of time (in s) over which limit is calculated
}

// readRAPLPowerLimit returns a RAPLPowerLimit struct for cpu
func readRAPLPowerLimit(cpu int) (RAPLPowerLimit, error) {
	// This calculation is a bit odd. Register 0x606 has the information that defines
	// the units that we use in register 0x610, so we need to parse that first.
	rpl := RAPLPowerLimit{cpu: cpu}

	MSRFile, err := GetMsrFile(cpu)
	if err != nil {
		return rpl, err
	}

	rplUnitBitfield, err := readMSRIntValue(MSRFile, powerLimitUnits)
	if err != nil {
		return rpl, err
	}

	// powerUnits given in W, timeUnits in s
	powerUnits, timeUnits := getRAPLPowerUnits(rplUnitBitfield)

	rplBitfield, err := readMSRIntValue(MSRFile, powerLimit)
	if err != nil {
		return rpl, err
	}

	rpl.powerLimit = float64(rplBitfield&0x7fff) * powerUnits    // bits 14:0
	rpl.timeWindow = float64((rplBitfield>>17)&0x7f) * timeUnits // bits 23:17
	rpl.enabled = (rplBitfield>>15)&0x1 == 1                     // bit 15
	rpl.clamping = (rplBitfield>>16)&0x1 == 1                    // bit 16

	return rpl, nil
}

// getRAPLPowerUnits extracts the actual units in Watts and seconds from the 0x606 MSR register
func getRAPLPowerUnits(rplUnitBitfield int64) (float64, float64) {
	// For power-related info, the units are (2^p)^-1 mW where p is the uint from 3:0 in
	// the powerLimitUnits MSR
	pwrExponent := float64(rplUnitBitfield & 0x0f)
	powerUnits := 1 / math.Pow(2, pwrExponent)

	// Same for time in unit seconds, bits 19:16
	timeExponent := float64((rplUnitBitfield >> 16) & 0x0f)
	timeUnits := 1 / math.Pow(2, timeExponent)

	return powerUnits, timeUnits
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
	MSRFile, err := GetMsrFile(t.cpu)
	if err != nil {
		return fmt.Errorf("could not get MSR file for CPU %d: %s", t.cpu, err)
	}

	err = WriteMSRIntValue(MSRFile, tempOffset, uint64(newOffset<<24))
	if err != nil {
		return fmt.Errorf("could not set new offset for CPU %d: %s", t.cpu, err)
	}

	t.offset = newOffset
	return nil
}

// ReadTempTarget returns a TemperatureTarget struct for the cpu given in cpu
func ReadTempTarget(cpu int) (TemperatureTarget, error) {
	tempTarget := TemperatureTarget{cpu: cpu}

	// Temp target offset calculation:
	// Only the 29th-24th bits are relevant. Mask out 63rd-30th and shift right 24 bits
	// It's been a while since I've done this, so just for reference when I'm doing more
	// bit twiddling later on:
	//
	// 63    56 55    48 47    40 39    32 31    24 23    16 15     8 7      0
	// 00000000 00000000 00000000 00000000 00010100 01100100 00000000 00000000
	// mask: 00       00       00       00       7F       FF       FF       FF
	var tempOffsetMask int64 = 0x7fffffff
	// TODO(davidr) I don't think 7 is the right mask there

	// Same thing with bits 23:16 for the temperature target (right shift 16)
	var tempTargetMask int64 = 0xffffff

	MSRFile, err := GetMsrFile(cpu)
	if err != nil {
		return tempTarget, err
	}

	buf, err := readMSRIntValue(MSRFile, tempOffset)
	if err != nil {
		return tempTarget, err
	}

	tempTarget.offset = (buf & tempOffsetMask) >> 24
	tempTarget.target = (buf & tempTargetMask) >> 16
	return tempTarget, nil
}

// GetAllMsrFiles returns an array containing the /dev/cpu/XX/msr files for all CPUs on the
// system.
func GetAllMsrFiles() ([]string, error) {
	var MSRFiles []string

	cpus, err := GetAllCPUs()
	if err != nil {
		return MSRFiles, fmt.Errorf("could not get list of CPUS: %s", err)
	}

	for _, cpu := range cpus {
		msrfile, err := GetMsrFile(cpu)
		if err != nil {
			// TODO
			return MSRFiles, err
		}

		MSRFiles = append(MSRFiles, msrfile)
	}

	return MSRFiles, nil
}

// setVoltage sets the setPlane plane on cpu cpu to mVolts mV
func setVoltage(setPlane int, mVolts int, cpu int) error {
	if !isValidCPU(cpu) {
		return fmt.Errorf("msr: invalid CPU number %d", cpu)
	}

	MSRFile, err := GetMsrFile(cpu)
	if err != nil {
		return fmt.Errorf("msr: failed to set voltage on cpu %d: %s", cpu, err)
	}

	OffsetValue := calcUndervoltValue(setPlane, mVolts)
	fmt.Printf("OffsetValue: %#x\n", OffsetValue)
	err = WriteMSRIntValue(MSRFile, underVoltOffset, OffsetValue)
	if err != nil {
		fmt.Println("WriteMSRIntValue returned error: ", err)
		return err
	}

	return nil
}

func packOffset(offset uint32, plane int) uint64 {
	// I've deconstructed this calculation a little bit so I can remember how this magic
	// int64 was created. This is... not intuitive to me.
	planeBits := uint64(plane) << 40

	// I don't actually know what this bit does, but it seems to be needed
	unknownBit := uint64(1 << 36)

	// Set to 1 if we intend to write the value
	writeBit := uint64(1 << 32)

	return (1 << 63) | planeBits | writeBit | unknownBit | uint64(offset)
}

func calcUndervoltValue(plane int, offsetMv int) uint64 {
	// TODO(davidr) assert offset_mv < 0

	offset := uint32(math.Round(float64(offsetMv) * 1.024))

	// The actual value is only an 11 bit number, so we left-shift by 21
	offsetValue := 0xffe00000 & ((offset & 0xfff) << 21)
	return packOffset(offsetValue, plane)
}

func readMSRIntValue(msrFile string, MSRRegAddr int64) (int64, error) {
	fmt.Printf("readmsr: %#x\n", MSRRegAddr)

	var ReturnValue int64
	bytesValue := make([]byte, 8)

	file, err := os.OpenFile(msrFile, os.O_RDONLY, 0600)
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
func WriteMSRIntValue(msrFile string, MSRRegAddr int64, value uint64) error {
	fmt.Printf("writemsr: %#x\tval: %#x\n", MSRRegAddr, value)
	file, err := os.OpenFile(msrFile, os.O_WRONLY, 0600)
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

// GetMsrFile returns a string with the path the model specific register (msr) files associated
// with the given CPU. (i.e. "/dev/cpu/0/msr")
func GetMsrFile(cpu int) (string, error) {

	if !isValidCPU(cpu) {
		return "", fmt.Errorf("msr: invalid CPU number %d", cpu)
	}

	msrFile := fmt.Sprintf("/dev/cpu/%d/msr", cpu)
	// TODO(davidr) incorrect checking for whether stat succeeds
	if _, err := os.Stat(msrFile); os.IsNotExist(err) {
		return msrFile, err
	}

	return msrFile, nil
}
