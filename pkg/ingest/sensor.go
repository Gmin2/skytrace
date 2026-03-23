package ingest

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
)

// SensorInfo holds the true position of a sensor from location-override.json.
type SensorInfo struct {
	Name string
	Lat  float64
	Lon  float64
	Alt  float64 // meters
}

// SensorRegistry maps sensor IDs to their true locations.
type SensorRegistry struct {
	byID   map[int64]*SensorInfo
	byName map[string]*SensorInfo
}

// locationOverrideEntry matches the JSON structure in location-override.json.
type locationOverrideEntry struct {
	PublicKey string  `json:"public_key"`
	Name      string  `json:"name"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	Alt       float64 `json:"alt"`
}

// NewSensorRegistry loads sensor locations from a JSON file and builds a registry.
// Since we match sensors by proximity (the stream reports approximate positions),
// call Register() with each sensor ID and its stream-reported lat/lon to match.
func NewSensorRegistry(overridePath string) (*SensorRegistry, error) {
	data, err := os.ReadFile(overridePath)
	if err != nil {
		return nil, fmt.Errorf("read override file: %w", err)
	}

	var entries []locationOverrideEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse override file: %w", err)
	}

	reg := &SensorRegistry{
		byID:   make(map[int64]*SensorInfo),
		byName: make(map[string]*SensorInfo),
	}

	for _, e := range entries {
		info := &SensorInfo{
			Name: e.Name,
			Lat:  e.Lat,
			Lon:  e.Lon,
			Alt:  e.Alt,
		}
		reg.byName[e.Name] = info
	}

	return reg, nil
}

// Register matches a sensor ID (from the stream) to the closest override entry
// based on the stream-reported lat/lon. This handles the fact that the stream
// reports Alt=0.00 and approximate positions.
func (r *SensorRegistry) Register(sensorID int64, streamLat, streamLon float64) *SensorInfo {
	if info, ok := r.byID[sensorID]; ok {
		return info
	}

	// Find closest override entry by proximity
	var bestName string
	bestDist := math.MaxFloat64

	for name, info := range r.byName {
		dist := haversineKm(streamLat, streamLon, info.Lat, info.Lon)
		if dist < bestDist {
			bestDist = dist
			bestName = name
		}
	}

	if bestName != "" && bestDist < 5.0 { // within 5km
		info := r.byName[bestName]
		r.byID[sensorID] = info
		return info
	}

	// No match — use stream position as fallback
	info := &SensorInfo{
		Name: fmt.Sprintf("unknown-%d", sensorID),
		Lat:  streamLat,
		Lon:  streamLon,
		Alt:  0,
	}
	r.byID[sensorID] = info
	return info
}

// Get returns the sensor info for a given sensor ID.
func (r *SensorRegistry) Get(sensorID int64) *SensorInfo {
	return r.byID[sensorID]
}

// All returns all registered sensors.
func (r *SensorRegistry) All() map[int64]*SensorInfo {
	return r.byID
}

func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371.0 // Earth radius in km
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}
