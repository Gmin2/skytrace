import { useEffect, useMemo, useRef } from 'react'
import { MapContainer, TileLayer, CircleMarker, Polyline, Popup, Tooltip, Marker, useMap } from 'react-leaflet'
import L from 'leaflet'
import type { TrackData, SensorData } from '../types'
import 'leaflet/dist/leaflet.css'

// No trail limit — show all
const ARC_SEGMENTS = 16

interface MapProps {
  tracks: TrackData[]
  sensors: SensorData[]
  selectedIcao: string | null
  onSelectTrack: (icao: string | null) => void
  isDark: boolean
}

function getTrackColor(t: TrackData) {
  const isMlat = t.mlat_count > 0
  const isBoth = isMlat && t.adsb_count > 0
  return isMlat ? (isBoth ? '#f97316' : '#facc15') : '#22c55e'
}

function parabolicArc(start: [number, number], end: [number, number]): [number, number][] {
  const [lat1, lon1] = start
  const [lat2, lon2] = end
  const dx = lon2 - lon1
  const dy = lat2 - lat1
  const dist = Math.sqrt(dx * dx + dy * dy)
  const arcHeight = Math.max(0.002, dist * 0.3)
  const nx = -dy / (dist || 1)
  const ny = dx / (dist || 1)
  const points: [number, number][] = []
  for (let i = 0; i <= ARC_SEGMENTS; i++) {
    const t = i / ARC_SEGMENTS
    const lat = lat1 + (lat2 - lat1) * t
    const lon = lon1 + (lon2 - lon1) * t
    const bulge = 4 * t * (1 - t) * arcHeight
    points.push([lat + nx * bulge, lon + ny * bulge])
  }
  return points
}

const iconCache = new Map<string, L.DivIcon>()
function getPlaneIcon(heading: number, color: string, selected: boolean) {
  const hdg = Math.round(heading / 10) * 10
  const key = `${hdg}-${color}-${selected}`
  let icon = iconCache.get(key)
  if (!icon) {
    const size = selected ? 32 : 24
    const glow = selected ? 10 : 6
    const stroke = selected ? 'rgba(0,0,0,0.6)' : 'rgba(0,0,0,0.4)'
    icon = L.divIcon({
      className: '',
      iconSize: [size, size],
      iconAnchor: [size / 2, size / 2],
      html: `<div style="
        width:${size}px;height:${size}px;
        transform:rotate(${hdg}deg);
        filter:drop-shadow(0 0 ${glow}px ${color}) drop-shadow(0 1px 2px rgba(0,0,0,0.5));
      "><svg viewBox="0 0 24 24" width="${size}" height="${size}">
        <path d="M21 16v-2l-8-5V3.5A1.5 1.5 0 0 0 11.5 2 1.5 1.5 0 0 0 10 3.5V9l-8 5v2l8-2.5V19l-2 1.5V22l3.5-1 3.5 1v-1.5L13 19v-5.5l8 2.5z"
          fill="${color}" stroke="${stroke}" stroke-width="0.8"/>
      </svg></div>`,
    })
    iconCache.set(key, icon)
    if (iconCache.size > 200) {
      const first = iconCache.keys().next().value
      if (first) iconCache.delete(first)
    }
  }
  return icon
}

function MapController({ selectedIcao, tracks }: { selectedIcao: string | null; tracks: TrackData[] }) {
  const map = useMap()
  useEffect(() => {
    if (selectedIcao) {
      const t = tracks.find(t => t.icao === selectedIcao)
      // Pan to selected aircraft without changing zoom
      if (t) map.panTo([t.lat, t.lon], { duration: 0.5 })
    }
  }, [selectedIcao, tracks, map])
  return null
}

