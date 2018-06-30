package msr

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"

	"github.com/davidr/ddtp/pkg/util"
	log "github.com/sirupsen/logrus"
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


func readMSRIntValue(msrFile string, MSRRegAddr int64) (uint64, error) {
	log.Infof("reading value from %s:%d", msrFile, MSRRegAddr)
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

	log.Debugf("read 0x%016x", ReturnValue)
	return ReturnValue, err
}

// WriteMSRIntValue packs a uint64 into a byte array and writes said array to the MSR file
// msr_file (i.e. for one spcific CPU) at location MSRRegAddr
func WriteMSRIntValue(msrFile string, MSRRegAddr int64, value uint64) error {
	log.Debugf("writing 0x%016x to %s:%x", value, msrFile, MSRRegAddr)
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

	log.Debugf("returning msr file %s for CPU %d", msrFile, cpu)
	return msrFile, nil
}
