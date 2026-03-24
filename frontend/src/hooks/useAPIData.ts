import { useState, useEffect } from 'react'
import type { AccuracyData } from '../types'

export interface SensorQualityData {
  [sensorId: string]: {
    msg_count: number
    mlat_contributions: number
    aircraft_count: number
    msg_rate_hz: number
  }
}

const API_BASE = import.meta.env.DEV
  ? ''
  : `http://${import.meta.env.VITE_BACKEND_URL || '167.71.231.68:8080'}`

export function useAPIData() {
  const [accuracy, setAccuracy] = useState<AccuracyData | null>(null)
  const [sensorQuality, setSensorQuality] = useState<SensorQualityData | null>(null)

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [accRes, sqRes] = await Promise.all([
          fetch(`${API_BASE}/api/accuracy`),
          fetch(`${API_BASE}/api/sensor-quality`),
        ])
        if (accRes.ok) setAccuracy(await accRes.json())
        if (sqRes.ok) setSensorQuality(await sqRes.json())
      } catch {
        // ignore fetch errors
      }
    }

    fetchData()
    const interval = setInterval(fetchData, 10000) // refresh every 10s
    return () => clearInterval(interval)
  }, [])

  return { accuracy, sensorQuality }
}
