package msr

import (
	"testing"
)

func TestVoltageUnpacking(t *testing.T) {
	t.Log("Testing voltage offset values")
	m := map[uint64]int{
		0x40000000000: 0,
		0x100f3400000: -100,
	}

	var plane, offset int
	for k, v := range m {
		unpackOffset(&plane, &offset, k)
		if v != offset {
			t.Errorf("offset 0x%016x unpacks to %d, should be %d", k, offset, v)
		}
	}
}

func TestVoltageUnpackAcrossRange(t *testing.T) {
	// Iterate through the domain of acceptable values for offset ([-999,999]) and
	// make sure they convert and then unconvert to the same value

	var plane, offset int

	for i := -999; i <= 999; i++ {
		uVoltValue := calcUndervoltValue(0, i)
		unpackOffset(&plane, &offset, uVoltValue)

		if i != offset {
			t.Errorf("voltage offset of %d packs and unpacks to %d", i, offset)
		}

	}
}
