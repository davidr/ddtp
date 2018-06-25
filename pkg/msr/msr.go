package msr

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"os"

	"github.com/davidr/ddtp/pkg/util"
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
func getRAPLPowerUnits(rplUnitBitfield uint64) (float64, float64) {
	// For power-related info, the units are (2^p)^-1 mW where p is the uint from 3:0 in
	// the powerLimitUnits MSR
	pwrExponent := float64(rplUnitBitfield & 0x0f)
	powerUnits := 1 / math.Pow(2, pwrExponent)

	// Same for time in unit seconds, bits 19:16
	timeExponent := float64((rplUnitBitfield >> 16) & 0x0f)
	timeUnits := 1 / math.Pow(2, timeExponent)

	return powerUnits, timeUnits
}

// GetAllMsrFiles returns an array containing the /dev/cpu/XX/msr files for all CPUs on the
// system.
func GetAllMsrFiles() ([]string, error) {
	var MSRFiles []string

	cpus, err := util.GetAllCPUs()
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
	// TODO(davidr) assert offset_mv < 0

	offset := uint32(math.Round(float64(offsetMv) * 1.024))

	// The actual value is only an 11 bit number, so we left-shift by 21
	offsetValue := 0xffe00000 & ((offset & 0xfff) << 21)
	return packOffset(offsetValue, plane, true)
}

func readMSRIntValue(msrFile string, MSRRegAddr int64) (uint64, error) {
	var ReturnValue uint64
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
	file, err := os.OpenFile(msrFile, os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	// Seek MSRRegAddr bytes from beginning of file
	_, err = file.Seek(MSRRegAddr, 0)
	if err != nil {
		// Something's gone seriously wrong. We've opened the MSR file WRONLY, but somehow can't
		// seek in it?
		file.Close()
		return err
	}

	buff := new(bytes.Buffer)
	err = binary.Write(buff, binary.LittleEndian, value)
	_, err = file.Write(buff.Bytes())
	if err != nil {
		file.Close()
		return err
	}

	file.Close()
	return nil
}

// GetMsrFile returns a string with the path the model specific register (msr) files associated
// with the given CPU. (i.e. "/dev/cpu/0/msr")
func GetMsrFile(cpu int) (string, error) {

	if !util.IsValidCPU(cpu) {
		return "", fmt.Errorf("msr: invalid CPU number %d", cpu)
	}

	msrFile := fmt.Sprintf("/dev/cpu/%d/msr", cpu)
	// TODO(davidr) incorrect checking for whether stat succeeds
	if _, err := os.Stat(msrFile); os.IsNotExist(err) {
		return msrFile, err
	}

	return msrFile, nil
}
