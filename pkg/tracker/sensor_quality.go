package tracker

import (
	"math"
	"sync"
)

// SensorQuality tracks per-sensor quality metrics.
type SensorQuality struct {
	mu      sync.Mutex
	sensors map[int64]*SensorStats
}

type SensorStats struct {
	MsgCount       int     `json:"msg_count"`
	MlatContrib    int     `json:"mlat_contributions"` // how many MLAT solves this sensor participated in
	UniqueAircraft map[uint32]bool `json:"-"`
	AircraftCount  int     `json:"aircraft_count"`

	// Clock drift estimation
	// For each sensor pair that saw the same message, we record the TDOA.
	// If we know the aircraft position (from ADS-B), the expected TDOA can be computed.
	// The difference = clock offset estimate.
	ClockOffsets []float64 `json:"-"` // nanoseconds
	MeanClockNs  float64   `json:"mean_clock_offset_ns"`
	StdClockNs   float64   `json:"std_clock_offset_ns"`

	// Message rate
	FirstMsgNs int64 `json:"-"`
	LastMsgNs  int64 `json:"-"`
	MsgRateHz  float64 `json:"msg_rate_hz"`
}

func NewSensorQuality() *SensorQuality {
	return &SensorQuality{
		sensors: make(map[int64]*SensorStats),
	}
}

// RecordMessage records a message from a sensor.
func (sq *SensorQuality) RecordMessage(sensorID int64, icao uint32, tsNs int64) {
	sq.mu.Lock()
	defer sq.mu.Unlock()

	s, ok := sq.sensors[sensorID]
	if !ok {
		s = &SensorStats{
			UniqueAircraft: make(map[uint32]bool),
			FirstMsgNs:     tsNs,
		}
		sq.sensors[sensorID] = s
	}

	s.MsgCount++
	s.UniqueAircraft[icao] = true
	s.AircraftCount = len(s.UniqueAircraft)
	s.LastMsgNs = tsNs

	// Compute message rate
	durationSec := float64(s.LastMsgNs-s.FirstMsgNs) / 1e9
	if durationSec > 0 {
		s.MsgRateHz = float64(s.MsgCount) / durationSec
	}
}

// RecordMLATContribution records that a sensor participated in an MLAT solve.
func (sq *SensorQuality) RecordMLATContribution(sensorID int64) {
	sq.mu.Lock()
	defer sq.mu.Unlock()

	if s, ok := sq.sensors[sensorID]; ok {
		s.MlatContrib++
	}
}

// RecordClockOffset records an estimated clock offset for a sensor (in nanoseconds).
func (sq *SensorQuality) RecordClockOffset(sensorID int64, offsetNs float64) {
	sq.mu.Lock()
	defer sq.mu.Unlock()

	s, ok := sq.sensors[sensorID]
	if !ok {
		return
	}

	s.ClockOffsets = append(s.ClockOffsets, offsetNs)
	// Keep only last 100 measurements
	if len(s.ClockOffsets) > 100 {
		s.ClockOffsets = s.ClockOffsets[len(s.ClockOffsets)-100:]
	}

	// Update mean and std
	n := len(s.ClockOffsets)
	var sum float64
	for _, v := range s.ClockOffsets {
		sum += v
	}
	mean := sum / float64(n)
	s.MeanClockNs = mean

	var sumSq float64
	for _, v := range s.ClockOffsets {
		d := v - mean
		sumSq += d * d
	}
	s.StdClockNs = math.Sqrt(sumSq / float64(n))
}

// GetAll returns stats for all sensors.
func (sq *SensorQuality) GetAll() map[int64]*SensorStats {
	sq.mu.Lock()
	defer sq.mu.Unlock()

	result := make(map[int64]*SensorStats)
	for id, s := range sq.sensors {
		cp := *s
		cp.UniqueAircraft = nil // don't copy the map
		cp.ClockOffsets = nil
		result[id] = &cp
	}
	return result
}
