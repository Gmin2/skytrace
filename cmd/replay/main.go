// cmd/replay/main.go
// Replays log.txt through the ingestion layer and prints stats.
// Verifies: message count, sensor matching, altitude overrides.
// Usage: go run cmd/replay/main.go --log log.txt --overrides location-override.json
package main

import (
	"flag"
	"fmt"
	"os"

	"quickstart/pkg/correlator"
	"quickstart/pkg/ingest"
	"quickstart/pkg/mlat"
	"quickstart/pkg/modes"
	"quickstart/pkg/tracker"
)

func main() {
	logPath := flag.String("log", "log.txt", "Path to log file")
	overridePath := flag.String("overrides", "location-override.json", "Path to sensor location overrides")
	flag.Parse()

	// Load sensor registry
	registry, err := ingest.NewSensorRegistry(*overridePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load sensor overrides: %v\n", err)
		os.Exit(1)
	}

	// Start replay
	replayer := ingest.NewLogReplayer(*logPath)
	ch, err := replayer.Replay()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start replay: %v\n", err)
		os.Exit(1)
	}

	// Set up correlator
	corr := correlator.New(2, 3) // 2ms window, min 3 sensors

	// Collect correlation groups in background
	type groupStats struct {
		total      int
		bySensors  map[int]int
		maxTDOAus  float64
		sampleICAO uint32
	}
	gs := &groupStats{bySensors: make(map[int]int)}
	var mlatSolved, mlatFailed int
	tm := tracker.NewManager()
	corrDone := make(chan struct{})
	go func() {
		printed := 0
		for g := range corr.Output() {
			gs.total++
			gs.bySensors[len(g.Receptions)]++
			tdoa := g.TDOA()
			for _, t := range tdoa {
				us := t * 1e6
				if us > gs.maxTDOAus {
					gs.maxTDOAus = us
				}
			}

			// Try MLAT solve
			result, err := mlat.Solve(g)
			if err != nil {
				mlatFailed++
			} else {
				mlatSolved++
				tm.ProcessMLATFix(result)
				if printed < 5 {
					fmt.Printf("  [MLAT] ICAO=%06X sensors=%d → lat=%.4f lon=%.4f alt=%dft residual=%.0fm\n",
						result.ICAO, result.NumSensors, result.Lat, result.Lon, result.AltFt, result.Residual)
					printed++
				}
			}
		}
		close(corrDone)
	}()

	var (
		total       int
		sensorCount = make(map[int64]int)
		dfCount     = make(map[int]int)
	)

	for raw := range ch {
		total++
		sensorCount[raw.SensorID]++

		// Register sensor on first sight
		info := registry.Register(raw.SensorID, raw.SensorLat, raw.SensorLon)
		// Apply altitude override
		raw.SensorAlt = info.Alt

		// Decode and feed correlator + tracker
		decoded := modes.Decode(raw)
		dfCount[decoded.DF]++
		corr.Add(decoded)

		// Feed ADS-B data to tracker
		if decoded.DF == 17 && decoded.CRCValid {
			if decoded.Callsign != "" {
				tm.SetCallsign(decoded.ICAO, decoded.Callsign)
			}
			if decoded.TypeCode == 19 && decoded.VelocityKts > 0 {
				tm.SetVelocity(decoded.ICAO, decoded.VelocityKts, decoded.HeadingDeg, decoded.VertRateFpm)
			}
		}
	}

	// Flush remaining groups
	corr.Close()
	<-corrDone

	fmt.Println("\n=== Replay Statistics ===")
	fmt.Printf("Total messages: %d\n\n", total)

	fmt.Println("--- Sensor Registry ---")
	fmt.Printf("%-14s %-20s %10s %10s %8s %8s\n", "SensorID", "Name", "Lat", "Lon", "Alt(m)", "Msgs")
	for id, count := range sensorCount {
		info := registry.Get(id)
		if info != nil {
			fmt.Printf("%-14d %-20s %10.6f %10.6f %8.1f %8d\n",
				id, info.Name, info.Lat, info.Lon, info.Alt, count)
		} else {
			fmt.Printf("%-14d %-20s %10s %10s %8s %8d\n",
				id, "UNMATCHED", "-", "-", "-", count)
		}
	}

	fmt.Println("\n--- DF Distribution ---")
	for df := 0; df <= 24; df++ {
		if c, ok := dfCount[df]; ok {
			fmt.Printf("  DF%-2d: %6d\n", df, c)
		}
	}

	fmt.Println("\n--- Correlation Groups ---")
	fmt.Printf("Total groups (3+ sensors): %d\n", gs.total)
	fmt.Printf("Max TDOA: %.1f us\n", gs.maxTDOAus)
	for n := 3; n <= 6; n++ {
		if c, ok := gs.bySensors[n]; ok {
			fmt.Printf("  %d sensors: %d groups\n", n, c)
		}
	}

	fmt.Println("\n--- MLAT Results ---")
	fmt.Printf("Solved:  %d (%.1f%%)\n", mlatSolved, 100*float64(mlatSolved)/float64(gs.total))
	fmt.Printf("Failed:  %d (%.1f%%)\n", mlatFailed, 100*float64(mlatFailed)/float64(gs.total))

	// Print tracker summary
	tracks := tm.GetAllTracks()
	fmt.Printf("\n--- Tracked Aircraft: %d ---\n", len(tracks))
	fmt.Printf("%-8s %-10s %9s %9s %7s %6s %6s %5s %5s\n",
		"ICAO", "Callsign", "Lat", "Lon", "Alt(ft)", "Spd", "Hdg", "MLAT", "ADS-B")
	for _, t := range tracks {
		cs := t.Callsign
		if cs == "" {
			cs = "-"
		}
		fmt.Printf("%-8s %-10s %9.4f %9.4f %7d %6.0f %6.0f %5d %5d\n",
			fmt.Sprintf("%06X", t.ICAO), cs, t.Lat, t.Lon, t.AltFt,
			t.SpeedKts, t.HeadingDeg, t.MlatCount, t.AdsbCount)
	}
}
