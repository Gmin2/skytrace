import { Terminal } from 'lucide-react'
import type { FeedEvent } from '../types'

export default function LiveFeed({ feed, isDark }: { feed: FeedEvent[]; isDark?: boolean }) {
  const sub = isDark ? 'text-[#666]' : 'text-[#999]'
  const dim = isDark ? 'text-[#444]' : 'text-[#bbb]'
  const line = isDark ? 'border-[#111]' : 'border-[#eee]'

  return (
    <div>
      <div className={`${sub} text-xs tracking-widest mb-3 uppercase flex items-center gap-2`}>
        <Terminal size={12} className="text-cyan-500" />
        Live Feed
      </div>
      <div className="flex flex-col gap-0.5 max-h-[300px] overflow-y-auto custom-scrollbar">
        {feed.length === 0 && (
          <div className={`${dim} text-[10px]`}>Waiting for events...</div>
        )}
        {feed.map(e => (
          <div key={e.id} className={`text-[10px] font-mono flex gap-2 py-0.5 border-b ${line}`}>
            <span className={`${dim} shrink-0`}>{e.time}</span>
            <span className={`shrink-0 w-10 ${
              e.type === 'MLAT' ? 'text-yellow-500' :
              e.type === 'SYSTEM' ? 'text-cyan-500' :
              'text-green-500'
            }`}>{e.type}</span>
            {e.icao && <span className={isDark ? 'text-[#888]' : 'text-[#666]'}>{e.icao}</span>}
            <span className={`${isDark ? 'text-[#555]' : 'text-[#aaa]'} truncate`}>{e.detail}</span>
          </div>
        ))}
      </div>
    </div>
  )
}
