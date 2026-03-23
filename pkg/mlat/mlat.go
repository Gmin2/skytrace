package mlat

import (
	"fmt"
	"math"

	"quickstart/pkg/correlator"
)

// Result is the output of an MLAT solve.
type Result struct {
	ICAO       uint32
	Lat        float64
	Lon        float64
	AltM       float64
	AltFt      int
	TimestampNs int64
	NumSensors int
	Residual   float64 // RMS residual in meters
}

// Solve takes a correlation group and computes the aircraft position.
func Solve(group *correlator.Group) (*Result, error) {
	recs := group.Receptions
	if len(recs) < 3 {
		return nil, fmt.Errorf("need at least 3 sensors, got %d", len(recs))
	}

	// Convert sensor positions to ECEF
	sensors := make([][3]float64, len(recs))
	for i, r := range recs {
		x, y, z := GeodeticToECEF(r.SensorLat, r.SensorLon, r.SensorAlt)
		sensors[i] = [3]float64{x, y, z}
	}

	// Compute TDOA in seconds relative to first reception
	tdoaSec := make([]float64, len(recs)-1)
	for i := 1; i < len(recs); i++ {
		tdoaSec[i-1] = float64(recs[i].TimestampNs-recs[0].TimestampNs) / 1e9
	}

	// Compute sensor centroid
	var avgLat, avgLon float64
	for _, r := range recs {
		avgLat += r.SensorLat
		avgLon += r.SensorLon
	}
	avgLat /= float64(len(recs))
	avgLon /= float64(len(recs))

	// Try multiple initial guesses to avoid local minima.
	// Aircraft can be far from the sensor network, so we probe
	// different directions and altitudes.
	guessPoints := [][3]float64{
		{avgLat, avgLon, 10000},
		{avgLat, avgLon, 5000},
		{avgLat + 0.5, avgLon, 10000},
		{avgLat - 0.5, avgLon, 10000},
		{avgLat, avgLon - 1.0, 10000},
		{avgLat, avgLon + 1.0, 10000},
		{avgLat, avgLon - 2.0, 10000},
		{avgLat - 0.5, avgLon - 1.0, 10000},
	}

	var bestPos [3]float64
	bestResidual := math.MaxFloat64
	solved := false

	for _, gp := range guessPoints {
		gx, gy, gz := GeodeticToECEF(gp[0], gp[1], gp[2])
		guess := [3]float64{gx, gy, gz}

		pos, residual, err := SolveTDOA(sensors, tdoaSec, guess)
		if err == nil && residual < bestResidual {
			bestPos = pos
			bestResidual = residual
			solved = true
		}
	}

	if !solved {
		return nil, fmt.Errorf("all initial guesses failed")
	}

	// Filter: high residual means the TDOA data doesn't fit well
	// (likely due to clock drift or bad correlation)
	if bestResidual > 5000 {
		return nil, fmt.Errorf("residual too high: %.0fm", bestResidual)
	}

	pos := bestPos
	residual := bestResidual

	// Convert back to geodetic
	lat, lon, alt := ECEFToGeodetic(pos[0], pos[1], pos[2])

	// Sanity checks
	if math.Abs(lat-avgLat) > 5 || math.Abs(lon-avgLon) > 5 {
		return nil, fmt.Errorf("result too far from sensors: lat=%.2f lon=%.2f", lat, lon)
	}
	if alt < -500 || alt > 25000 {
		return nil, fmt.Errorf("unreasonable altitude: %.0fm", alt)
	}

	return &Result{
		ICAO:        group.ICAO,
		Lat:         lat,
		Lon:         lon,
		AltM:        alt,
		AltFt:       int(alt * 3.28084),
		TimestampNs: recs[0].TimestampNs,
		NumSensors:  len(recs),
		Residual:    residual,
	}, nil
}
