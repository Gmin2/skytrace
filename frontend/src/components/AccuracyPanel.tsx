import { Crosshair } from 'lucide-react'
import type { AccuracyData } from '../types'

function fmt(m: number): string {
  if (m < 1000) return `${m.toFixed(0)}m`
  return `${(m / 1000).toFixed(1)}km`
}

export default function AccuracyPanel({ data, isDark }: { data: AccuracyData | null; isDark?: boolean }) {
  const sub = isDark ? 'text-[#555]' : 'text-[#999]'
  const val = isDark ? 'text-[#ddd]' : 'text-[#222]'

  if (!data || data.count === 0) return null

  return (
    <div>
      <div className={`${sub} text-xs tracking-widest mb-3 uppercase flex items-center gap-2`}>
        <Crosshair size={12} className="text-cyan-500" />
        MLAT Accuracy
      </div>

      <div className="grid grid-cols-3 gap-y-3 gap-x-2 mb-3">
        <div>
          <div className={`text-[10px] ${sub} uppercase`}>Mean</div>
          <div className={`text-sm font-medium tabular-nums ${val}`}>{fmt(data.mean_m)}</div>
        </div>
        <div>
          <div className={`text-[10px] ${sub} uppercase`}>Median</div>
          <div className={`text-sm font-medium tabular-nums ${val}`}>{fmt(data.median_m)}</div>
        </div>
        <div>
          <div className={`text-[10px] ${sub} uppercase`}>P90</div>
          <div className={`text-sm font-medium tabular-nums ${val}`}>{fmt(data.p90_m)}</div>
        </div>
      </div>

      <div className={`text-[10px] ${sub} mb-1.5 uppercase`}>Accuracy Distribution ({data.count} samples)</div>
      <div className="flex flex-col gap-1">
        <AccBar label="< 1km" count={data.under_1km} total={data.count} color="bg-emerald-500" isDark={isDark} />
        <AccBar label="< 5km" count={data.under_5km} total={data.count} color="bg-cyan-500" isDark={isDark} />
        <AccBar label="< 50km" count={data.count - data.under_5km} total={data.count} color="bg-yellow-500" isDark={isDark} />
      </div>
    </div>
  )
}

function AccBar({ label, count, total, color, isDark }: {
  label: string; count: number; total: number; color: string; isDark?: boolean
}) {
  const pct = total > 0 ? (count / total) * 100 : 0
  return (
    <div className="flex items-center gap-2 text-[10px]">
      <span className={`w-10 ${isDark ? 'text-[#888]' : 'text-[#666]'}`}>{label}</span>
      <div className={`flex-1 h-1.5 rounded-full ${isDark ? 'bg-[#1a1a1a]' : 'bg-[#ddd]'}`}>
        <div className={`h-full rounded-full ${color} transition-all duration-500`} style={{ width: `${Math.max(pct, 1)}%` }} />
      </div>
      <span className={`w-8 text-right tabular-nums ${isDark ? 'text-[#666]' : 'text-[#999]'}`}>{count}</span>
    </div>
  )
}
