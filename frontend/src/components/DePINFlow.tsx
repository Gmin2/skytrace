import { Search, Link, Coins, Cpu, Upload } from 'lucide-react'
import type { StatsData } from '../types'

const steps = [
  { icon: Search, label: 'Discover', desc: 'Find sensors on Hedera', color: 'text-cyan-400' },
  { icon: Link, label: 'Connect', desc: 'P2P via libp2p/QUIC', color: 'text-blue-400' },
  { icon: Coins, label: 'Pay', desc: 'Shared accounts on HCS', color: 'text-yellow-400' },
  { icon: Cpu, label: 'Compute', desc: 'MLAT at the edge', color: 'text-green-400' },
  { icon: Upload, label: 'Publish', desc: 'Results on-chain', color: 'text-purple-400' },
]

export default function DePINFlow({ stats, isDark }: { stats: StatsData | null; isDark?: boolean }) {
  const sub = isDark ? 'text-[#555]' : 'text-[#999]'
  const line = isDark ? 'bg-[#222]' : 'bg-[#ccc]'

  return (
    <div>
      <div className={`${sub} text-xs tracking-widest mb-3 uppercase flex items-center gap-2`}>
        <Link size={12} className="text-cyan-500" />
        DePIN Flow
      </div>
      <div className="flex items-center justify-between relative">
        {/* Connecting line */}
        <div className={`absolute top-3 left-4 right-4 h-px ${line}`} />

        {steps.map((step, i) => {
          const Icon = step.icon
          const isActive = stats && (
            (i === 0 && stats.sensors_online > 0) ||
            (i === 1 && stats.total_messages > 0) ||
            (i === 2 && stats.total_messages > 0) ||
            (i === 3 && stats.mlat_solved > 0) ||
            (i === 4 && stats.mlat_solved > 0)
          )
          return (
            <div key={step.label} className="flex flex-col items-center gap-1 relative z-10">
              <div className={`w-6 h-6 rounded-full flex items-center justify-center ${
                isActive
                  ? isDark ? 'bg-[#111] border border-cyan-800' : 'bg-white border border-cyan-400'
                  : isDark ? 'bg-[#0a0a0a] border border-[#222]' : 'bg-[#e0e0e0] border border-[#ccc]'
              }`}>
                <Icon size={10} className={isActive ? step.color : isDark ? 'text-[#444]' : 'text-[#aaa]'} />
              </div>
              <span className={`text-[8px] ${isActive ? (isDark ? 'text-[#aaa]' : 'text-[#444]') : (isDark ? 'text-[#333]' : 'text-[#bbb]')}`}>
                {step.label}
              </span>
            </div>
          )
        })}
      </div>
    </div>
  )
}
