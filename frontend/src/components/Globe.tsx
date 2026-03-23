import { useEffect, useRef } from 'react'
import * as THREE from 'three'
import {
  CSS2DRenderer,
  CSS2DObject,
} from 'three/addons/renderers/CSS2DRenderer.js'
import { OrbitControls } from 'three/addons/controls/OrbitControls.js'
import type { TrackData, SensorData } from '../types'
import earthWaterUrl from '../assets/earth-water.png'

interface GlobeProps {
  tracks: TrackData[]
  sensors: SensorData[]
  selectedIcao: string | null
  onSelectTrack: (icao: string | null) => void
  isDark: boolean
}

const GLOBE_RADIUS = 1.4
const DOT_ROWS = 80
const DOT_SIZE = 0.007

const cities = [
  { name: 'London', lat: 51.5072, lon: -0.1276 },
  { name: 'New York', lat: 40.7128, lon: -74.006 },
  { name: 'San Francisco', lat: 37.7749, lon: -122.4194 },
  { name: 'Sydney', lat: -33.8688, lon: 151.2093 },
]

function latLonToVector3(lat: number, lon: number, radius: number) {
  const phi = (90 - lat) * (Math.PI / 180)
  const theta = (lon + 180) * (Math.PI / 180)
  return new THREE.Vector3(
    -(radius * Math.sin(phi) * Math.cos(theta)),
    radius * Math.cos(phi),
    radius * Math.sin(phi) * Math.sin(theta),
  )
}

/** Sample earth-water.png via fetch and return lat/lon pairs for land pixels */
async function loadLandPoints(): Promise<{ lat: number; lon: number }[]> {
  try {
    const resp = await fetch(earthWaterUrl)
    const blob = await resp.blob()
    const bitmap = await createImageBitmap(blob)

    const canvas = document.createElement('canvas')
    canvas.width = bitmap.width
    canvas.height = bitmap.height
    const ctx = canvas.getContext('2d')!
    ctx.drawImage(bitmap, 0, 0)
    const px = ctx.getImageData(0, 0, canvas.width, canvas.height).data

    const points: { lat: number; lon: number }[] = []

    for (let row = 0; row < DOT_ROWS; row++) {
      const lat = 90 - (row / DOT_ROWS) * 180
      const phi = (90 - lat) * (Math.PI / 180)
      const dotsInRow = Math.max(1, Math.floor(2 * DOT_ROWS * Math.sin(phi)))

      for (let col = 0; col < dotsInRow; col++) {
        const lon = (col / dotsInRow) * 360 - 180
        const imgX = Math.floor(((lon + 180) / 360) * canvas.width)
        const imgY = Math.floor(((90 - lat) / 180) * canvas.height)
        const idx = (imgY * canvas.width + imgX) * 4
        const r = px[idx]

        // earth-water.png: land = black (r≈0), water = white (r≈255)
        if (r < 80) {
          points.push({ lat, lon })
        }
      }
    }

    console.log(`[Globe] loaded ${points.length} land dots from ${canvas.width}x${canvas.height} texture`)
    return points
  } catch (err) {
    console.error('[Globe] failed to load land texture:', err)
    return []
  }
}

/** Create a curved arc between two points on the globe, bowing outward */
function createArc(
  start: THREE.Vector3,
  end: THREE.Vector3,
  segments = 48,
): THREE.Vector3[] {
  const points: THREE.Vector3[] = []
  const dist = start.distanceTo(end)
  // Arc height proportional to distance
  const arcHeight = Math.max(0.05, dist * 0.5)

  for (let i = 0; i <= segments; i++) {
    const t = i / segments
    // Spherical interpolation (slerp) between start and end
    const point = start.clone().lerp(end, t)
    // Normalize to globe surface then push outward with arc
    const elevation = Math.sin(t * Math.PI) * arcHeight
    point.normalize().multiplyScalar(GLOBE_RADIUS + 0.005 + elevation)
    points.push(point)
  }
  return points
}

