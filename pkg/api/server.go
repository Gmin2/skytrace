package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"quickstart/pkg/ingest"
	trackerpkg "quickstart/pkg/tracker"
)

// Server serves the HTTP API, WebSocket, and static frontend.
// AccuracyProvider returns accuracy stats.
type AccuracyProvider func() trackerpkg.AccuracySummary

// SensorQualityProvider returns sensor quality stats.
type SensorQualityProvider func() map[int64]*trackerpkg.SensorStats

type Server struct {
	hub     *Hub
	tracker *trackerpkg.Manager
	sensors *ingest.SensorRegistry

	AccuracyFn      AccuracyProvider
	SensorQualityFn SensorQualityProvider

	// Live stats (updated atomically or via mutex)
	mu            sync.RWMutex
	sensorMsgCount map[int64]int
	sensorLastSeen map[int64]int64
	totalMessages  int64
	corrGroups     int64
	mlatSolved     int64
	mlatFailed     int64
}

// NewServer creates a new API server.
func NewServer(tm *trackerpkg.Manager, sensors *ingest.SensorRegistry) *Server {
	return &Server{
		hub:            NewHub(),
		tracker:        tm,
		sensors:        sensors,
		sensorMsgCount: make(map[int64]int),
		sensorLastSeen: make(map[int64]int64),
	}
}

// Start begins serving on the given address (e.g., ":8080").
func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()

	// WebSocket
	mux.HandleFunc("/ws", s.hub.HandleWS)

	// REST API
	mux.HandleFunc("/api/sensors", s.handleSensors)
	mux.HandleFunc("/api/tracks", s.handleTracks)
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/api/accuracy", s.handleAccuracy)
	mux.HandleFunc("/api/sensor-quality", s.handleSensorQuality)

	// Static frontend (serve from frontend/dist if it exists)
	// Use "/" as catch-all — ServeMux routes more specific patterns first
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Serve index.html for root or any non-API/non-ws path (SPA routing)
		path := r.URL.Path
		if path == "/" {
			http.ServeFile(w, r, "frontend/dist/index.html")
			return
		}
		// Try to serve static file
		http.FileServer(http.Dir("frontend/dist")).ServeHTTP(w, r)
	})

	log.Printf("API server starting on %s", addr)
	return http.ListenAndServe(addr, mux)
}

// StartBroadcastLoop periodically broadcasts track and sensor data to WebSocket clients.
func (s *Server) StartBroadcastLoop() {
	// Tracks every 3 seconds
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			s.broadcastTracks()
		}
	}()

	// Stats + sensors every 3 seconds
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			s.broadcastStats()
			s.broadcastSensors()
		}
	}()
}

// RecordMessage records a message from a sensor (for stats).
func (s *Server) RecordMessage(sensorID int64, tsNs int64) {
	atomic.AddInt64(&s.totalMessages, 1)
	s.mu.Lock()
	s.sensorMsgCount[sensorID]++
	s.sensorLastSeen[sensorID] = tsNs
	s.mu.Unlock()
}

// RecordCorrelation records a correlation group result.
func (s *Server) RecordCorrelation(solved bool) {
	atomic.AddInt64(&s.corrGroups, 1)
	if solved {
		atomic.AddInt64(&s.mlatSolved, 1)
	} else {
		atomic.AddInt64(&s.mlatFailed, 1)
	}
}

// BroadcastMLATFix sends an MLAT fix to all WebSocket clients.
func (s *Server) BroadcastMLATFix(fix MLATFixData) {
	s.hub.Broadcast(WSMessage{Type: "mlat_fix", Data: fix})
}

