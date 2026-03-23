package correlator

import (
	"encoding/hex"
	"fmt"
	"sort"
	"sync"

	"quickstart/pkg/modes"
)

// Correlator groups identical ModeS transmissions from multiple sensors.
// When the same raw bytes are received by 3+ sensors within a time window,
// it emits a Group suitable for MLAT solving.
type Correlator struct {
	mu         sync.Mutex
	pending    map[string]*pendingGroup
	windowNs   int64 // correlation window in nanoseconds
	minSensors int   // minimum sensors for a valid group
	output     chan *Group
}

type pendingGroup struct {
	icao       uint32
	rawHex     string
	receptions []Reception
	createdNs  int64
	sensorSeen map[int64]bool // prevent duplicate sensor in same group
}

// New creates a new Correlator.
// windowMs: correlation window in milliseconds (typically 2ms).
// minSensors: minimum number of distinct sensors (typically 3 or 4).
func New(windowMs int, minSensors int) *Correlator {
	return &Correlator{
		pending:    make(map[string]*pendingGroup),
		windowNs:   int64(windowMs) * 1_000_000,
		minSensors: minSensors,
		output:     make(chan *Group, 1000),
	}
}

// Output returns the channel of completed correlation groups.
func (c *Correlator) Output() <-chan *Group {
	return c.output
}

// Add processes a decoded message into the correlator.
func (c *Correlator) Add(msg *modes.DecodedMessage) {
	if !msg.CRCValid || msg.ICAO == 0 {
		return
	}

	// Only correlate DF11 and DF17 (they have reliable ICAO and identical content)
	if msg.DF != 11 && msg.DF != 17 {
		return
	}

	rawHex := hex.EncodeToString(msg.Raw)
	key := fmt.Sprintf("%06x:%s", msg.ICAO, rawHex)
	tsNs := msg.TimestampNs()

	reception := Reception{
		SensorID:    msg.SensorID,
		SensorLat:   msg.SensorLat,
		SensorLon:   msg.SensorLon,
		SensorAlt:   msg.SensorAlt,
		TimestampNs: tsNs,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	pg, exists := c.pending[key]
	if exists {
		// Check if within window
		if abs64(tsNs-pg.createdNs) <= c.windowNs {
			// Don't add duplicate sensor
			if !pg.sensorSeen[msg.SensorID] {
				pg.receptions = append(pg.receptions, reception)
				pg.sensorSeen[msg.SensorID] = true
			}
			return
		}
		// Window expired — flush the old group
		c.flush(pg)
		delete(c.pending, key)
	}

	// Start new pending group
	c.pending[key] = &pendingGroup{
		icao:       msg.ICAO,
		rawHex:     rawHex,
		receptions: []Reception{reception},
		createdNs:  tsNs,
		sensorSeen: map[int64]bool{msg.SensorID: true},
	}
}

// Flush forces all pending groups to be emitted (call at end of replay).
func (c *Correlator) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, pg := range c.pending {
		c.flush(pg)
		delete(c.pending, key)
	}
}

// Close flushes and closes the output channel.
func (c *Correlator) Close() {
	c.Flush()
	close(c.output)
}

func (c *Correlator) flush(pg *pendingGroup) {
	if len(pg.receptions) < c.minSensors {
		return
	}

	// Sort receptions by timestamp
	sort.Slice(pg.receptions, func(i, j int) bool {
		return pg.receptions[i].TimestampNs < pg.receptions[j].TimestampNs
	})

	// Deduplicate co-located sensors (keep only one per unique location)
	deduped := deduplicateColocated(pg.receptions)
	if len(deduped) < c.minSensors {
		return
	}

	g := &Group{
		ICAO:       pg.icao,
		RawHex:     pg.rawHex,
		Receptions: deduped,
		CreatedNs:  pg.createdNs,
	}

	select {
	case c.output <- g:
	default:
	}
}

// deduplicateColocated removes receptions from sensors at the same location
// (within 100m). Keeps the first reception from each unique location.
func deduplicateColocated(recs []Reception) []Reception {
	var result []Reception
	for _, r := range recs {
		colocated := false
		for _, existing := range result {
			dlat := r.SensorLat - existing.SensorLat
			dlon := r.SensorLon - existing.SensorLon
			// Quick distance check: ~0.001 degree ≈ 100m
			if dlat*dlat+dlon*dlon < 0.000001 {
				colocated = true
				break
			}
		}
		if !colocated {
			result = append(result, r)
		}
	}
	return result
}

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
