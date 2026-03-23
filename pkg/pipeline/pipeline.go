package pipeline

import (
	"fmt"
	"log"
	"time"

	"quickstart/pkg/api"
	"quickstart/pkg/correlator"
	"quickstart/pkg/hcs"
	"quickstart/pkg/ingest"
	"quickstart/pkg/mlat"
	"quickstart/pkg/modes"
	"quickstart/pkg/tracker"
)

// Pipeline wires all components together.
type Pipeline struct {
	Registry      *ingest.SensorRegistry
	Correlator    *correlator.Correlator
	Tracker       *tracker.Manager
	Server        *api.Server
	HCS           *hcs.Publisher
	Accuracy      *tracker.AccuracyStats
	SensorQuality *tracker.SensorQuality

	// CPR state for ADS-B position decoding
	evenFrames map[uint32]*modes.DecodedMessage
	oddFrames  map[uint32]*modes.DecodedMessage

	// ADS-B last known positions for cross-validation
	adsbPositions map[uint32]adsbPos
}

type adsbPos struct {
	lat, lon float64
	altFt    int
	tsNs     int64
}

// New creates a new pipeline.
func New(overridePath string) (*Pipeline, error) {
	reg, err := ingest.NewSensorRegistry(overridePath)
	if err != nil {
		return nil, fmt.Errorf("load sensor registry: %w", err)
	}

	corr := correlator.New(2, 3) // 2ms window, min 3 sensors
	tm := tracker.NewManager()
	srv := api.NewServer(tm, reg)
	accuracy := tracker.NewAccuracyStats()
	sq := tracker.NewSensorQuality()

	// HCS publisher in dry-run mode (no Hedera credentials in replay mode)
	hcsPub := hcs.NewPublisher("0.0.skytrace", 5*time.Second, nil)

	srv.AccuracyFn = func() tracker.AccuracySummary { return accuracy.Summary() }
	srv.SensorQualityFn = func() map[int64]*tracker.SensorStats { return sq.GetAll() }

	p := &Pipeline{
		Registry:      reg,
		Correlator:    corr,
		Tracker:       tm,
		Server:        srv,
		HCS:           hcsPub,
		Accuracy:      accuracy,
		SensorQuality: sq,
		evenFrames:    make(map[uint32]*modes.DecodedMessage),
		oddFrames:     make(map[uint32]*modes.DecodedMessage),
		adsbPositions: make(map[uint32]adsbPos),
	}

	// Start correlation consumer
	go p.consumeCorrelations()

	return p, nil
}

// ProcessMessage handles a single raw message through the full pipeline.
func (p *Pipeline) ProcessMessage(raw *modes.RawMessage) {
	// Apply sensor altitude override
	info := p.Registry.Register(raw.SensorID, raw.SensorLat, raw.SensorLon)
	raw.SensorLat = info.Lat
	raw.SensorLon = info.Lon
	raw.SensorAlt = info.Alt

	// Record for stats
	p.Server.RecordMessage(raw.SensorID, raw.TimestampNs())

	// Decode
	decoded := modes.Decode(raw)

	// Sensor quality tracking
	if decoded.CRCValid && decoded.ICAO != 0 {
		p.SensorQuality.RecordMessage(raw.SensorID, decoded.ICAO, raw.TimestampNs())
	}

	// Feed correlator (DF11 and DF17 with valid CRC)
	p.Correlator.Add(decoded)

	// Process ADS-B data
	if decoded.DF == 17 && decoded.CRCValid {
		p.processADSB(decoded)
	}
}

func (p *Pipeline) processADSB(msg *modes.DecodedMessage) {
	// Callsign
	if msg.Callsign != "" {
		p.Tracker.SetCallsign(msg.ICAO, msg.Callsign)
	}

	// Velocity
	if msg.TypeCode == 19 && msg.VelocityKts > 0 {
		p.Tracker.SetVelocity(msg.ICAO, msg.VelocityKts, msg.HeadingDeg, msg.VertRateFpm)
	}

	// Position (CPR decode)
	if msg.TypeCode >= 9 && msg.TypeCode <= 18 {
		if msg.CPROddFlag {
			p.oddFrames[msg.ICAO] = msg
		} else {
			p.evenFrames[msg.ICAO] = msg
		}

		even, hasEven := p.evenFrames[msg.ICAO]
		odd, hasOdd := p.oddFrames[msg.ICAO]
		if hasEven && hasOdd {
			dtNs := even.TimestampNs() - odd.TimestampNs()
			if dtNs < 0 {
				dtNs = -dtNs
			}
			if dtNs < 10_000_000_000 { // 10 seconds
				lat, lon, ok := modes.CPRGlobalDecode(
					even.CPRLat, even.CPRLon,
					odd.CPRLat, odd.CPRLon,
					msg.CPROddFlag,
				)
				if ok {
					p.Tracker.ProcessADSBFix(msg.ICAO, lat, lon, msg.AltitudeFt, msg.TimestampNs())
					// Store for cross-validation
					p.adsbPositions[msg.ICAO] = adsbPos{
						lat: lat, lon: lon, altFt: msg.AltitudeFt, tsNs: msg.TimestampNs(),
					}
				}
			}
		}
	}
}