export default function SkyMap({ tracks, sensors, selectedIcao, onSelectTrack, isDark }: MapProps) {
  const tileUrl = isDark
    ? 'https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png'
    : 'https://{s}.basemaps.cartocdn.com/light_all/{z}/{x}/{y}{r}.png'

  const visibleTracks = useMemo(() => {
    return tracks.filter(t => !t.coasted)
  }, [tracks])

  const trailTracks = visibleTracks

  return (
    <MapContainer
      center={[50.5, -4.0]}
      zoom={6}
      minZoom={2}
      maxZoom={16}
      className="w-full h-full"
      zoomControl={false}
      attributionControl={false}
      style={{ background: isDark ? '#050505' : '#e8e8ec' }}
    >
      <TileLayer url={tileUrl} />
      <MapController selectedIcao={selectedIcao} tracks={visibleTracks} />

      {sensors.map(s => (
        <CircleMarker
          key={s.id}
          center={[s.lat, s.lon]}
          radius={7}
          pathOptions={{ color: '#06b6d4', fillColor: '#06b6d4', fillOpacity: 0.8, weight: 2 }}
        >
          <Tooltip permanent direction="top" offset={[0, -8]}
            className="!bg-black/80 !text-cyan-400 !border-cyan-800 !text-[10px] !font-mono !px-1.5 !py-0.5 !rounded !shadow-lg"
          >
            {s.name}
          </Tooltip>
        </CircleMarker>
      ))}

      {trailTracks.map(t => {
        if (!t.history || t.history.length < 2) return null
        const isSelected = t.icao === selectedIcao
        const color = getTrackColor(t)
        const start: [number, number] = [t.history[0].lat, t.history[0].lon]
        const end: [number, number] = [t.lat, t.lon]
        return (
          <Polyline
            key={`trail-${t.icao}`}
            positions={parabolicArc(start, end)}
            pathOptions={{ color, weight: isSelected ? 3 : 1.5, opacity: isSelected ? 0.9 : 0.4 }}
          />
        )
      })}

      {visibleTracks.map(t => (
        <AircraftMarker
          key={`ac-${t.icao}`}
          track={t}
          isSelected={t.icao === selectedIcao}
          onSelect={onSelectTrack}
        />
      ))}
    </MapContainer>
  )
}

function AircraftMarker({ track: t, isSelected, onSelect }: {
  track: TrackData; isSelected: boolean; onSelect: (icao: string | null) => void
}) {
  const markerRef = useRef<L.Marker>(null)
  const color = getTrackColor(t)

  // Auto-open popup when selected from sidebar
  useEffect(() => {
    if (isSelected && markerRef.current) {
      markerRef.current.openPopup()
    }
  }, [isSelected])

  return (
    <Marker
      ref={markerRef}
      position={[t.lat, t.lon]}
      icon={getPlaneIcon(t.heading_deg, color, isSelected)}
      eventHandlers={{ click: () => onSelect(isSelected ? null : t.icao) }}
    >
      <Popup className="!bg-[#111] !text-gray-100 !border-gray-700 !rounded-lg !font-mono !shadow-xl">
        <div className="text-xs space-y-1 min-w-[180px]">
          <div className="text-base font-bold text-cyan-400">{t.icao}</div>
          {t.callsign && <div className="text-sm text-white">{t.callsign}</div>}
          <div className="h-px bg-gray-700 my-1" />
          <div className="flex justify-between"><span className="text-gray-400">Alt</span><span>{t.alt_ft.toLocaleString()} ft</span></div>
          <div className="flex justify-between"><span className="text-gray-400">Speed</span><span>{t.speed_kts.toFixed(0)} kts</span></div>
          <div className="flex justify-between"><span className="text-gray-400">Heading</span><span>{t.heading_deg.toFixed(0)}°</span></div>
          <div className="h-px bg-gray-700 my-1" />
          <div className="flex justify-between"><span className="text-yellow-500">MLAT</span><span>{t.mlat_count}</span></div>
          <div className="flex justify-between"><span className="text-green-500">ADS-B</span><span>{t.adsb_count}</span></div>
        </div>
      </Popup>
    </Marker>
  )
}
