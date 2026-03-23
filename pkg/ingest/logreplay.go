package ingest

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"quickstart/pkg/modes"
)

// LogReplayer replays a log.txt file, parsing structured ModeS messages.
type LogReplayer struct {
	path string
}

func NewLogReplayer(path string) *LogReplayer {
	return &LogReplayer{path: path}
}

var (
	reSensorID  = regexp.MustCompile(`Sensor ID: (\d+)`)
	reSensorPos = regexp.MustCompile(`Sensor Position: Lat=([0-9.-]+), Lon=([0-9.-]+), Alt=([0-9.-]+)`)
	reTimestamp = regexp.MustCompile(`SecondsSinceMidnight=(\d+), Nanoseconds=(\d+)`)
	reRawHex    = regexp.MustCompile(`Raw ModeS \(hex\): ([0-9a-f]+)`)
)

// Replay reads the log file and sends parsed messages to the returned channel.
// It blocks until the entire file is read, then closes the channel.
func (r *LogReplayer) Replay() (<-chan *modes.RawMessage, error) {
	f, err := os.Open(r.path)
	if err != nil {
		return nil, fmt.Errorf("open log: %w", err)
	}

	ch := make(chan *modes.RawMessage, 1000)

	go func() {
		defer f.Close()
		defer close(ch)

		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

		var (
			sensorID    int64
			sensorLat   float64
			sensorLon   float64
			sensorAlt   float64
			secSinceMid uint64
			nanoseconds uint64
		)

		for scanner.Scan() {
			line := scanner.Text()

			if m := reSensorID.FindStringSubmatch(line); m != nil {
				sensorID, _ = strconv.ParseInt(m[1], 10, 64)
				continue
			}

			if m := reSensorPos.FindStringSubmatch(line); m != nil {
				sensorLat, _ = strconv.ParseFloat(m[1], 64)
				sensorLon, _ = strconv.ParseFloat(m[2], 64)
				sensorAlt, _ = strconv.ParseFloat(m[3], 64)
				continue
			}

			if m := reTimestamp.FindStringSubmatch(line); m != nil {
				secSinceMid, _ = strconv.ParseUint(m[1], 10, 64)
				nanoseconds, _ = strconv.ParseUint(m[2], 10, 64)
				continue
			}

			if m := reRawHex.FindStringSubmatch(line); m != nil {
				rawBytes, err := hex.DecodeString(strings.TrimSpace(m[1]))
				if err != nil {
					continue
				}

				msg := &modes.RawMessage{
					SensorID:    sensorID,
					SensorLat:   sensorLat,
					SensorLon:   sensorLon,
					SensorAlt:   sensorAlt,
					SecSinceMid: secSinceMid,
					Nanoseconds: nanoseconds,
					Raw:         rawBytes,
				}
				ch <- msg
			}
		}
	}()

	return ch, nil
}