func (s *Server) broadcastTracks() {
	tracks := s.tracker.GetAllTracks()
	data := make([]TrackData, 0, len(tracks))
	for _, t := range tracks {
		td := TrackData{
			ICAO:        fmt.Sprintf("%06X", t.ICAO),
			Callsign:    t.Callsign,
			Lat:         t.Lat,
			Lon:         t.Lon,
			AltFt:       t.AltFt,
			SpeedKts:    t.SpeedKts,
			HeadingDeg:  t.HeadingDeg,
			VertRateFpm: t.VertRateFpm,
			MlatCount:   t.MlatCount,
			AdsbCount:   t.AdsbCount,
			Coasted:     t.Coasted,
		}
		for _, hp := range t.History {
			td.History = append(td.History, HistoryPoint{
				Lat:   hp.Lat,
				Lon:   hp.Lon,
				AltFt: hp.AltFt,
			})
		}
		data = append(data, td)
	}
	s.hub.Broadcast(WSMessage{Type: "tracks", Data: data})
}

func (s *Server) broadcastSensors() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var data []SensorData
	for id, info := range s.sensors.All() {
		data = append(data, SensorData{
			ID:       id,
			Name:     info.Name,
			Lat:      info.Lat,
			Lon:      info.Lon,
			AltM:     info.Alt,
			MsgCount: s.sensorMsgCount[id],
			LastSeen: s.sensorLastSeen[id],
		})
	}
	s.hub.Broadcast(WSMessage{Type: "sensors", Data: data})
}

func (s *Server) broadcastStats() {
	active, coasted, _, _ := s.tracker.Stats()
	s.hub.Broadcast(WSMessage{Type: "stats", Data: StatsData{
		TotalMessages: int(atomic.LoadInt64(&s.totalMessages)),
		CorrGroups:    int(atomic.LoadInt64(&s.corrGroups)),
		MlatSolved:    int(atomic.LoadInt64(&s.mlatSolved)),
		MlatFailed:    int(atomic.LoadInt64(&s.mlatFailed)),
		ActiveTracks:  active,
		CoastedTracks: coasted,
		SensorsOnline: len(s.sensors.All()),
	}})
}

// REST handlers

func (s *Server) handleSensors(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	s.mu.RLock()
	defer s.mu.RUnlock()

	var data []SensorData
	for id, info := range s.sensors.All() {
		data = append(data, SensorData{
			ID:       id,
			Name:     info.Name,
			Lat:      info.Lat,
			Lon:      info.Lon,
			AltM:     info.Alt,
			MsgCount: s.sensorMsgCount[id],
			LastSeen: s.sensorLastSeen[id],
		})
	}
	json.NewEncoder(w).Encode(data)
}

func (s *Server) handleTracks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	tracks := s.tracker.GetAllTracks()
	var data []TrackData
	for _, t := range tracks {
		td := TrackData{
			ICAO:       fmt.Sprintf("%06X", t.ICAO),
			Callsign:   t.Callsign,
			Lat:        t.Lat,
			Lon:        t.Lon,
			AltFt:      t.AltFt,
			SpeedKts:   t.SpeedKts,
			HeadingDeg: t.HeadingDeg,
			MlatCount:  t.MlatCount,
			AdsbCount:  t.AdsbCount,
			Coasted:    t.Coasted,
		}
		data = append(data, td)
	}
	json.NewEncoder(w).Encode(data)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	active, coasted, _, _ := s.tracker.Stats()
	json.NewEncoder(w).Encode(StatsData{
		TotalMessages: int(atomic.LoadInt64(&s.totalMessages)),
		CorrGroups:    int(atomic.LoadInt64(&s.corrGroups)),
		MlatSolved:    int(atomic.LoadInt64(&s.mlatSolved)),
		MlatFailed:    int(atomic.LoadInt64(&s.mlatFailed)),
		ActiveTracks:  active,
		CoastedTracks: coasted,
		SensorsOnline: len(s.sensors.All()),
	})
}

func (s *Server) handleAccuracy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if s.AccuracyFn != nil {
		json.NewEncoder(w).Encode(s.AccuracyFn())
	} else {
		json.NewEncoder(w).Encode(map[string]string{"status": "no data"})
	}
}

func (s *Server) handleSensorQuality(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if s.SensorQualityFn != nil {
		json.NewEncoder(w).Encode(s.SensorQualityFn())
	} else {
		json.NewEncoder(w).Encode(map[string]string{"status": "no data"})
	}
}
