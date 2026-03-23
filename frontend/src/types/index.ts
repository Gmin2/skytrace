export interface TrackData {
  icao: string
  callsign: string
  lat: number
  lon: number
  alt_ft: number
  speed_kts: number
  heading_deg: number
  vert_rate_fpm: number
  mlat_count: number
  adsb_count: number
  coasted: boolean
  history: HistoryPoint[] | null
}

export interface HistoryPoint {
  lat: number
  lon: number
  alt_ft: number
}

export interface SensorData {
  id: number
  name: string
  lat: number
  lon: number
  alt_m: number
  msg_count: number
  last_seen_ns: number
}

export interface StatsData {
  total_messages: number
  corr_groups: number
  mlat_solved: number
  mlat_failed: number
  active_tracks: number
  coasted_tracks: number
  sensors_online: number
}

export interface MLATFixData {
  icao: string
  lat: number
  lon: number
  alt_ft: number
  num_sensors: number
  residual: number
}

export interface AccuracyData {
  count: number
  mean_m: number
  median_m: number
  p90_m: number
  p95_m: number
  min_m: number
  max_m: number
  under_100m: number
  under_500m: number
  under_1km: number
  under_5km: number
}

export interface WSMessage {
  type: 'tracks' | 'sensors' | 'stats' | 'mlat_fix'
  data: unknown
}

export interface FeedEvent {
  id: number
  time: string
  type: string
  icao: string
  callsign: string
  detail: string
}
