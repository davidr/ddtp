package msr

import (
	"fmt"
	"math"
)

// VoltagePlanes is a simple map from a logical voltage plane name to its integer
// value as required by MSR 0x150
var VoltagePlanes = map[string]int{
	"cpu":      0,
	"gpu":      1,
	"cache":    2,
	"uncore":   3,
	"analogio": 4,
}

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

// SetVoltage sets the voltagePlane plane on cpu cpu to mVolts mV
func SetVoltage(voltagePlane int, mVolts int, cpu int) error {
	MSRFile, err := GetMsrFile(cpu)
	if err != nil {
		return fmt.Errorf("msr: failed to set voltage on cpu %d: %s", cpu, err)
	}

	OffsetValue := calcUndervoltValue(voltagePlane, mVolts)
	fmt.Printf("OffsetValue: %#x\n", OffsetValue)
	err = WriteMSRIntValue(MSRFile, underVoltOffset, OffsetValue)
	if err != nil {
		fmt.Println("WriteMSRIntValue returned error: ", err)
		return err
	}

	return nil
}

// GetVoltage gets the voltage offset in mV for the requested plane on the requested CPU
func GetVoltage(voltagePlane int, cpu int) (int, error) {
	MSRFile, err := GetMsrFile(cpu)
	if err != nil {
		return 0, fmt.Errorf("msr: failed to get msrfile cpu %d: %s", cpu, err)
	}

	// I think to read the value associated with a voltage plane, you have to write a
	// "read" request (i.e. one without the write bit set) to the MSR and then turn
	// around and read it.
	//
	// I have no idea what I'm doing.
	readOffset := packOffset(0, voltagePlane, false)
	err = WriteMSRIntValue(MSRFile, underVoltOffset, readOffset)
	if err != nil {
		return 0, fmt.Errorf("msr: could not write read request to MSR: %s", err)
	}

	registerData, err := readMSRIntValue(MSRFile, underVoltOffset)
	if err != nil {
		return 0, fmt.Errorf("msr: could not read value from MSR: %s", err)
	}

	var plane, offset int
	unpackOffset(&plane, &offset, registerData)
	return offset, nil
}

func unpackOffset(voltagePlane *int, voltageOffset *int, registerData uint64) error {
	// Some CPUs include the plane in the reponse
	p := (registerData >> 40) & 0xf
	*voltagePlane = int(p)

	// The number is 11 bits starting at bit 31, so shift right 21 and mask off all
	// but the last 11 bits. Since the register data is being read as uint64, we need
	// to cast to an int here before we can start doing meaningful arithmetic.
	o := int((registerData >> 21) & 0x7ff)

	// TODO(davidr) - document this
	if o > 1024 {
		o = -(2048 - o)
	}

	// do the 1.024 division, round back to the nearest int and move on
	of := float64(o) / 1.024
	*voltageOffset = int(math.Round(of))

	return nil

}

func packOffset(offset uint32, plane int, write bool) uint64 {
	// I've deconstructed this calculation a little bit so I can remember how this magic
	// int64 was created. This is... not intuitive to me.
	planeBits := uint64(plane) << 40

	// I don't actually know what this bit does, but it seems to be needed
	unknownBit := uint64(1 << 36)

	// Set to 1 if we intend to write the value
	writeBit := uint64(0 << 32)
	if write {
		writeBit = uint64(1 << 32)
	}

	return (1 << 63) | planeBits | writeBit | unknownBit | uint64(offset)
}

func calcUndervoltValue(plane int, offsetMv int) uint64 {
	offset := uint32(math.Round(float64(offsetMv) * 1.024))

	// The actual value is only an 11 bit number, so we left-shift by 21
	offsetValue := 0xffe00000 & ((offset & 0xfff) << 21)
	return packOffset(offsetValue, plane, true)
}
