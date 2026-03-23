package tracker

import (
	"math"
	"sync"

	mlatpkg "quickstart/pkg/mlat"
)

// Manager manages all active aircraft tracks.
type Manager struct {
	mu     sync.RWMutex
	tracks map[uint32]*Track // keyed by ICAO
}

func NewManager() *Manager {
	return &Manager{
		tracks: make(map[uint32]*Track),
	}
}

// ProcessMLATFix updates or creates a track from an MLAT position fix.
func (m *Manager) ProcessMLATFix(result *mlatpkg.Result) {
	m.mu.Lock()
	defer m.mu.Unlock()

	x, y, z := mlatpkg.GeodeticToECEF(result.Lat, result.Lon, result.AltM)

	t, exists := m.tracks[result.ICAO]
	if !exists {
		t = &Track{ICAO: result.ICAO}
		initKalman(t, x, y, z)
		m.tracks[result.ICAO] = t
	} else {
		// Predict to current time
		dt := float64(result.TimestampNs-t.LastUpdateNs) / 1e9
		if dt > 0 && dt < 300 {
			predictKalman(t, dt)
		}
		// Update with measurement
		updateKalman(t, x, y, z, sigmaMlatPos)
	}

	// Update geodetic position from Kalman state
	t.Lat, t.Lon, _ = mlatpkg.ECEFToGeodetic(t.X, t.Y, t.Z)
	t.AltFt = result.AltFt
	t.LastUpdateNs = result.TimestampNs
	t.MlatCount++
	t.Coasted = false

	// Add to history
	t.History = append(t.History, HistoryPoint{
		Lat:         t.Lat,
		Lon:         t.Lon,
		AltFt:       t.AltFt,
		TimestampNs: result.TimestampNs,
	})
	if len(t.History) > maxHistoryLen {
		t.History = t.History[len(t.History)-maxHistoryLen:]
	}
}

// ProcessADSBFix updates a track with a decoded ADS-B position.
func (m *Manager) ProcessADSBFix(icao uint32, lat, lon float64, altFt int, tsNs int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	altM := float64(altFt) / 3.28084
	x, y, z := mlatpkg.GeodeticToECEF(lat, lon, altM)

	t, exists := m.tracks[icao]
	if !exists {
		t = &Track{ICAO: icao}
		initKalman(t, x, y, z)
		m.tracks[icao] = t
	} else {
		dt := float64(tsNs-t.LastUpdateNs) / 1e9
		if dt > 0 && dt < 300 {
			predictKalman(t, dt)
		}
		updateKalman(t, x, y, z, sigmaAdsbPos)
	}

	t.Lat, t.Lon, _ = mlatpkg.ECEFToGeodetic(t.X, t.Y, t.Z)
	t.AltFt = altFt
	t.LastUpdateNs = tsNs
	t.AdsbCount++
	t.Coasted = false

	t.History = append(t.History, HistoryPoint{
		Lat:         t.Lat,
		Lon:         t.Lon,
		AltFt:       t.AltFt,
		TimestampNs: tsNs,
	})
	if len(t.History) > maxHistoryLen {
		t.History = t.History[len(t.History)-maxHistoryLen:]
	}
}

// SetCallsign sets the callsign for a track (from DF17 TC1-4).
func (m *Manager) SetCallsign(icao uint32, callsign string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.tracks[icao]; ok {
		t.Callsign = callsign
	}
}

// SetVelocity sets velocity info from ADS-B (DF17 TC19).
func (m *Manager) SetVelocity(icao uint32, speedKts, headingDeg float64, vertRateFpm int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.tracks[icao]; ok {
		t.SpeedKts = speedKts
		t.HeadingDeg = headingDeg
		t.VertRateFpm = vertRateFpm
	}
}

// CoastAndPrune marks stale tracks as coasted and removes very old ones.
// Returns number of pruned tracks.
func (m *Manager) CoastAndPrune(nowNs int64) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	pruned := 0
	for icao, t := range m.tracks {
		ageSec := float64(nowNs-t.LastUpdateNs) / 1e9
		if ageSec > 120 { // 2 minutes without update
			delete(m.tracks, icao)
			pruned++
		} else if ageSec > 10 {
			t.Coasted = true
		}
	}
	return pruned
}

// GetActiveTracks returns a snapshot of all active (non-coasted) tracks.
func (m *Manager) GetActiveTracks() []*Track {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Track
	for _, t := range m.tracks {
		if !t.Coasted {
			cp := *t
			cp.History = make([]HistoryPoint, len(t.History))
			copy(cp.History, t.History)
			result = append(result, &cp)
		}
	}
	return result
}

// GetAllTracks returns all tracks including coasted.
func (m *Manager) GetAllTracks() []*Track {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Track
	for _, t := range m.tracks {
		cp := *t
		cp.History = make([]HistoryPoint, len(t.History))
		copy(cp.History, t.History)
		result = append(result, &cp)
	}
	return result
}

// Stats returns tracker statistics.
func (m *Manager) Stats() (active, coasted, totalMlat, totalAdsb int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, t := range m.tracks {
		if t.Coasted {
			coasted++
		} else {
			active++
		}
		totalMlat += t.MlatCount
		totalAdsb += t.AdsbCount
	}
	return
}

// ComputeHeading computes approximate heading from ECEF velocity.
func ComputeHeading(vx, vy, vz float64) float64 {
	hdg := math.Atan2(vy, vx) * 180 / math.Pi
	return math.Mod(hdg+360, 360)
}
