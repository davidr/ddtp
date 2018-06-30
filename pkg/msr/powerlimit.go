package msr

import "math"

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
