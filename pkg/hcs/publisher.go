package hcs

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// Publisher publishes MLAT results to an HCS topic.
// It rate-limits to avoid excessive Hedera fees.
type Publisher struct {
	mu          sync.Mutex
	sendFunc    func(topicID string, msg []byte) error
	topicID     string
	interval    time.Duration
	lastPublish time.Time
	pending     *PublishMessage
	enabled     bool
}

// PublishMessage is the JSON structure published to HCS.
type PublishMessage struct {
	Type      string        `json:"type"`
	Timestamp string        `json:"timestamp"`
	Tracks    []TrackUpdate `json:"tracks"`
	Stats     PipelineStats `json:"stats"`
}

type TrackUpdate struct {
	ICAO     string  `json:"icao"`
	Callsign string  `json:"callsign,omitempty"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	AltFt    int     `json:"alt_ft"`
	SpeedKts float64 `json:"speed_kts,omitempty"`
	Heading  float64 `json:"heading,omitempty"`
	Source   string  `json:"source"` // "mlat" or "adsb"
}

type PipelineStats struct {
	ActiveTracks  int `json:"active_tracks"`
	MlatSolvesMin int `json:"mlat_solves_per_min"`
	SensorsOnline int `json:"sensors_online"`
}

// NewPublisher creates a new HCS publisher.
// sendFunc is the function that actually sends to Hedera (nil = dry run / log only).
// interval controls rate limiting (e.g., 5 seconds between publishes).
func NewPublisher(topicID string, interval time.Duration, sendFunc func(string, []byte) error) *Publisher {
	p := &Publisher{
		topicID:  topicID,
		interval: interval,
		sendFunc: sendFunc,
		enabled:  sendFunc != nil,
	}

	// Start periodic flush
	go p.flushLoop()

	return p
}

// QueueTrackUpdate queues a track update for the next publish cycle.
func (p *Publisher) QueueTrackUpdate(tracks []TrackUpdate, stats PipelineStats) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.pending = &PublishMessage{
		Type:      "mlat_track_update",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Tracks:    tracks,
		Stats:     stats,
	}
}

func (p *Publisher) flushLoop() {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for range ticker.C {
		p.flush()
	}
}

func (p *Publisher) flush() {
	p.mu.Lock()
	msg := p.pending
	p.pending = nil
	p.mu.Unlock()

	if msg == nil || len(msg.Tracks) == 0 {
		return
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("HCS: failed to marshal message: %v", err)
		return
	}

	if p.enabled && p.sendFunc != nil {
		if err := p.sendFunc(p.topicID, data); err != nil {
			log.Printf("HCS: failed to publish to topic %s: %v", p.topicID, err)
		} else {
			log.Printf("HCS: published %d tracks to topic %s", len(msg.Tracks), p.topicID)
		}
	} else {
		// Dry run — just log
		log.Printf("HCS [dry-run]: would publish %d tracks (%d bytes) to topic %s",
			len(msg.Tracks), len(data), p.topicID)
	}

	p.lastPublish = time.Now()
}

// Stats returns the topic ID and publish state.
func (p *Publisher) Stats() string {
	if !p.enabled {
		return fmt.Sprintf("HCS: dry-run mode (topic=%s)", p.topicID)
	}
	return fmt.Sprintf("HCS: publishing to %s every %s", p.topicID, p.interval)
}