export default function Globe({
  tracks,
  sensors,
  selectedIcao,
  onSelectTrack,
  isDark,
}: GlobeProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const sceneRef = useRef<{
    scene: THREE.Scene
    camera: THREE.PerspectiveCamera
    renderer: THREE.WebGLRenderer
    labelRenderer: CSS2DRenderer
    controls: OrbitControls
    globeGroup: THREE.Group
    landDots: THREE.InstancedMesh | null
    landDotMaterial: THREE.MeshBasicMaterial
    sensorGroup: THREE.Group
    sensorGeometry: THREE.SphereGeometry
    sensorMaterial: THREE.MeshBasicMaterial
    trackGroup: THREE.Group
    trackGeometry: THREE.SphereGeometry
    routeGroup: THREE.Group
    planeLabels: CSS2DObject[]
    cityLabels: { label: CSS2DObject; anchorLocal: THREE.Vector3 }[]
    globeFill: THREE.Mesh
    disposed: boolean
  } | null>(null)
  const selectedIcaoRef = useRef<string | null>(selectedIcao)
  const onSelectTrackRef = useRef(onSelectTrack)
  const isDarkRef = useRef(isDark)

  useEffect(() => { selectedIcaoRef.current = selectedIcao }, [selectedIcao])
  useEffect(() => { onSelectTrackRef.current = onSelectTrack }, [onSelectTrack])

  // Theme changes
  useEffect(() => {
    isDarkRef.current = isDark
    const s = sceneRef.current
    if (!s) return
    const bg = isDark ? '#050505' : '#e8e8ec'
    if (s.scene.background instanceof THREE.Color) {
      s.scene.background.set(bg)
    }
    const fillMat = s.globeFill.material as THREE.ShaderMaterial
    fillMat.uniforms.uDark.value = isDark ? 1.0 : 0.0
  }, [isDark])

  // Main scene setup — runs once
  useEffect(() => {
    if (!containerRef.current) return

    const rect = containerRef.current.getBoundingClientRect()
    const w = Math.max(1, Math.floor(rect.width))
    const h = Math.max(1, Math.floor(rect.height))

    // Scene
    const scene = new THREE.Scene()
    scene.background = new THREE.Color(isDarkRef.current ? '#050505' : '#e8e8ec')

    // Camera
    const camera = new THREE.PerspectiveCamera(45, w / h, 0.1, 100)
    camera.position.set(0, 0, 4.5)

    // Renderer
    const renderer = new THREE.WebGLRenderer({ antialias: true })
    renderer.setSize(w, h)
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2))
    containerRef.current.appendChild(renderer.domElement)

    // Label renderer
    const labelRenderer = new CSS2DRenderer()
    labelRenderer.setSize(w, h)
    labelRenderer.domElement.style.position = 'absolute'
    labelRenderer.domElement.style.top = '0'
    labelRenderer.domElement.style.pointerEvents = 'none'
    containerRef.current.appendChild(labelRenderer.domElement)

    // Controls
    const controls = new OrbitControls(camera, containerRef.current)
    controls.enableDamping = true
    controls.dampingFactor = 0.05
    controls.enablePan = false
    controls.minDistance = 2.5
    controls.maxDistance = 8
    controls.autoRotate = true
    controls.autoRotateSpeed = 0.6
    controls.rotateSpeed = 0.7

    const globeGroup = new THREE.Group()
    scene.add(globeGroup)

    // Globe sphere with fresnel rim — acts as both fill and edge outline
    const fillGeo = new THREE.SphereGeometry(GLOBE_RADIUS - 0.002, 64, 64)
    const fillMat = new THREE.ShaderMaterial({
      uniforms: {
        uDark: { value: isDarkRef.current ? 1.0 : 0.0 },
      },
      vertexShader: `
        varying vec3 vNormal;
        varying vec3 vViewDir;
        void main() {
          vNormal = normalize(normalMatrix * normal);
          vec4 mvPos = modelViewMatrix * vec4(position, 1.0);
          vViewDir = normalize(-mvPos.xyz);
          gl_Position = projectionMatrix * mvPos;
        }
      `,
      fragmentShader: `
        uniform float uDark;
        varying vec3 vNormal;
        varying vec3 vViewDir;
        void main() {
          float rim = 1.0 - max(dot(vNormal, vViewDir), 0.0);
          float rimPow = pow(rim, 3.0);

          // Dark mode: transparent center, visible rim edge
          vec3 darkColor = vec3(0.35) * rimPow;
          float darkAlpha = rimPow * 0.7 + 0.08;

          // Light mode: solid fill with subtle rim
          vec3 lightColor = mix(vec3(0.91, 0.91, 0.925), vec3(0.7), rimPow * 0.5);
          float lightAlpha = 0.95;

          vec3 color = mix(lightColor, darkColor, uDark);
          float alpha = mix(lightAlpha, darkAlpha, uDark);
          gl_FragColor = vec4(color, alpha);
        }
      `,
      transparent: true,
      depthWrite: false,
      side: THREE.FrontSide,
    })
    const globeFill = new THREE.Mesh(fillGeo, fillMat)
    globeGroup.add(globeFill)

    // Land dot material
    // White base — per-instance colors control brightness
    const landDotMaterial = new THREE.MeshBasicMaterial({ color: 0xffffff })

    // City labels
    const cityLabels: { label: CSS2DObject; anchorLocal: THREE.Vector3 }[] = []
    cities.forEach((city) => {
      const div = document.createElement('div')
      div.className = 'city-label-container transition-opacity duration-300'
      div.innerHTML = `
        <div class="city-name">${city.name}</div>
        <div class="city-arrow"></div>
        <div class="city-dot"></div>
      `
      const label = new CSS2DObject(div)
      const anchorLocal = latLonToVector3(city.lat, city.lon, GLOBE_RADIUS + 0.01)
      label.position.copy(anchorLocal.clone().normalize().multiplyScalar(GLOBE_RADIUS + 0.05))
      globeGroup.add(label)
      cityLabels.push({ label, anchorLocal })
    })

    // Sensor markers — visible station dots
    const sensorGroup = new THREE.Group()
    globeGroup.add(sensorGroup)
    const sensorGeometry = new THREE.SphereGeometry(0.025, 10, 10)
    const sensorMaterial = new THREE.MeshBasicMaterial({ color: 0x06b6d4 })

    // Track markers — aircraft position dots
    const trackGroup = new THREE.Group()
    globeGroup.add(trackGroup)
    const trackGeometry = new THREE.SphereGeometry(0.018, 8, 8)

    // Route trails — flight path lines
    const routeGroup = new THREE.Group()
    globeGroup.add(routeGroup)

    // Store refs
    const state = {
      scene,
      camera,
      renderer,
      labelRenderer,
      controls,
      globeGroup,
      landDots: null as THREE.InstancedMesh | null,
      landDotMaterial,
      sensorGroup,
      sensorGeometry,
      sensorMaterial,
      trackGroup,
      trackGeometry,
      routeGroup,
      planeLabels: [] as CSS2DObject[],
      cityLabels,
      globeFill,
      disposed: false,
    }
    sceneRef.current = state

    // Load land dots asynchronously
    loadLandPoints().then((points) => {
      if (state.disposed) return
      const dotGeo = new THREE.SphereGeometry(DOT_SIZE, 5, 5)
      const instancedMesh = new THREE.InstancedMesh(dotGeo, landDotMaterial, points.length)
      instancedMesh.instanceColor = new THREE.InstancedBufferAttribute(
        new Float32Array(points.length * 3), 3,
      )
      const dummy = new THREE.Object3D()
      const lightDir = new THREE.Vector3(1, 0.8, 0.6).normalize()
      const color = new THREE.Color()

      points.forEach((p, i) => {
        const pos = latLonToVector3(p.lat, p.lon, GLOBE_RADIUS + 0.002)
        dummy.position.copy(pos)
        dummy.lookAt(pos.clone().multiplyScalar(2))
        dummy.updateMatrix()
        instancedMesh.setMatrixAt(i, dummy.matrix)

        // Per-dot lighting: brighter on sunlit side
        const normal = pos.clone().normalize()
        const ndotl = Math.max(0, normal.dot(lightDir))
        const brightness = 0.08 + 0.25 * ndotl // dim side ~0.08, bright side ~0.33
        color.setRGB(brightness, brightness, brightness)
        instancedMesh.setColorAt(i, color)
      })
      instancedMesh.instanceMatrix.needsUpdate = true
      instancedMesh.instanceColor.needsUpdate = true
      globeGroup.add(instancedMesh)
      state.landDots = instancedMesh
    })

    // Animation
    let animationFrameId: number
    const animate = () => {
      if (state.disposed) return
      animationFrameId = requestAnimationFrame(animate)

      controls.update()

      // Hide city labels on back side
      const camPos = camera.position
      cityLabels.forEach(({ label, anchorLocal }) => {
        const world = anchorLocal.clone().applyMatrix4(globeGroup.matrixWorld)
        const normal = world.clone().normalize()
        const viewDir = camPos.clone().sub(world).normalize()
        label.element.style.opacity = normal.dot(viewDir) < 0.15 ? '0' : '1'
      })

      renderer.render(scene, camera)
      labelRenderer.render(scene, camera)
    }
    animate()

    // Resize
    const handleResize = () => {
      if (!containerRef.current) return
      const r = containerRef.current.getBoundingClientRect()
      const rw = Math.max(1, Math.floor(r.width))
      const rh = Math.max(1, Math.floor(r.height))
      camera.aspect = rw / rh
      camera.updateProjectionMatrix()
      renderer.setSize(rw, rh)
      labelRenderer.setSize(rw, rh)
    }
    window.addEventListener('resize', handleResize)

    // Click to select track
    const handlePointerDown = (event: PointerEvent) => {
      if (!containerRef.current) return
      const r = containerRef.current.getBoundingClientRect()
      const pointer = new THREE.Vector2(
        ((event.clientX - r.left) / r.width) * 2 - 1,
        -((event.clientY - r.top) / r.height) * 2 + 1,
      )
      const raycaster = new THREE.Raycaster()
      raycaster.setFromCamera(pointer, camera)
      const hits = raycaster.intersectObjects(trackGroup.children, true)
      if (hits.length) {
        const icao = hits[0].object.userData?.icao
        if (typeof icao === 'string') {
          const already = selectedIcaoRef.current === icao
          onSelectTrackRef.current(already ? null : icao)
        }
      }
    }
    renderer.domElement.addEventListener('pointerdown', handlePointerDown)

    // Cleanup
    return () => {
      state.disposed = true
      window.removeEventListener('resize', handleResize)
      renderer.domElement.removeEventListener('pointerdown', handlePointerDown)
      cancelAnimationFrame(animationFrameId)
      controls.dispose()
      renderer.dispose()
      fillGeo.dispose()
      fillMat.dispose()
      sensorGeometry.dispose()
      sensorMaterial.dispose()
      trackGeometry.dispose()
      landDotMaterial.dispose()
      if (state.landDots) {
        state.landDots.geometry.dispose()
      }
      renderer.domElement.remove()
      labelRenderer.domElement.remove()
      scene.clear()
      sceneRef.current = null
    }
  }, [])

  // Update sensor markers
  useEffect(() => {
    const s = sceneRef.current
    if (!s) return
    sensors.forEach((sensor, i) => {
      let mesh = s.sensorGroup.children[i] as THREE.Mesh | undefined
      if (!mesh) {
        mesh = new THREE.Mesh(s.sensorGeometry, s.sensorMaterial)
        s.sensorGroup.add(mesh)
      }
      mesh.position.copy(latLonToVector3(sensor.lat, sensor.lon, GLOBE_RADIUS + 0.005))
    })
  }, [sensors])

  // Update track markers and route arcs — performance optimized
  useEffect(() => {
    const s = sceneRef.current
    if (!s) return
    const { trackGroup, routeGroup, globeGroup } = s

    // Show all non-coasted aircraft
    const visible = tracks.filter(t => !t.coasted)
    const visibleIcaos = new Set(visible.map(t => t.icao))

    // Reuse or create track meshes (no CSS2D — pure 3D dots)
    const existingMeshes = new Map<string, THREE.Mesh>()
    trackGroup.children.forEach(child => {
      if (child.userData?.icao) existingMeshes.set(child.userData.icao as string, child as THREE.Mesh)
    })

    // Remove meshes for tracks no longer visible
    existingMeshes.forEach((mesh, icao) => {
      if (!visibleIcaos.has(icao)) {
        trackGroup.remove(mesh)
        mesh.geometry.dispose()
        ;(mesh.material as THREE.Material).dispose()
      }
    })

    // Materials cache
    const matCache: Record<string, THREE.MeshBasicMaterial> = {
      mlat: new THREE.MeshBasicMaterial({ color: 0xfacc15 }),
      adsb: new THREE.MeshBasicMaterial({ color: 0x22c55e }),
      both: new THREE.MeshBasicMaterial({ color: 0xf97316 }),
    }

    visible.forEach(track => {
      const isMlat = track.mlat_count > 0
      const isBoth = isMlat && track.adsb_count > 0
      const matKey = isMlat ? (isBoth ? 'both' : 'mlat') : 'adsb'
      const isSelected = track.icao === selectedIcao

      const pos = latLonToVector3(track.lat, track.lon, GLOBE_RADIUS + 0.015)
      let mesh = existingMeshes.get(track.icao)

      if (!mesh) {
        mesh = new THREE.Mesh(s.trackGeometry, matCache[matKey])
        mesh.userData = { icao: track.icao }
        trackGroup.add(mesh)
      }

      mesh.position.copy(pos)
      mesh.scale.setScalar(isSelected ? 2.5 : 1)
    })

    // Clear and rebuild route arcs (reuse geometry pool)
    routeGroup.children.forEach(child => {
      if (child instanceof THREE.Line) {
        child.geometry.dispose()
        ;(child.material as THREE.Material).dispose()
      }
    })
    routeGroup.clear()

    // Draw trails for all aircraft
    visible.forEach(track => {
      if (!track.history || track.history.length < 2) return
      const isMlat = track.mlat_count > 0
      const isBoth = isMlat && track.adsb_count > 0
      const trackColor = isMlat ? (isBoth ? 0xf97316 : 0xfacc15) : 0x22c55e

      const histStart = track.history[0]
      const histEnd = track.history[track.history.length - 1]
      const startVec = latLonToVector3(histStart.lat, histStart.lon, GLOBE_RADIUS)
      const endVec = latLonToVector3(histEnd.lat, histEnd.lon, GLOBE_RADIUS)
      const arcPts = createArc(startVec, endVec, 16) // fewer segments
      const lineGeo = new THREE.BufferGeometry().setFromPoints(arcPts)
      const lineMat = new THREE.LineBasicMaterial({ color: trackColor, transparent: true, opacity: 0.5 })
      routeGroup.add(new THREE.Line(lineGeo, lineMat))
    })

    // Selected aircraft label — only ONE CSS2D label at a time
    s.planeLabels.forEach(l => { globeGroup.remove(l); l.element.remove() })
    s.planeLabels = []

    if (selectedIcao) {
      const sel = tracks.find(t => t.icao === selectedIcao)
      if (sel) {
        const div = document.createElement('div')
        div.style.cssText = 'font:bold 10px monospace;color:#06b6d4;background:rgba(0,0,0,0.8);padding:2px 6px;border-radius:3px;border:1px solid #06b6d4;white-space:nowrap;pointer-events:none;'
        div.textContent = `${sel.callsign || sel.icao} ${sel.alt_ft}ft`
        const label = new CSS2DObject(div)
        label.position.copy(latLonToVector3(sel.lat, sel.lon, GLOBE_RADIUS + 0.06))
        globeGroup.add(label)
        s.planeLabels.push(label)
      }
    }
  }, [tracks, selectedIcao])

  return (
    <div className={`relative w-full h-full overflow-hidden ${isDark ? 'globe-dark' : 'globe-light'}`}>
      <div ref={containerRef} className="absolute inset-0 cursor-move" />
      <style>{`
        .globe-dark { background: #000; }
        .globe-light { background: #e8e8ec; }
        .plane-icon { transition: font-size 0.2s; }
        .city-label-container {
          display: flex;
          flex-direction: column;
          align-items: center;
          margin-top: -12px;
        }
        .city-name {
          font-size: clamp(9px, 1.1vw, 11px);
          padding: clamp(3px, 0.55vw, 4px) clamp(5px, 0.8vw, 6px);
          border-radius: 2px;
          font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
          white-space: nowrap;
          letter-spacing: 0.05em;
          transition: background-color 0.3s, color 0.3s;
        }
        .globe-dark .city-name {
          background: #fff;
          color: #000;
        }
        .globe-light .city-name {
          background: #000;
          color: #fff;
        }
        .city-arrow {
          width: 0;
          height: 0;
          border-left: 4px solid transparent;
          border-right: 4px solid transparent;
          margin-bottom: 2px;
          transition: border-top-color 0.3s;
        }
        .globe-dark .city-arrow { border-top: 4px solid #fff; }
        .globe-light .city-arrow { border-top: 4px solid #000; }
        .city-dot {
          width: 4px;
          height: 4px;
          border-radius: 50%;
          transition: background-color 0.3s;
        }
        .globe-dark .city-dot { background: #fff; }
        .globe-light .city-dot { background: #000; }
      `}</style>
    </div>
  )
}
