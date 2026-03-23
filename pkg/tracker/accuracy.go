package tracker

import (
	"math"
	"sync"
)

// AccuracyStats tracks MLAT accuracy by comparing against ADS-B ground truth.
type AccuracyStats struct {
	mu     sync.Mutex
	errors []float64 // distance errors in meters (MLAT vs ADS-B)
}

func NewAccuracyStats() *AccuracyStats {
	return &AccuracyStats{}
}

// Record records an error measurement (distance in meters between MLAT and ADS-B position).
func (a *AccuracyStats) Record(errorMeters float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.errors = append(a.errors, errorMeters)
}

// Summary returns accuracy statistics.
func (a *AccuracyStats) Summary() AccuracySummary {
	a.mu.Lock()
	defer a.mu.Unlock()

	n := len(a.errors)
	if n == 0 {
		return AccuracySummary{}
	}

	// Sort a copy for percentiles
	sorted := make([]float64, n)
	copy(sorted, a.errors)
	sortFloat64s(sorted)

	var sum float64
	for _, e := range sorted {
		sum += e
	}

	return AccuracySummary{
		Count:     n,
		MeanM:     sum / float64(n),
		MedianM:   sorted[n/2],
		P90M:      sorted[int(float64(n)*0.9)],
		P95M:      sorted[int(float64(n)*0.95)],
		MinM:      sorted[0],
		MaxM:      sorted[n-1],
		Under100:  countBelow(sorted, 100),
		Under500:  countBelow(sorted, 500),
		Under1000: countBelow(sorted, 1000),
		Under5000: countBelow(sorted, 5000),
	}
}

type AccuracySummary struct {
	Count     int     `json:"count"`
	MeanM     float64 `json:"mean_m"`
	MedianM   float64 `json:"median_m"`
	P90M      float64 `json:"p90_m"`
	P95M      float64 `json:"p95_m"`
	MinM      float64 `json:"min_m"`
	MaxM      float64 `json:"max_m"`
	Under100  int     `json:"under_100m"`
	Under500  int     `json:"under_500m"`
	Under1000 int     `json:"under_1km"`
	Under5000 int     `json:"under_5km"`
}

func countBelow(sorted []float64, threshold float64) int {
	count := 0
	for _, v := range sorted {
		if v <= threshold {
			count++
		} else {
			break
		}
	}
	return count
}

// HaversineM computes the great-circle distance between two lat/lon points in meters.
func HaversineM(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000.0 // Earth radius in meters
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

// Simple insertion sort for float64 slice (good enough for our data sizes)
func sortFloat64s(s []float64) {
	for i := 1; i < len(s); i++ {
		key := s[i]
		j := i - 1
		for j >= 0 && s[j] > key {
			s[j+1] = s[j]
			j--
		}
		s[j+1] = key
	}
}
