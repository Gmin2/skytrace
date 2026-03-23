package modes

import "math"

// CPR (Compact Position Reporting) decoder for ADS-B airborne positions.
//
// Global decoding requires one odd + one even frame from the same aircraft.
// The even frame determines which latitude zone, the formulas then resolve
// an unambiguous position.

const (
	cprMaxAirborne = 131072.0 // 2^17
	nzAirborne     = 15       // Number of latitude zones for airborne
)

// nl returns the Number of Longitude zones for a given latitude.
func nl(lat float64) int {
	if math.Abs(lat) >= 87.0 {
		return 1
	}
	nz := float64(2 * nzAirborne)
	a := 1.0 - math.Cos(math.Pi/(2.0*nz))
	b := math.Cos(math.Pi/180.0*lat) * math.Cos(math.Pi/180.0*lat)
	result := math.Floor(2.0 * math.Pi / math.Acos(1.0-a/b))
	if result < 1 {
		return 1
	}
	return int(result)
}

// CPRGlobalDecode decodes a pair of CPR frames (one even, one odd) into lat/lon.
// evenLat/evenLon are the CPR values from the even frame (F=0).
// oddLat/oddLon are the CPR values from the odd frame (F=1).
// lastIsOdd indicates which frame arrived most recently (used to pick the correct zone).
// Returns latitude, longitude in degrees, and ok=true if valid.
func CPRGlobalDecode(evenLat, evenLon, oddLat, oddLon int, lastIsOdd bool) (lat, lon float64, ok bool) {
	// Normalize to [0, 1)
	rlat0 := float64(evenLat) / cprMaxAirborne
	rlat1 := float64(oddLat) / cprMaxAirborne
	rlon0 := float64(evenLon) / cprMaxAirborne
	rlon1 := float64(oddLon) / cprMaxAirborne

	// Latitude zone sizes
	dLat0 := 360.0 / (4.0 * float64(nzAirborne))     // even: 6.0 degrees
	dLat1 := 360.0 / (4.0*float64(nzAirborne) - 1.0)  // odd: 6.101...degrees

	// Latitude zone index
	j := math.Floor(59.0*rlat0 - 60.0*rlat1 + 0.5)

	// Compute candidate latitudes
	lat0 := dLat0 * (math.Mod(j, 60.0) + rlat0)
	lat1 := dLat1 * (math.Mod(j, 59.0) + rlat1)

	// Normalize to [-90, 90]
	if lat0 >= 270.0 {
		lat0 -= 360.0
	}
	if lat1 >= 270.0 {
		lat1 -= 360.0
	}

	// Check that both latitudes are in the same NL zone
	if nl(lat0) != nl(lat1) {
		return 0, 0, false
	}

	// Pick latitude based on most recent frame
	if lastIsOdd {
		lat = lat1
	} else {
		lat = lat0
	}

	// Longitude
	var nli int
	var rlon float64
	if lastIsOdd {
		nli = nl(lat1)
		ni := maxInt(nli-1, 1)
		m := math.Floor(float64(evenLon)*(float64(nli)-1.0)/cprMaxAirborne - float64(oddLon)*float64(nli)/cprMaxAirborne + 0.5)
		lon = (360.0 / float64(ni)) * (math.Mod(m, float64(ni)) + rlon1)
		_ = rlon
	} else {
		nli = nl(lat0)
		ni := maxInt(nli, 1)
		m := math.Floor(float64(evenLon)*(float64(nli)-1.0)/cprMaxAirborne - float64(oddLon)*float64(nli)/cprMaxAirborne + 0.5)
		lon = (360.0 / float64(ni)) * (math.Mod(m, float64(ni)) + rlon0)
		_ = rlon
	}

	// Normalize longitude to [-180, 180]
	if lon >= 180.0 {
		lon -= 360.0
	}

	// Sanity check
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return 0, 0, false
	}

	return lat, lon, true
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
