import { Activity, Target, Crosshair, Radar } from 'lucide-react'
import type { StatsData } from '../types'

export default function StatsPanel({ stats, isDark }: { stats: StatsData | null; isDark?: boolean }) {
  const sub = isDark ? 'text-[#555]' : 'text-[#999]'
  const val = isDark ? 'text-[#ddd]' : 'text-[#222]'

  if (!stats) return <div className={`${sub} text-xs`}>Waiting for data...</div>

  const solveRate = stats.corr_groups > 0
    ? ((stats.mlat_solved / stats.corr_groups) * 100).toFixed(1)
    : '0'

  return (
    <div>
      <div className={`${sub} text-xs tracking-widest mb-4 uppercase flex items-center gap-2`}>
        <Activity size={12} className="text-cyan-500" />
        Pipeline
      </div>
      <div className="grid grid-cols-2 gap-y-4 gap-x-3">
        <Stat icon={<Radar size={14} />} label="Messages" value={stats.total_messages.toLocaleString()} sub={sub} val={val} />
        <Stat icon={<Target size={14} />} label="Groups" value={stats.corr_groups.toLocaleString()} sub={sub} val={val} />
        <Stat icon={<Crosshair size={14} />} label="MLAT Solved" value={`${stats.mlat_solved}`} extra={`${solveRate}%`} sub={sub} val={val} />
        <Stat label="Failed" value={`${stats.mlat_failed}`} sub={sub} val={val} />
        <Stat label="Active Tracks" value={`${stats.active_tracks}`} sub={sub} val={val} highlight />
        <Stat label="Sensors" value={`${stats.sensors_online}`} sub={sub} val={val} highlight />
      </div>
    </div>
  )
}

function Stat({ label, value, extra, highlight, icon, sub, val }: {
  label: string; value: string; extra?: string; highlight?: boolean; icon?: React.ReactNode; sub: string; val: string
}) {
  return (
    <div>
      <div className={`${sub} text-[10px] tracking-wider uppercase flex items-center gap-1`}>
        {icon}
        {label}
      </div>
      <div className={`text-base font-medium tabular-nums ${highlight ? 'text-cyan-400' : val}`}>
        {value}
        {extra && <span className={`text-[10px] ${sub} ml-1`}>{extra}</span>}
      </div>
    </div>
  )
}
