package modes

import "math"

// ADS-B character set for callsign decoding (TC 1-4).
const charset = "#ABCDEFGHIJKLMNOPQRSTUVWXYZ##### ###############0123456789######"

// Decode decodes a RawMessage into a DecodedMessage.
func Decode(raw *RawMessage) *DecodedMessage {
	msg := &DecodedMessage{RawMessage: *raw}
	if len(raw.Raw) < 7 {
		return msg
	}

	msg.DF = int(raw.Raw[0] >> 3)
	msg.ICAO, msg.CRCValid = ExtractICAO(raw.Raw)

	// Only decode DF17 extended squitter in detail
	if msg.DF == 17 && len(raw.Raw) == 14 && msg.CRCValid {
		decodeDF17(msg)
	}

	// DF4/DF20: extract altitude
	if (msg.DF == 4 || msg.DF == 20) && msg.CRCValid {
		msg.AltitudeFt = decodeAltitude13(raw.Raw)
	}

	return msg
}

func decodeDF17(msg *DecodedMessage) {
	me := msg.Raw[4:11] // Message Extension (56 bits = 7 bytes)
	msg.TypeCode = int(me[0] >> 3)

	switch {
	case msg.TypeCode >= 1 && msg.TypeCode <= 4:
		decodeIdentification(msg, me)
	case msg.TypeCode >= 9 && msg.TypeCode <= 18:
		decodeAirbornePosition(msg, me)
	case msg.TypeCode == 19:
		decodeAirborneVelocity(msg, me)
	}
}

// decodeIdentification decodes aircraft identification (callsign) from TC 1-4.
func decodeIdentification(msg *DecodedMessage, me []byte) {
	// 8 characters, each 6 bits, starting from bit 8 of ME
	// ME is 7 bytes = 56 bits. TC is bits 0-4. Cat is bits 5-7.
	// Characters start at bit 8.
	bits := uint64(0)
	for _, b := range me {
		bits = (bits << 8) | uint64(b)
	}

	var callsign [8]byte
	for i := 0; i < 8; i++ {
		shift := uint(42 - i*6) // 48-6=42, 42-6=36, ...
		idx := (bits >> shift) & 0x3F
		if int(idx) < len(charset) {
			callsign[i] = charset[idx]
		} else {
			callsign[i] = ' '
		}
	}

	// Trim trailing spaces and # characters
	s := string(callsign[:])
	end := len(s)
	for end > 0 && (s[end-1] == ' ' || s[end-1] == '#') {
		end--
	}
	msg.Callsign = s[:end]
}

// decodeAirbornePosition decodes airborne position from TC 9-18.
func decodeAirbornePosition(msg *DecodedMessage, me []byte) {
	// ME layout (56 bits):
	// [TC:5][SS:2][SAF:1][ALT:12][T:1][F:1][LAT-CPR:17][LON-CPR:17]

	// Altitude: bits 8-19 (12 bits)
	alt12 := (uint16(me[1]) << 4) | (uint16(me[2]) >> 4)
	msg.AltitudeFt = decodeAC12(alt12)

	// CPR odd/even flag: bit 21
	msg.CPROddFlag = (me[2] & 0x04) != 0

	// CPR Latitude: bits 22-38 (17 bits)
	msg.CPRLat = int((uint32(me[2])&0x03)<<15 | uint32(me[3])<<7 | uint32(me[4])>>1)

	// CPR Longitude: bits 39-55 (17 bits)
	msg.CPRLon = int((uint32(me[4])&0x01)<<16 | uint32(me[5])<<8 | uint32(me[6]))
}

// decodeAirborneVelocity decodes airborne velocity from TC 19.
func decodeAirborneVelocity(msg *DecodedMessage, me []byte) {
	// Subtype: bits 5-7
	subtype := int(me[0]) & 0x07

	if subtype == 1 || subtype == 2 {
		// Ground speed (subtype 1 = subsonic, 2 = supersonic)
		// East-West velocity: bits 14-23
		ewDir := (me[1] >> 2) & 0x01 // 0=east, 1=west
		ewVel := int((uint16(me[1])&0x03)<<8|uint16(me[2])) - 1

		// North-South velocity: bits 25-34
		nsDir := (me[3] >> 7) & 0x01 // 0=north, 1=south
		nsVel := int((uint16(me[3])&0x7F)<<3|uint16(me[4])>>5) - 1

		if subtype == 2 {
			ewVel *= 4
			nsVel *= 4
		}

		vEW := float64(ewVel)
		if ewDir == 1 {
			vEW = -vEW
		}
		vNS := float64(nsVel)
		if nsDir == 1 {
			vNS = -vNS
		}

		msg.VelocityKts = math.Sqrt(vEW*vEW + vNS*vNS)
		msg.HeadingDeg = math.Mod(math.Atan2(vEW, vNS)*180/math.Pi+360, 360)

		// Vertical rate: bits 36-45
		vrSign := (me[4] >> 4) & 0x01
		vrVal := int((uint16(me[4])&0x07)<<6|uint16(me[5])>>2) - 1
		msg.VertRateFpm = vrVal * 64
		if vrSign == 1 {
			msg.VertRateFpm = -msg.VertRateFpm
		}
	}
}

// decodeAC12 decodes a 12-bit altitude code (Gillham code with M-bit).
func decodeAC12(ac12 uint16) int {
	// Check M-bit (bit 6, 0-indexed from MSB in the 12-bit field)
	mBit := (ac12 >> 6) & 1
	if mBit == 1 {
		// Metric altitude (25ft increments)
		// Remove M-bit and Q-bit
		n := int((ac12>>7)<<4 | (ac12 & 0x3F))
		return n * 25
	}

	// Q-bit is bit 4 (0-indexed from LSB in the 12-bit field)
	qBit := (ac12 >> 4) & 1
	if qBit == 1 {
		// 25ft resolution
		// Remove Q-bit: upper 7 bits and lower 4 bits
		n := int((ac12>>5)<<4 | (ac12 & 0x0F))
		return n*25 - 1000
	}

	// Gillham code (100ft resolution) - complex, return 0 for now
	return 0
}

// decodeAltitude13 decodes the 13-bit altitude field from DF4/DF20 messages.
func decodeAltitude13(raw []byte) int {
	// Altitude code is in bits 20-32 (13 bits)
	ac13 := (uint16(raw[2])<<8 | uint16(raw[3])) & 0x1FFF
	// M-bit is bit 6, Q-bit is bit 4
	qBit := (ac13 >> 4) & 1
	if qBit == 1 {
		n := int((ac13>>5)<<4 | (ac13 & 0x0F))
		return n*25 - 1000
	}
	return 0
}
