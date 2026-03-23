package correlator

// Reception is one sensor's observation of a ModeS transmission.
type Reception struct {
	SensorID   int64
	SensorLat  float64
	SensorLon  float64
	SensorAlt  float64
	TimestampNs int64 // nanoseconds since midnight
}

// Group is a set of receptions of the same physical transmission
// from the same aircraft, seen by multiple sensors.
type Group struct {
	ICAO       uint32
	RawHex     string      // hex of the raw ModeS bytes (the correlation key)
	Receptions []Reception // sorted by timestamp
	CreatedNs  int64       // timestamp of first reception
}

// TDOA returns the time differences of arrival relative to the first reception.
// Returns N-1 values in seconds.
func (g *Group) TDOA() []float64 {
	if len(g.Receptions) < 2 {
		return nil
	}
	ref := g.Receptions[0].TimestampNs
	tdoa := make([]float64, len(g.Receptions)-1)
	for i := 1; i < len(g.Receptions); i++ {
		tdoa[i-1] = float64(g.Receptions[i].TimestampNs-ref) / 1e9
	}
	return tdoa
}
