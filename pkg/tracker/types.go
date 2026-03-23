package tracker

// Track represents a tracked aircraft.
type Track struct {
	ICAO       uint32
	Callsign   string
	Lat        float64
	Lon        float64
	AltFt      int
	HeadingDeg float64
	SpeedKts   float64
	VertRateFpm int

	// ECEF state for Kalman filter
	X, Y, Z    float64 // position (meters)
	VX, VY, VZ float64 // velocity (m/s)

	// Kalman covariance (6x6, flattened upper triangle for simplicity)
	P [6][6]float64

	LastUpdateNs int64 // nanoseconds since midnight
	MlatCount    int
	AdsbCount    int
	Coasted      bool
	History      []HistoryPoint
}

// HistoryPoint is a position snapshot for trail rendering.
type HistoryPoint struct {
	Lat         float64
	Lon         float64
	AltFt       int
	TimestampNs int64
}
