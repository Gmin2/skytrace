import { useState, useEffect, useRef } from 'react'
import type { TrackData, SensorData, StatsData, FeedEvent } from '../types'

const MOCK_SENSORS: SensorData[] = [
  { id: 1, name: 'London Heathrow', lat: 51.47, lon: -0.46, alt_m: 25, msg_count: 6709, last_seen_ns: 0 },
  { id: 2, name: 'Paris CDG', lat: 49.01, lon: 2.55, alt_m: 119, msg_count: 8740, last_seen_ns: 0 },
  { id: 3, name: 'New York JFK', lat: 40.64, lon: -73.78, alt_m: 4, msg_count: 2973, last_seen_ns: 0 },
  { id: 4, name: 'Dubai DXB', lat: 25.25, lon: 55.36, alt_m: 19, msg_count: 2202, last_seen_ns: 0 },
  { id: 5, name: 'Singapore SIN', lat: 1.35, lon: 103.99, alt_m: 7, msg_count: 12535, last_seen_ns: 0 },
  { id: 6, name: 'Tokyo NRT', lat: 35.76, lon: 140.39, alt_m: 41, msg_count: 3120, last_seen_ns: 0 },
]

const MOCK_AIRCRAFT: Omit<TrackData, 'history'>[] = [
  // Transatlantic: London → New York
  { icao: 'A8CECD', callsign: 'LXJ667', lat: 48.0, lon: -30.0, alt_ft: 43500, speed_kts: 465, heading_deg: 260, vert_rate_fpm: 0, mlat_count: 83, adsb_count: 176, coasted: false },
  // Europe → Middle East: Paris → Dubai
  { icao: '4CAD3C', callsign: 'RYR302F', lat: 40.0, lon: 25.0, alt_ft: 38000, speed_kts: 420, heading_deg: 120, vert_rate_fpm: -200, mlat_count: 113, adsb_count: 0, coasted: false },
  // Transatlantic return: New York → London
  { icao: 'AA1DCD', callsign: 'AAL39', lat: 52.0, lon: -20.0, alt_ft: 22400, speed_kts: 480, heading_deg: 70, vert_rate_fpm: 0, mlat_count: 118, adsb_count: 0, coasted: false },
  // Europe domestic: Paris → London
  { icao: '39CF13', callsign: 'AFR475', lat: 50.0, lon: 1.0, alt_ft: 35000, speed_kts: 450, heading_deg: 340, vert_rate_fpm: 0, mlat_count: 46, adsb_count: 89, coasted: false },
  // Asia route: Dubai → Singapore
  { icao: '48506B', callsign: 'TFL367', lat: 12.0, lon: 80.0, alt_ft: 37690, speed_kts: 430, heading_deg: 110, vert_rate_fpm: 0, mlat_count: 50, adsb_count: 120, coasted: false },
  // Asia route: Singapore → Tokyo
  { icao: '407E4E', callsign: 'EXS44CC', lat: 20.0, lon: 120.0, alt_ft: 36000, speed_kts: 480, heading_deg: 35, vert_rate_fpm: 0, mlat_count: 14, adsb_count: 67, coasted: false },
  // South Atlantic
  { icao: '402BB6', callsign: 'GBOOF', lat: 10.0, lon: -30.0, alt_ft: 41000, speed_kts: 490, heading_deg: 200, vert_rate_fpm: 0, mlat_count: 24, adsb_count: 0, coasted: false },
  // North America domestic
  { icao: '406440', callsign: 'EZY17UE', lat: 38.0, lon: -90.0, alt_ft: 28000, speed_kts: 410, heading_deg: 80, vert_rate_fpm: -500, mlat_count: 0, adsb_count: 145, coasted: false },
  // Polar route: London → Tokyo
  { icao: '40768B', callsign: 'EZY84PZ', lat: 65.0, lon: 60.0, alt_ft: 39000, speed_kts: 500, heading_deg: 55, vert_rate_fpm: 0, mlat_count: 8, adsb_count: 55, coasted: false },
  // Middle East → Europe
  { icao: '407937', callsign: 'AFZ35Y', lat: 35.0, lon: 40.0, alt_ft: 37000, speed_kts: 450, heading_deg: 310, vert_rate_fpm: 0, mlat_count: 24, adsb_count: 0, coasted: false },
  // Pacific crossing
  { icao: 'A2A2DF', callsign: 'UAL109', lat: 35.0, lon: 170.0, alt_ft: 36000, speed_kts: 490, heading_deg: 60, vert_rate_fpm: 0, mlat_count: 13, adsb_count: 41, coasted: false },
  // Africa route
  { icao: '3C671A', callsign: 'DLH460', lat: 5.0, lon: 10.0, alt_ft: 39000, speed_kts: 475, heading_deg: 180, vert_rate_fpm: 0, mlat_count: 0, adsb_count: 30, coasted: false },
]

