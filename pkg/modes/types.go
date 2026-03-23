package modes

// RawMessage is the parsed binary packet from a sensor.
type RawMessage struct {
	SensorID    int64
	SensorLat   float64
	SensorLon   float64
	SensorAlt   float64
	SecSinceMid uint64
	Nanoseconds uint64
	Raw         []byte // 7 or 14 bytes ModeS frame
}

// TimestampNs returns the absolute timestamp in nanoseconds since midnight.
func (r *RawMessage) TimestampNs() int64 {
	return int64(r.SecSinceMid)*1_000_000_000 + int64(r.Nanoseconds)
}

// DecodedMessage extends RawMessage with decoded ModeS fields.
type DecodedMessage struct {
	RawMessage
	DF       int    // Downlink Format (0-24)
	ICAO     uint32 // 24-bit aircraft address
	TypeCode int    // For DF17: type code (0-31)
	// DF17 TC 1-4: identification
	Callsign string
	// DF17 TC 9-18: airborne position
	AltitudeFt int
	CPRLat     int
	CPRLon     int
	CPROddFlag bool
	// DF17 TC 19: airborne velocity
	VelocityKts float64
	HeadingDeg  float64
	VertRateFpm int
	// Validity
	CRCValid bool
}

// DecodedPosition is a resolved aircraft position from CPR decoding.
type DecodedPosition struct {
	ICAO        uint32
	Lat         float64
	Lon         float64
	AltFt       int
	TimestampNs int64
}