func (p *Pipeline) consumeCorrelations() {
	for group := range p.Correlator.Output() {
		result, err := mlat.Solve(group)
		if err != nil {
			p.Server.RecordCorrelation(false)
			continue
		}

		p.Server.RecordCorrelation(true)
		p.Tracker.ProcessMLATFix(result)

		// Record sensor quality contributions
		for _, rec := range group.Receptions {
			p.SensorQuality.RecordMLATContribution(rec.SensorID)
		}

		// Cross-validate with ADS-B ground truth
		if adsb, ok := p.adsbPositions[result.ICAO]; ok {
			// Only compare if ADS-B position is recent (within 5 seconds)
			dtSec := float64(result.TimestampNs-adsb.tsNs) / 1e9
			if dtSec >= 0 && dtSec < 5.0 {
				errorM := tracker.HaversineM(result.Lat, result.Lon, adsb.lat, adsb.lon)
				p.Accuracy.Record(errorM)
			}
		}

		// Queue for HCS publishing
		p.HCS.QueueTrackUpdate([]hcs.TrackUpdate{{
			ICAO:   fmt.Sprintf("%06X", result.ICAO),
			Lat:    result.Lat,
			Lon:    result.Lon,
			AltFt:  result.AltFt,
			Source: "mlat",
		}}, hcs.PipelineStats{
			SensorsOnline: len(p.Registry.All()),
		})

		// Broadcast to WebSocket clients
		p.Server.BroadcastMLATFix(api.MLATFixData{
			ICAO:       fmt.Sprintf("%06X", result.ICAO),
			Lat:        result.Lat,
			Lon:        result.Lon,
			AltFt:      result.AltFt,
			NumSensors: result.NumSensors,
			Residual:   result.Residual,
		})
	}
}

// RunReplay replays a log file through the pipeline.
func (p *Pipeline) RunReplay(logPath string, realtime bool) error {
	replayer := ingest.NewLogReplayer(logPath)
	ch, err := replayer.Replay()
	if err != nil {
		return err
	}

	var lastTs int64
	count := 0
	for raw := range ch {
		if realtime && lastTs > 0 {
			dt := raw.TimestampNs() - lastTs
			if dt > 0 && dt < 5_000_000_000 {
				time.Sleep(time.Duration(dt / 10)) // 10x speed
			}
		}
		lastTs = raw.TimestampNs()

		p.ProcessMessage(raw)
		count++

		if count%5000 == 0 {
			active, _, _, _ := p.Tracker.Stats()
			log.Printf("Processed %d messages, %d active tracks", count, active)
		}
	}

	// Flush remaining correlation groups
	p.Correlator.Close()
	time.Sleep(100 * time.Millisecond)

	// Print summary
	active, coasted, totalMlat, totalAdsb := p.Tracker.Stats()
	log.Printf("Replay complete: %d messages, %d tracks (active=%d coasted=%d), MLAT=%d ADS-B=%d",
		count, active+coasted, active, coasted, totalMlat, totalAdsb)

	// Print accuracy stats
	acc := p.Accuracy.Summary()
	if acc.Count > 0 {
		log.Printf("MLAT Accuracy (vs ADS-B): %d measurements, mean=%.0fm, median=%.0fm, p90=%.0fm",
			acc.Count, acc.MeanM, acc.MedianM, acc.P90M)
		log.Printf("  <100m: %d, <500m: %d, <1km: %d, <5km: %d",
			acc.Under100, acc.Under500, acc.Under1000, acc.Under5000)
	}

	// Print sensor quality
	log.Println("Sensor Quality:")
	for id, s := range p.SensorQuality.GetAll() {
		info := p.Registry.Get(id)
		name := fmt.Sprintf("%d", id)
		if info != nil {
			name = info.Name
		}
		log.Printf("  %-20s msgs=%d aircraft=%d mlat_contrib=%d rate=%.1f/s",
			name, s.MsgCount, s.AircraftCount, s.MlatContrib, s.MsgRateHz)
	}

	return nil
}

// StartServer starts the HTTP/WebSocket server.
func (p *Pipeline) StartServer(addr string) error {
	p.Server.StartBroadcastLoop()
	return p.Server.Start(addr)
}

// GetAccuracy returns the current accuracy summary.
func (p *Pipeline) GetAccuracy() tracker.AccuracySummary {
	return p.Accuracy.Summary()
}

// GetSensorQuality returns quality stats for all sensors.
func (p *Pipeline) GetSensorQuality() map[int64]*tracker.SensorStats {
	return p.SensorQuality.GetAll()
}
