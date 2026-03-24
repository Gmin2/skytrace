import { useEffect, useRef, useState } from 'react'
import type { TrackData, SensorData, StatsData, WSMessage } from '../types'

const BACKEND = import.meta.env.VITE_BACKEND_URL || '167.71.231.68:8080'

const WS_URL = import.meta.env.DEV
  ? `ws://${window.location.hostname}:5173/ws`
  : `ws://${BACKEND}/ws`

const RECONNECT_DELAY = 2000
const TRACK_THROTTLE_MS = 3000
const MAX_HISTORY = 20

export function useWebSocket() {
  const [tracks, setTracks] = useState<TrackData[]>([])
  const [sensors, setSensors] = useState<SensorData[]>([])
  const [stats, setStats] = useState<StatsData | null>(null)
  const [connected, setConnected] = useState(false)
  const lastTrackUpdateRef = useRef(0)
  const pendingTracksRef = useRef<TrackData[] | null>(null)

  useEffect(() => {
    let reconnectTimer: ReturnType<typeof setTimeout>
    let trackFlushTimer: ReturnType<typeof setTimeout>
    let ws: WebSocket

    const flushTracks = () => {
      const pending = pendingTracksRef.current
      if (pending) {
        // Trim history to MAX_HISTORY points
        const trimmed = pending.map(t => ({
          ...t,
          history: t.history ? t.history.slice(-MAX_HISTORY) : null,
        }))
        setTracks(trimmed)
        pendingTracksRef.current = null
        lastTrackUpdateRef.current = Date.now()
      }
    }

    const connect = () => {
      ws = new WebSocket(WS_URL)

      ws.onopen = () => {
        setConnected(true)
      }

      ws.onclose = () => {
        setConnected(false)
        reconnectTimer = setTimeout(connect, RECONNECT_DELAY)
      }

      ws.onerror = () => ws.close()

      ws.onmessage = (event) => {
        try {
          const msg: WSMessage = JSON.parse(event.data)

          switch (msg.type) {
            case 'tracks': {
              // Throttle track updates
              pendingTracksRef.current = msg.data as TrackData[]
              const elapsed = Date.now() - lastTrackUpdateRef.current
              if (elapsed >= TRACK_THROTTLE_MS) {
                flushTracks()
              } else {
                clearTimeout(trackFlushTimer)
                trackFlushTimer = setTimeout(flushTracks, TRACK_THROTTLE_MS - elapsed)
              }
              break
            }
            case 'sensors':
              setSensors(msg.data as SensorData[])
              break
            case 'stats':
              setStats(msg.data as StatsData)
              break
            case 'mlat_fix':
              // Ignored for performance — feed removed
              break
          }
        } catch {
          // ignore
        }
      }
    }

    connect()

    return () => {
      clearTimeout(reconnectTimer)
      clearTimeout(trackFlushTimer)
      ws?.close()
    }
  }, [])

  return { tracks, sensors, stats, connected }
}
