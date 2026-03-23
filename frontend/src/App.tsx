import { useState } from 'react'
import { Wifi, WifiOff, Sun, Moon, Globe2, Map as MapIcon } from 'lucide-react'
// import { useMockData } from './hooks/useMockData'
import { useWebSocket } from './hooks/useWebSocket'
import { useAPIData } from './hooks/useAPIData'
import Globe from './components/Globe'
import SkyMap from './components/Map'
import SensorPanel from './components/SensorPanel'
import StatsPanel from './components/StatsPanel'
import AccuracyPanel from './components/AccuracyPanel'
import DePINFlow from './components/DePINFlow'
import AircraftList from './components/AircraftList'

export default function App() {
  const { tracks, sensors, stats, connected } = useWebSocket()
  const { accuracy, sensorQuality } = useAPIData()
  const [selectedIcao, setSelectedIcao] = useState<string | null>(null)
  const [isDark, setIsDark] = useState(true)
  const [viewMode, setViewMode] = useState<'globe' | 'map'>('globe')

  const bg = isDark ? 'bg-[#050505]' : 'bg-[#e8e8ec]'
  const text = isDark ? 'text-[#e0e0e0]' : 'text-[#1a1a1a]'
  const border = isDark ? 'border-[#1a1a1a]' : 'border-[#ddd]'
  const muted = isDark ? 'text-[#444]' : 'text-[#999]'
  const divider = isDark ? 'bg-[#1a1a1a]' : 'bg-[#ddd]'

  return (
    <div className={`h-screen ${bg} ${text} font-mono flex flex-col overflow-hidden selection:bg-cyan-800 selection:text-white transition-colors duration-500`}>

      {/* Header */}
      <header className={`flex justify-between items-center px-6 py-3 border-b ${border} shrink-0 z-10`}>
        <div className="flex items-center gap-3">
          <div className="w-2 h-2 rounded-full bg-cyan-500 shadow-[0_0_8px_#06b6d4]" />
          <h1 className="text-lg font-bold tracking-[0.2em] uppercase">
            SkyTrace
            <span className={`${muted} text-xs font-normal tracking-normal ml-2`}>// Decentralized MLAT</span>
          </h1>
        </div>
        <div className="flex items-center gap-4 text-xs">
          <div className="flex items-center gap-2">
            {connected
              ? <><Wifi size={14} className="text-cyan-500" /><span className="text-cyan-500">LIVE</span></>
              : <><WifiOff size={14} className="text-red-500" /><span className="text-red-500">OFFLINE</span></>
            }
          </div>
          <div className={muted}>
            {stats ? `${stats.sensors_online} sensors` : '...'}
          </div>
          <div className={muted}>
            {stats ? `${stats.active_tracks} tracks` : '...'}
          </div>
          <button
            onClick={() => setIsDark(!isDark)}
            className={`p-1.5 rounded-full ${isDark ? 'hover:bg-white/10' : 'hover:bg-black/10'} transition-colors cursor-pointer`}
          >
            {isDark ? <Sun size={14} className="text-[#888]" /> : <Moon size={14} className="text-[#555]" />}
          </button>
        </div>
      </header>

      {/* Main content */}
      <div className="flex-1 flex min-h-0">

        {/* Left sidebar */}
        <div className={`w-[280px] shrink-0 border-r ${border} overflow-y-auto custom-scrollbar p-4 flex flex-col gap-5`}>
          <DePINFlow stats={stats} isDark={isDark} />
          <div className={`h-px ${divider}`} />
          <StatsPanel stats={stats} isDark={isDark} />
          <div className={`h-px ${divider}`} />
          <AccuracyPanel data={accuracy} isDark={isDark} />
          <div className={`h-px ${divider}`} />
          <SensorPanel sensors={sensors} quality={sensorQuality} isDark={isDark} />
        </div>

        {/* Globe / Map */}
        <div className="flex-1 relative">
          {viewMode === 'globe' ? (
            <Globe
              tracks={tracks}
              sensors={sensors}
              selectedIcao={selectedIcao}
              onSelectTrack={setSelectedIcao}
              isDark={isDark}
            />
          ) : (
            <SkyMap
              tracks={tracks}
              sensors={sensors}
              selectedIcao={selectedIcao}
              onSelectTrack={setSelectedIcao}
              isDark={isDark}
            />
          )}
          {/* View toggle */}
          <div className="absolute top-3 right-3 z-[1000] flex gap-1">
            <button
              onClick={() => setViewMode('globe')}
              className={`p-2 rounded-lg backdrop-blur-md transition-colors cursor-pointer ${
                viewMode === 'globe'
                  ? isDark ? 'bg-white/20 text-white' : 'bg-black/20 text-black'
                  : isDark ? 'bg-white/5 text-[#666] hover:bg-white/10' : 'bg-black/5 text-[#999] hover:bg-black/10'
              }`}
              title="3D Globe"
            >
              <Globe2 size={16} />
            </button>
            <button
              onClick={() => setViewMode('map')}
              className={`p-2 rounded-lg backdrop-blur-md transition-colors cursor-pointer ${
                viewMode === 'map'
                  ? isDark ? 'bg-white/20 text-white' : 'bg-black/20 text-black'
                  : isDark ? 'bg-white/5 text-[#666] hover:bg-white/10' : 'bg-black/5 text-[#999] hover:bg-black/10'
              }`}
              title="Flat Map"
            >
              <MapIcon size={16} />
            </button>
          </div>
          {/* Legend */}
          <div className={`absolute bottom-3 left-3 ${isDark ? 'bg-black/70' : 'bg-white/80'} backdrop-blur-sm rounded px-3 py-2 text-[10px] flex gap-4 z-[1000]`}>
            <div className="flex items-center gap-1.5">
              <div className="w-2 h-2 rounded-full bg-yellow-500" />
              <span className={isDark ? 'text-[#888]' : 'text-[#666]'}>MLAT only</span>
            </div>
            <div className="flex items-center gap-1.5">
              <div className="w-2 h-2 rounded-full bg-green-500" />
              <span className={isDark ? 'text-[#888]' : 'text-[#666]'}>ADS-B only</span>
            </div>
            <div className="flex items-center gap-1.5">
              <div className="w-2 h-2 rounded-full bg-orange-500" />
              <span className={isDark ? 'text-[#888]' : 'text-[#666]'}>Both</span>
            </div>
            <div className="flex items-center gap-1.5">
              <div className="w-2 h-2 rounded-full bg-cyan-500" />
              <span className={isDark ? 'text-[#888]' : 'text-[#666]'}>Sensor</span>
            </div>
          </div>
        </div>

        {/* Right sidebar */}
        <div className={`w-[320px] shrink-0 border-l ${border} p-4 flex flex-col min-h-0`}>
          <AircraftList tracks={tracks} selectedIcao={selectedIcao} onSelect={setSelectedIcao} isDark={isDark} />
        </div>
      </div>
    </div>
  )
}
