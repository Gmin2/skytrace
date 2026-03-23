package modes

// CRC-24 for Mode S messages.
// Generator polynomial: x^24 + x^23 + x^10 + x^9 + x^8 + x^6 + x^5 + x^4 + x^2 + x + 1
// = 0x1FFF409 (25-bit), or 0xFFF409 (24-bit remainder).

const crcGenerator = 0xFFF409

// crc24Table is a precomputed CRC-24 lookup table for byte-at-a-time processing.
var crc24Table [256]uint32

func init() {
	for i := 0; i < 256; i++ {
		crc := uint32(i) << 16
		for bit := 0; bit < 8; bit++ {
			if crc&0x800000 != 0 {
				crc = (crc << 1) ^ crcGenerator
			} else {
				crc = crc << 1
			}
		}
		crc24Table[i] = crc & 0xFFFFFF
	}
}

// CRC24 computes the CRC-24 of the given data bytes.
func CRC24(data []byte, nBytes int) uint32 {
	var crc uint32
	for i := 0; i < nBytes; i++ {
		crc = crc24Table[((crc>>16)^uint32(data[i]))&0xFF] ^ (crc << 8)
		crc &= 0xFFFFFF
	}
	return crc
}

// ExtractICAO extracts the ICAO address and validates CRC for the given raw ModeS frame.
// For DF11 (56-bit): ICAO is bytes 1-3. CRC is computed over first 4 bytes,
//
//	remainder XORed with last 3 bytes should equal ICAO.
//
// For DF17/DF18 (112-bit): ICAO is bytes 1-3. CRC of first 11 bytes should match last 3 bytes.
// For DF4/5/20/21: ICAO is recovered from parity (CRC XOR last 3 bytes).
func ExtractICAO(raw []byte) (icao uint32, crcValid bool) {
	if len(raw) < 7 {
		return 0, false
	}

	df := int(raw[0] >> 3)

	switch {
	case df == 11:
		// DF11: ICAO address is directly in bytes 1-3.
		// The AP (Address/Parity) field in bytes 4-6 encodes CRC XOR address info.
		// For squitters from ADS-B sensors, we trust the ICAO from bytes 1-3.
		// CRC validation: compute CRC over first 4 bytes, XOR with last 3 bytes,
		// result should match ICAO (or be close for valid messages).
		icao = uint32(raw[1])<<16 | uint32(raw[2])<<8 | uint32(raw[3])
		crc := CRC24(raw[:4], 4)
		pi := uint32(raw[4])<<16 | uint32(raw[5])<<8 | uint32(raw[6])
		recovered := crc ^ pi
		// Valid if recovered address matches ICAO, or if full-message CRC is 0
		crcValid = recovered == icao || CRC24(raw, 7) == 0
		return icao, crcValid

	case df == 17 || df == 18:
		// DF17/18: ICAO is directly in bytes 1-3
		icao = uint32(raw[1])<<16 | uint32(raw[2])<<8 | uint32(raw[3])
		// CRC over all 14 bytes should be 0
		remainder := CRC24(raw, 14)
		crcValid = remainder == 0
		return icao, crcValid

	case df == 4 || df == 5 || df == 20 || df == 21:
		// Short (DF4/5) or long (DF20/21): ICAO is hidden in parity
		// CRC of message XOR last 3 bytes = ICAO address
		// We can't validate without knowing the ICAO, so we extract it
		n := 7
		if df == 20 || df == 21 {
			n = 14
		}
		if len(raw) < n {
			return 0, false
		}
		remainder := CRC24(raw, n)
		// The remainder IS the ICAO (when parity bits contain ICAO XOR CRC)
		icao = remainder
		// We can't definitively validate CRC for these formats without a known ICAO list
		// But a non-zero remainder that looks like a valid ICAO is our best bet
		crcValid = icao != 0
		return icao, crcValid

	default:
		return 0, false
	}
}
