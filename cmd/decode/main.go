// cmd/decode/main.go
// Standalone ModeS decoder for verification.
// Usage: go run cmd/decode/main.go < log.txt
// Or:    go run cmd/decode/main.go --hex 8d406440ea447864013c08792c43
package main

import (
	"bufio"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"quickstart/pkg/modes"
)

var (
	hexFlag  = flag.String("hex", "", "Decode a single hex ModeS frame")
	logFile  = flag.String("log", "", "Parse a log.txt file and print stats")
	verbose  = flag.Bool("v", false, "Print every decoded message")
)

func main() {
	flag.Parse()

	if *hexFlag != "" {
		decodeSingle(*hexFlag)
		return
	}

	if *logFile != "" {
		decodeLogFile(*logFile)
		return
	}

	fmt.Println("Usage:")
	fmt.Println("  go run cmd/decode/main.go --hex 8d406440ea447864013c08792c43")
	fmt.Println("  go run cmd/decode/main.go --log log.txt")
	fmt.Println("  go run cmd/decode/main.go --log log.txt -v")
}

func decodeSingle(hexStr string) {
	raw, err := hex.DecodeString(strings.TrimSpace(hexStr))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid hex: %v\n", err)
		os.Exit(1)
	}

	msg := modes.Decode(&modes.RawMessage{Raw: raw})
	printDecoded(msg)
}

func decodeLogFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open %s: %v\n", path, err)
		os.Exit(1)
	}
	defer f.Close()

	reHex := regexp.MustCompile(`Raw ModeS \(hex\): ([0-9a-f]+)`)
	reSensor := regexp.MustCompile(`Sensor ID: (\d+)`)
	rePos := regexp.MustCompile(`Sensor Position: Lat=([0-9.-]+), Lon=([0-9.-]+)`)

	var (
		totalMessages int
		dfCounts      = make(map[int]int)
		icaoSet       = make(map[uint32]bool)
		callsigns     = make(map[uint32]string)
		crcPass       int
		crcFail       int
		positionMsgs  int
		velocityMsgs  int
		identMsgs     int
		sensorSet     = make(map[string]bool)
		// CPR pairs for position decoding
		evenFrames = make(map[uint32]*modes.DecodedMessage)
		oddFrames  = make(map[uint32]*modes.DecodedMessage)
		positions  int
	)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		if m := reSensor.FindStringSubmatch(line); m != nil {
			sensorSet[m[1]] = true
		}
		if m := rePos.FindStringSubmatch(line); m != nil {
			_ = m // sensor position tracking
		}

		m := reHex.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		raw, err := hex.DecodeString(m[1])
		if err != nil {
			continue
		}

		totalMessages++
		msg := modes.Decode(&modes.RawMessage{Raw: raw})
		dfCounts[msg.DF]++

		if msg.CRCValid {
			crcPass++
			icaoSet[msg.ICAO] = true
		} else {
			crcFail++
		}

		if msg.DF == 17 && msg.CRCValid {
			switch {
			case msg.TypeCode >= 1 && msg.TypeCode <= 4:
				identMsgs++
				if msg.Callsign != "" {
					callsigns[msg.ICAO] = msg.Callsign
				}
			case msg.TypeCode >= 9 && msg.TypeCode <= 18:
				positionMsgs++
				// Try CPR pairing
				if msg.CPROddFlag {
					oddFrames[msg.ICAO] = msg
				} else {
					evenFrames[msg.ICAO] = msg
				}
				// Attempt global decode
				even, hasEven := evenFrames[msg.ICAO]
				odd, hasOdd := oddFrames[msg.ICAO]
				if hasEven && hasOdd {
					lat, lon, ok := modes.CPRGlobalDecode(
						even.CPRLat, even.CPRLon,
						odd.CPRLat, odd.CPRLon,
						msg.CPROddFlag,
					)
					if ok {
						positions++
						if *verbose {
							fmt.Printf("  -> POSITION: ICAO=%06X lat=%.4f lon=%.4f alt=%dft\n", msg.ICAO, lat, lon, msg.AltitudeFt)
						}
					}
				}
			case msg.TypeCode == 19:
				velocityMsgs++
			}
		}

		if *verbose && msg.CRCValid {
			printDecoded(msg)
		}
	}

	// Print summary
	fmt.Println("=== ModeS Decoder Statistics ===")
	fmt.Printf("Total messages:     %d\n", totalMessages)
	fmt.Printf("CRC pass:           %d (%.1f%%)\n", crcPass, 100*float64(crcPass)/float64(totalMessages))
	fmt.Printf("CRC fail:           %d (%.1f%%)\n", crcFail, 100*float64(crcFail)/float64(totalMessages))
	fmt.Printf("Unique ICAO:        %d\n", len(icaoSet))
	fmt.Printf("Unique sensors:     %d\n", len(sensorSet))
	fmt.Println()

	fmt.Println("--- Downlink Format Distribution ---")
	for df := 0; df <= 24; df++ {
		if c, ok := dfCounts[df]; ok {
			fmt.Printf("  DF%-2d: %6d  (%s)\n", df, c, dfName(df))
		}
	}
	fmt.Println()

	fmt.Println("--- DF17 ADS-B Breakdown ---")
	fmt.Printf("  Identification (TC 1-4):   %d\n", identMsgs)
	fmt.Printf("  Position (TC 9-18):        %d\n", positionMsgs)
	fmt.Printf("  Velocity (TC 19):          %d\n", velocityMsgs)
	fmt.Printf("  Positions decoded (CPR):   %d\n", positions)
	fmt.Println()

	if len(callsigns) > 0 {
		fmt.Println("--- Callsigns ---")
		for icao, cs := range callsigns {
			fmt.Printf("  %06X: %s\n", icao, cs)
		}
	}
}

func printDecoded(msg *modes.DecodedMessage) {
	fmt.Printf("DF=%d ICAO=%06X CRC=%v", msg.DF, msg.ICAO, msg.CRCValid)
	if msg.DF == 17 {
		fmt.Printf(" TC=%d", msg.TypeCode)
		if msg.Callsign != "" {
			fmt.Printf(" Callsign=%s", msg.Callsign)
		}
		if msg.TypeCode >= 9 && msg.TypeCode <= 18 {
			odd := "even"
			if msg.CPROddFlag {
				odd = "odd"
			}
			fmt.Printf(" Alt=%dft CPR(%s)=(%d,%d)", msg.AltitudeFt, odd, msg.CPRLat, msg.CPRLon)
		}
		if msg.TypeCode == 19 && msg.VelocityKts > 0 {
			fmt.Printf(" Speed=%.0fkts Hdg=%.0f° VRate=%dfpm", msg.VelocityKts, msg.HeadingDeg, msg.VertRateFpm)
		}
	}
	fmt.Printf(" [%x]\n", msg.Raw)
}

func dfName(df int) string {
	switch df {
	case 0:
		return "Short Air-Air Surveillance"
	case 4:
		return "Surveillance Altitude Reply"
	case 5:
		return "Surveillance Identity Reply"
	case 11:
		return "All-Call Reply"
	case 16:
		return "Long Air-Air Surveillance"
	case 17:
		return "ADS-B Extended Squitter"
	case 18:
		return "TIS-B / ADS-R"
	case 19:
		return "Military Extended Squitter"
	case 20:
		return "Comm-B Altitude Reply"
	case 21:
		return "Comm-B Identity Reply"
	case 24:
		return "Comm-D Extended Length"
	default:
		return "Unknown"
	}
}
