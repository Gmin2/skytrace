package ingest

import (
	"encoding/binary"
	"fmt"
	"math"

	"quickstart/pkg/modes"
)

const (
	sensorIDSize    = 8
	sensorLatSize   = 8
	sensorLonSize   = 8
	sensorAltSize   = 8
	secSinceMidSize = 8
	nanosecondsSize = 8
	minFixedSize    = sensorIDSize + sensorLatSize + sensorLonSize + sensorAltSize + secSinceMidSize + nanosecondsSize
)

// ParsePacket parses a binary packet (without the length byte) into a RawMessage.
func ParsePacket(data []byte) (*modes.RawMessage, error) {
	if len(data) < minFixedSize {
		return nil, fmt.Errorf("packet too short: %d bytes", len(data))
	}

	off := 0
	msg := &modes.RawMessage{}

	msg.SensorID = int64(binary.BigEndian.Uint64(data[off : off+8]))
	off += 8

	msg.SensorLat = math.Float64frombits(binary.BigEndian.Uint64(data[off : off+8]))
	off += 8

	msg.SensorLon = math.Float64frombits(binary.BigEndian.Uint64(data[off : off+8]))
	off += 8

	msg.SensorAlt = math.Float64frombits(binary.BigEndian.Uint64(data[off : off+8]))
	off += 8

	msg.SecSinceMid = binary.BigEndian.Uint64(data[off : off+8])
	off += 8

	msg.Nanoseconds = binary.BigEndian.Uint64(data[off : off+8])
	off += 8

	msg.Raw = make([]byte, len(data)-off)
	copy(msg.Raw, data[off:])

	return msg, nil
}