export function useMockData() {
  const [tracks, setTracks] = useState<TrackData[]>([])
  const [sensors] = useState<SensorData[]>(MOCK_SENSORS)
  const [stats, setStats] = useState<StatsData>({
    total_messages: 132904,
    corr_groups: 2945,
    mlat_solved: 1410,
    mlat_failed: 1535,
    active_tracks: 12,
    coasted_tracks: 3,
    sensors_online: 6,
  })
  const [feed, setFeed] = useState<FeedEvent[]>([])
  const feedIdRef = useRef(0)

  useEffect(() => {
    // Initialize tracks with history
    const initial: TrackData[] = MOCK_AIRCRAFT.map(a => ({
      ...a,
      history: Array.from({ length: 30 }, (_, i) => ({
        lat: a.lat - (30 - i) * 0.3 * Math.cos(a.heading_deg * Math.PI / 180),
        lon: a.lon - (30 - i) * 0.3 * Math.sin(a.heading_deg * Math.PI / 180),
        alt_ft: a.alt_ft,
      })),
    }))
    setTracks(initial)

    // Simulate movement
    const interval = setInterval(() => {
      setTracks(prev => prev.map(t => {
        const hdgRad = t.heading_deg * Math.PI / 180
        const speed = 0.08 * (t.speed_kts / 400)
        const newLat = t.lat + speed * Math.cos(hdgRad)
        const newLon = t.lon + speed * Math.sin(hdgRad)
        const newAlt = t.alt_ft + t.vert_rate_fpm / 60

        const history = [...(t.history || []), { lat: newLat, lon: newLon, alt_ft: Math.round(newAlt) }]
        if (history.length > 60) history.shift()

        return { ...t, lat: newLat, lon: newLon, alt_ft: Math.round(newAlt), history }
      }))

      // Increment stats
      setStats(prev => ({
        ...prev,
        total_messages: prev.total_messages + Math.floor(Math.random() * 50) + 20,
        corr_groups: prev.corr_groups + Math.floor(Math.random() * 3),
        mlat_solved: prev.mlat_solved + Math.floor(Math.random() * 2),
      }))

      // Random feed events
      if (Math.random() > 0.3) {
        const ac = MOCK_AIRCRAFT[Math.floor(Math.random() * MOCK_AIRCRAFT.length)]
        const now = new Date()
        setFeed(prev => [{
          id: feedIdRef.current++,
          time: now.toLocaleTimeString('en-GB', { hour12: false }),
          type: Math.random() > 0.5 ? 'MLAT' : 'ADS-B',
          icao: ac.icao,
          callsign: ac.callsign,
          detail: `${ac.lat.toFixed(3)},${ac.lon.toFixed(3)} ${ac.alt_ft}ft`,
        }, ...prev].slice(0, 50))
      }
    }, 1000)

    return () => clearInterval(interval)
  }, [])

  return { tracks, sensors, stats, feed, connected: false }
}
