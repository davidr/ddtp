package msr

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"

	"github.com/davidr/ddtp/pkg/util"
	log "github.com/sirupsen/logrus"
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
	log.Infof("reading value from %s:0x%x", msrFile, MSRRegAddr)
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
	log.Debugf("writing 0x%016x to %s:0x%x", value, msrFile, MSRRegAddr)
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

