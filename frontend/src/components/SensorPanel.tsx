import { Radio } from 'lucide-react'
import type { SensorData } from '../types'
import type { SensorQualityData } from '../hooks/useAPIData'

interface Props {
  sensors: SensorData[]
  quality: SensorQualityData | null
  isDark?: boolean
}

export default function SensorPanel({ sensors, quality, isDark }: Props) {
  const sub = isDark ? 'text-[#666]' : 'text-[#999]'
  const label = isDark ? 'text-[#aaa]' : 'text-[#555]'
  const dim = isDark ? 'text-[#444]' : 'text-[#bbb]'

  return (
    <div>
      <div className={`${sub} text-xs tracking-widest mb-3 uppercase flex items-center gap-2`}>
        <Radio size={12} className="text-cyan-500" />
        Sensors ({sensors.length})
      </div>
      <div className="flex flex-col gap-2.5">
        {sensors.map(s => {
          const q = quality?.[String(s.id)]
          return (
            <div key={s.id} className={`text-xs ${isDark ? 'border-[#111]' : 'border-[#e0e0e0]'} border-b pb-2`}>
              <div className="flex items-center justify-between mb-1">
                <div className="flex items-center gap-2">
                  <div className="w-1.5 h-1.5 rounded-full bg-cyan-400 shadow-[0_0_4px_#06b6d4]" />
                  <span className={`${label} truncate max-w-[120px]`}>{s.name}</span>
                </div>
                <span className={`${dim} tabular-nums text-[10px]`}>{s.msg_count.toLocaleString()} msgs</span>
              </div>
              {q && (
                <div className={`flex gap-3 ml-4 text-[10px] ${dim}`}>
                  <span><span className="text-yellow-600">{q.mlat_contributions.toLocaleString()}</span> mlat</span>
                  <span><span className="text-cyan-600">{q.aircraft_count}</span> ac</span>
                  <span>{q.msg_rate_hz.toFixed(0)} Hz</span>
                </div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
