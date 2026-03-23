package modes

import (
	"encoding/hex"
	"fmt"
	"testing"
)

func TestCRC24Known(t *testing.T) {
	// Test with known DF17 and DF11 frames from our log
	tests := []struct {
		name   string
		hexStr string
		df     int
		icao   uint32
	}{
		{"DF17", "8d406440ea447864013c08792c43", 17, 0x406440},
		{"DF11_a", "5d407e4e1b9d73", 11, 0x407E4E},
		{"DF11_b", "5d48c22218e3e8", 11, 0x48C222},
		{"DF11_c", "5da8cecd8fdd5b", 11, 0xA8CECD},
	}

	for _, tt := range tests {
		raw, _ := hex.DecodeString(tt.hexStr)
		remainder := CRC24(raw, len(raw))
		df := int(raw[0] >> 3)
		icao := uint32(raw[1])<<16 | uint32(raw[2])<<8 | uint32(raw[3])

		fmt.Printf("%s: hex=%s df=%d icao=%06X remainder=%06X (decimal=%d)\n",
			tt.name, tt.hexStr, df, icao, remainder, remainder)

		if df == 17 {
			if remainder != 0 {
				t.Errorf("%s: DF17 CRC should be 0, got %06X", tt.name, remainder)
			}
		}
	}
}

func TestCRC24BitByBit(t *testing.T) {
	// Verify CRC24 with a bit-by-bit implementation
	raw, _ := hex.DecodeString("8d406440ea447864013c08792c43")

	// Bit-by-bit CRC
	var crc uint32
	for i := 0; i < len(raw)*8; i++ {
		byteIdx := i / 8
		bitIdx := 7 - (i % 8)
		bit := (uint32(raw[byteIdx]) >> uint(bitIdx)) & 1

		if crc&0x800000 != 0 {
			crc = ((crc << 1) | bit) ^ 0xFFF409
		} else {
			crc = (crc << 1) | bit
		}
		crc &= 0xFFFFFF
	}
	fmt.Printf("Bit-by-bit CRC of DF17: %06X\n", crc)
}
