package mlat

import "math"

// WGS84 ellipsoid constants.
const (
	WGS84A  = 6378137.0            // semi-major axis (meters)
	WGS84B  = 6356752.314245       // semi-minor axis
	WGS84E2 = 0.00669437999014     // first eccentricity squared
	WGS84F  = 1.0 / 298.257223563 // flattening
)

// GeodeticToECEF converts WGS84 lat/lon/alt to Earth-Centered Earth-Fixed coordinates.
// lat and lon are in degrees, alt in meters.
// Returns x, y, z in meters.
func GeodeticToECEF(latDeg, lonDeg, altM float64) (x, y, z float64) {
	lat := latDeg * math.Pi / 180
	lon := lonDeg * math.Pi / 180

	sinLat := math.Sin(lat)
	cosLat := math.Cos(lat)
	sinLon := math.Sin(lon)
	cosLon := math.Cos(lon)

	// Radius of curvature in the prime vertical
	N := WGS84A / math.Sqrt(1-WGS84E2*sinLat*sinLat)

	x = (N + altM) * cosLat * cosLon
	y = (N + altM) * cosLat * sinLon
	z = (N*(1-WGS84E2) + altM) * sinLat
	return
}

// ECEFToGeodetic converts ECEF coordinates back to WGS84 lat/lon/alt.
// Uses iterative Bowring method (converges in 2-3 iterations).
// Returns lat and lon in degrees, alt in meters.
func ECEFToGeodetic(x, y, z float64) (latDeg, lonDeg, altM float64) {
	lon := math.Atan2(y, x)

	p := math.Sqrt(x*x + y*y)

	// Initial estimate using Bowring's method
	theta := math.Atan2(z*WGS84A, p*WGS84B)

	lat := math.Atan2(
		z+WGS84E2/(1-WGS84E2)*WGS84A*WGS84A/WGS84B*math.Pow(math.Sin(theta), 3),
		p-WGS84E2*WGS84A*math.Pow(math.Cos(theta), 3),
	)

	// Iterate for better accuracy
	for i := 0; i < 5; i++ {
		sinLat := math.Sin(lat)
		N := WGS84A / math.Sqrt(1-WGS84E2*sinLat*sinLat)
		lat = math.Atan2(z+WGS84E2*N*sinLat, p)
	}

	sinLat := math.Sin(lat)
	N := WGS84A / math.Sqrt(1-WGS84E2*sinLat*sinLat)
	altM = p/math.Cos(lat) - N

	latDeg = lat * 180 / math.Pi
	lonDeg = lon * 180 / math.Pi
	return
}
