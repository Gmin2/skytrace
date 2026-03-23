import { Plane } from 'lucide-react'
import type { TrackData } from '../types'

interface Props {
  tracks: TrackData[]
  selectedIcao: string | null
  onSelect: (icao: string | null) => void
  isDark?: boolean
}

export default function AircraftList({ tracks, selectedIcao, onSelect, isDark }: Props) {
  const sub = isDark ? 'text-[#666]' : 'text-[#999]'
  const hover = isDark ? 'hover:bg-[#111]' : 'hover:bg-[#e8e8e8]'
  const selectedBg = isDark ? 'bg-cyan-950/50 border-cyan-800/50' : 'bg-cyan-100/50 border-cyan-300'

  const sorted = [...tracks]
    .filter(t => !t.coasted)
    .sort((a, b) => (b.mlat_count + b.adsb_count) - (a.mlat_count + a.adsb_count))

  return (
    <div className="flex flex-col h-full min-h-0">
      <div className={`${sub} text-xs tracking-widest mb-3 uppercase flex items-center gap-2 shrink-0`}>
        <Plane size={12} className="text-cyan-500" />
        Aircraft ({sorted.length})
      </div>
      <div className="flex flex-col gap-0.5 overflow-y-auto custom-scrollbar flex-1 min-h-0">
        {sorted.map(t => {
          const isSelected = t.icao === selectedIcao
          const isMlat = t.mlat_count > 0 && t.adsb_count === 0
          const isBoth = t.mlat_count > 0 && t.adsb_count > 0
          return (
            <div
              key={t.icao}
              onClick={() => onSelect(t.icao === selectedIcao ? null : t.icao)}
              className={`flex items-center justify-between text-[11px] py-1 px-1.5 rounded cursor-pointer transition-colors border ${
                isSelected ? selectedBg : `${hover} border-transparent`
              }`}
            >
              <div className="flex items-center gap-2 min-w-0">
                <div className={`w-1.5 h-1.5 rounded-full shrink-0 ${
                  isMlat ? 'bg-yellow-500' : isBoth ? 'bg-orange-500' : 'bg-green-500'
                }`} />
                <span className={`${isDark ? 'text-[#ccc]' : 'text-[#333]'} font-bold shrink-0`}>{t.icao}</span>
                <span className={sub}>{t.callsign || ''}</span>
              </div>
              <div className="flex items-center gap-3 shrink-0 text-[10px]">
                <span className={`${isDark ? 'text-[#555]' : 'text-[#aaa]'} tabular-nums w-14 text-right`}>{t.alt_ft.toLocaleString()}ft</span>
                <span className="text-yellow-600 tabular-nums w-6 text-right">{t.mlat_count}</span>
                <span className="text-green-700 tabular-nums w-6 text-right">{t.adsb_count}</span>
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
