package msr

import "fmt"

// TemperatureTarget is a struct corresponding to the TEMPERATURE_TARGET MSR for
// an individual CPU
type TemperatureTarget struct {
	cpu    int   // CPU number
	target int64 // default thermal throttling activation temperature in degrees C
	offset int64 // offset from the default in degrees C at which to start throttling
}

// GetThrottleTemp returns the throttle temperature calculated from the TemperatureTarget
// default and offset data
func (t *TemperatureTarget) GetThrottleTemp() int64 {
	return t.target - t.offset
}

// SetThrottleTemp sets the throttle temperature for the CPU to temp by way of an offset
// from TemperatureTarget.target (e.g. if t.target == 100, then setThrottleTemp(90)
// will set t.offset to 10)
func (t *TemperatureTarget) SetThrottleTemp(throttleTemp int64) error {
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

// GetTempTarget returns a TemperatureTarget struct for the cpu given in cpu
func GetTempTarget(cpu int) (TemperatureTarget, error) {
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
