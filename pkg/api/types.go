package api

// WebSocket message types sent to the frontend.

type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type TrackData struct {
	ICAO        string         `json:"icao"`
	Callsign    string         `json:"callsign"`
	Lat         float64        `json:"lat"`
	Lon         float64        `json:"lon"`
	AltFt       int            `json:"alt_ft"`
	SpeedKts    float64        `json:"speed_kts"`
	HeadingDeg  float64        `json:"heading_deg"`
	VertRateFpm int            `json:"vert_rate_fpm"`
	MlatCount   int            `json:"mlat_count"`
	AdsbCount   int            `json:"adsb_count"`
	Coasted     bool           `json:"coasted"`
	History     []HistoryPoint `json:"history"`
}

type HistoryPoint struct {
	Lat   float64 `json:"lat"`
	Lon   float64 `json:"lon"`
	AltFt int     `json:"alt_ft"`
}

type SensorData struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	AltM     float64 `json:"alt_m"`
	MsgCount int     `json:"msg_count"`
	LastSeen int64   `json:"last_seen_ns"`
}

type StatsData struct {
	TotalMessages  int `json:"total_messages"`
	CorrGroups     int `json:"corr_groups"`
	MlatSolved     int `json:"mlat_solved"`
	MlatFailed     int `json:"mlat_failed"`
	ActiveTracks   int `json:"active_tracks"`
	CoastedTracks  int `json:"coasted_tracks"`
	SensorsOnline  int `json:"sensors_online"`
}

type MLATFixData struct {
	ICAO       string  `json:"icao"`
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
	AltFt      int     `json:"alt_ft"`
	NumSensors int     `json:"num_sensors"`
	Residual   float64 `json:"residual"`
}
