import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { MapContainer, TileLayer, Marker, Popup, Circle } from 'react-leaflet'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import {
  Mail,
  MessageCircle,
  MessageSquare,
  MapPin,
  Search,
  Download,
  Send,
  Copy,
  Activity,
  AlertTriangle,
  X,
} from 'lucide-react'
import { adminService, type ProfessionalLocation } from '../services/admin.service'
import { buildTemplateOptions } from '../lib/emergencyTemplates'
import { EmergencyTemplatesModal } from '../components/Admin/EmergencyTemplatesModal'
import { COUNTRY_OPTIONS, getStatesForCountry } from '../components/Auth/countries'
import { COUNTRY_CENTROIDS, VE_STATE_CENTROIDS, type LatLng } from '../lib/regionCentroids'
import { Select } from '../components/ui/Select'
import Avatar from '../components/Common/Avatar'
import styles from './ProfessionalsMap.module.css'

type ActiveFilter = '' | 'true' | 'false'
type CheckInStatus = 'pendiente' | 'contactado' | 'ok' | 'sin_respuesta'

const ACTIVE_OPTIONS = [
  { value: '', label: 'Todos' },
  { value: 'true', label: 'Activos' },
  { value: 'false', label: 'Inactivos' },
]

const CHECKIN_OPTIONS: { value: CheckInStatus; label: string }[] = [
  { value: 'pendiente', label: 'Pendiente' },
  { value: 'contactado', label: 'Contactado' },
  { value: 'ok', label: 'OK' },
  { value: 'sin_respuesta', label: 'Sin respuesta' },
]

const CHECKIN_COLOR: Record<CheckInStatus, string> = {
  ok: '#16a34a',
  contactado: '#d97706',
  sin_respuesta: '#dc2626',
  pendiente: '#94a3b8',
}

const USGS_FEEDS = [
  { value: '2.5_day', label: 'Mag 2.5+ · 24h' },
  { value: '4.5_week', label: 'Mag 4.5+ · 7 días' },
  { value: 'significant_week', label: 'Significativos · 7 días' },
]

interface Quake {
  id: string
  mag: number
  place: string
  time: number
  lat: number
  lng: number
}

interface FollowUpItem {
  user_id: number
  status: string
}

const countBadgeIcon = (count: number, atRisk: boolean) =>
  L.divIcon({
    className: 'pmap-badge',
    html: `<div class="${styles['marker-badge']}" style="background:${atRisk ? '#dc2626' : '#2563eb'}">${count}</div>`,
    iconSize: [34, 34],
    iconAnchor: [17, 17],
  })

const quakeIcon = (mag: number) =>
  L.divIcon({
    className: 'pmap-quake',
    html: `<div class="${styles['quake-marker']}">${mag.toFixed(1)}</div>`,
    iconSize: [30, 30],
    iconAnchor: [15, 15],
  })

const personPinIcon = (name: string) =>
  L.divIcon({
    className: 'pmap-person',
    html: `<div class="${styles['person-pin']}"><span>${(name || '?').slice(0, 2).toUpperCase()}</span></div>`,
    iconSize: [30, 38],
    iconAnchor: [15, 38],
    popupAnchor: [0, -34],
  })

const whatsappHref = (phone: string) => {
  const digits = (phone || '').replace(/\D/g, '')
  return digits ? `https://wa.me/${digits}` : null
}


const haversineKm = (a: LatLng, b: LatLng) => {
  const R = 6371
  const dLat = ((b[0] - a[0]) * Math.PI) / 180
  const dLng = ((b[1] - a[1]) * Math.PI) / 180
  const lat1 = (a[0] * Math.PI) / 180
  const lat2 = (b[0] * Math.PI) / 180
  const h =
    Math.sin(dLat / 2) ** 2 + Math.sin(dLng / 2) ** 2 * Math.cos(lat1) * Math.cos(lat2)
  return 2 * R * Math.asin(Math.sqrt(h))
}

const csvCell = (v: unknown) => `"${String(v ?? '').replace(/"/g, '""')}"`

const fetchQuakes = async (feed: string): Promise<Quake[]> => {
  const res = await fetch(
    `https://earthquake.usgs.gov/earthquakes/feed/v1.0/summary/${feed}.geojson`,
  )
  if (!res.ok) throw new Error('USGS feed error')
  const json = await res.json()
  return (json.features ?? []).map((f: any) => ({
    id: f.id,
    mag: f.properties?.mag ?? 0,
    place: f.properties?.place ?? 'Desconocido',
    time: f.properties?.time ?? 0,
    lng: f.geometry?.coordinates?.[0],
    lat: f.geometry?.coordinates?.[1],
  }))
}

export default function ProfessionalsMap() {
  const navigate = useNavigate()
  const qc = useQueryClient()
  const [country, setCountry] = useState('')
  const [state, setState] = useState('')
  const [searchParams, setSearchParams] = useSearchParams()

  useEffect(() => {
    const c = searchParams.get('country')
    const s = searchParams.get('state')
    if (!c && !s) return
    if (c) setCountry(c)
    if (s) setState(s)
    setSearchParams({}, { replace: true })
  }, [searchParams, setSearchParams])
  const [active, setActive] = useState<ActiveFilter>('')
  const [search, setSearch] = useState('')
  const [selectedRegion, setSelectedRegion] = useState<string | null>(null)
  const mapRef = useRef<L.Map | null>(null)
  const [focusedId, setFocusedId] = useState<number | null>(null)

  const focusRegion = (region: string, pos?: [number, number]) => {
    setSelectedRegion(region)
    if (pos && mapRef.current) {
      mapRef.current.flyTo(pos, Math.max(mapRef.current.getZoom(), 6), { duration: 0.8 })
    }
  }

  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set())
  const [contactOpen, setContactOpen] = useState(false)
  const [contactList, setContactList] = useState<ProfessionalLocation[]>([])

  const [showQuakes, setShowQuakes] = useState(false)
  const [quakeFeed, setQuakeFeed] = useState('4.5_week')
  const [radiusKm, setRadiusKm] = useState(150)

  const { data, isLoading, isError } = useQuery({
    queryKey: ['professional-locations', country, state, active],
    queryFn: () => adminService.getProfessionalLocations({ country, state, active }),
  })

  const checkInQuery = useQuery({
    queryKey: ['follow-ups', 'emergencia'],
    queryFn: async () => {
      const items = (await adminService.getFollowUps('emergencia')) as FollowUpItem[]
      const map: Record<number, CheckInStatus> = {}
      for (const it of items) map[it.user_id] = it.status as CheckInStatus
      return map
    },
  })
  const checkIns = checkInQuery.data ?? {}

  const quakesQuery = useQuery({
    queryKey: ['usgs-quakes', quakeFeed],
    queryFn: () => fetchQuakes(quakeFeed),
    enabled: showQuakes,
    refetchInterval: showQuakes ? 5 * 60 * 1000 : false,
  })
  const quakes = quakesQuery.data ?? []

  const all = data?.professionals ?? []

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase()
    if (!q) return all
    return all.filter(
      (p) => p.name?.toLowerCase().includes(q) || p.email?.toLowerCase().includes(q),
    )
  }, [all, search])

  const withLocation = useMemo(() => filtered.filter((p) => p.country), [filtered])
  const noLocation = useMemo(() => filtered.filter((p) => !p.country), [filtered])

  const useStateLevel = country === 'Venezuela'

  const regionKeyOf = (p: ProfessionalLocation) => (useStateLevel ? p.state : p.country)

  const personPos = (p: ProfessionalLocation): LatLng | undefined => {
    if (p.country === 'Venezuela' && p.state && VE_STATE_CENTROIDS[p.state]) return VE_STATE_CENTROIDS[p.state]
    if (p.country && COUNTRY_CENTROIDS[p.country]) return COUNTRY_CENTROIDS[p.country]
    return undefined
  }

  const focusProfessional = (p: ProfessionalLocation) => {
    const pos = personPos(p)
    if (!pos) return
    setFocusedId(p.id)
    const stateLevel = p.country === 'Venezuela' && !!p.state && !!VE_STATE_CENTROIDS[p.state]
    const z = stateLevel ? 8 : 6
    if (mapRef.current) mapRef.current.flyTo(pos, Math.max(mapRef.current.getZoom(), z), { duration: 0.8 })
  }

  const markers = useMemo(() => {
    const centroids: Record<string, LatLng> = useStateLevel ? VE_STATE_CENTROIDS : COUNTRY_CENTROIDS
    const groups = new Map<string, ProfessionalLocation[]>()
    for (const p of withLocation) {
      const key = regionKeyOf(p)
      if (!key) continue
      if (!groups.has(key)) groups.set(key, [])
      groups.get(key)!.push(p)
    }
    return Array.from(groups.entries())
      .map(([region, people]) => ({ region, people, pos: centroids[region] }))
      .filter((m) => !!m.pos)
  }, [withLocation, useStateLevel])

  const riskRegions = useMemo(() => {
    if (!showQuakes || quakes.length === 0) return new Set<string>()
    const out = new Set<string>()
    for (const m of markers) {
      for (const q of quakes) {
        if (haversineKm(m.pos!, [q.lat, q.lng]) <= radiusKm) {
          out.add(m.region)
          break
        }
      }
    }
    return out
  }, [markers, quakes, radiusKm, showQuakes])

  const atRiskPeople = useMemo(
    () => markers.filter((m) => riskRegions.has(m.region)).flatMap((m) => m.people),
    [markers, riskRegions],
  )

  const selectedPeople = useMemo(() => {
    if (selectedRegion === null) return withLocation
    return withLocation.filter((p) => regionKeyOf(p) === selectedRegion)
  }, [withLocation, selectedRegion, useStateLevel])

  const stats = useMemo(() => {
    const total = filtered.length
    const located = withLocation.length
    const tally = (key: (p: ProfessionalLocation) => string) => {
      const m = new Map<string, number>()
      for (const p of withLocation) {
        const k = key(p)
        if (!k) continue
        m.set(k, (m.get(k) ?? 0) + 1)
      }
      return Array.from(m.entries())
        .sort((a, b) => b[1] - a[1])
        .slice(0, 3)
    }
    return {
      total,
      located,
      missing: total - located,
      coverage: total ? Math.round((located / total) * 100) : 0,
      topCountries: tally((p) => p.country),
      topStates: tally((p) => p.state),
    }
  }, [filtered, withLocation])

  const checkInProgress = useMemo(() => {
    const people = selectedPeople
    const ok = people.filter((p) => checkIns[p.id] === 'ok').length
    return { ok, total: people.length, pct: people.length ? Math.round((ok / people.length) * 100) : 0 }
  }, [selectedPeople, checkIns])

  const mapCenter: LatLng = useStateLevel ? [6.4238, -66.5897] : [12, -30]
  const mapZoom = useStateLevel ? 6 : 2

  const stateOptions = useMemo(
    () => [{ value: '', label: 'Todos los estados' }, ...getStatesForCountry(country)],
    [country],
  )

  const onCountryChange = (v: string) => {
    setCountry(v)
    setState('')
    setSelectedRegion(null)
  }

  const toggleId = (id: number) =>
    setSelectedIds((prev) => {
      const next = new Set(prev)
      next.has(id) ? next.delete(id) : next.add(id)
      return next
    })

  const visibleIds = useMemo(() => selectedPeople.map((p) => p.id), [selectedPeople])
  const allVisibleSelected =
    visibleIds.length > 0 && visibleIds.every((id) => selectedIds.has(id))

  const toggleAllVisible = () =>
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (allVisibleSelected) visibleIds.forEach((id) => next.delete(id))
      else visibleIds.forEach((id) => next.add(id))
      return next
    })

  const selectedProfessionals = useMemo(
    () => all.filter((p) => selectedIds.has(p.id)),
    [all, selectedIds],
  )

  const setCheckIn = async (userId: number, status: CheckInStatus) => {
    qc.setQueryData<Record<number, CheckInStatus>>(['follow-ups', 'emergencia'], (old) => ({
      ...(old ?? {}),
      [userId]: status,
    }))
    try {
      await adminService.createFollowUp({ user_id: userId, kind: 'emergencia', status })
    } finally {
      qc.invalidateQueries({ queryKey: ['follow-ups', 'emergencia'] })
    }
  }

  const exportCsv = () => {
    const header = ['Nombre', 'Email', 'Teléfono', 'Empresa', 'País', 'Estado', 'Ciudad', 'Activo']
    const rows = filtered.map((p) => [
      p.name,
      p.email,
      p.phone_number,
      p.company,
      p.country,
      p.state,
      p.city,
      p.is_active ? 'Sí' : 'No',
    ])
    const csv = '﻿' + [header, ...rows].map((r) => r.map(csvCell).join(',')).join('\n')
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'profesionales.csv'
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
  }

  const copyPhones = async () => {
    const phones = selectedProfessionals
      .map((p) => (p.phone_number || '').trim())
      .filter(Boolean)
      .join(', ')
    if (!phones) return
    try {
      await navigator.clipboard.writeText(phones)
    } catch {
    }
  }

  const openEmail = (people: ProfessionalLocation[]) => {
    const targets = people.filter((p) => p.email)
    if (targets.length === 0) return
    setContactList(targets)
    setContactOpen(true)
  }

  const contactButtons = (p: ProfessionalLocation) => {
    const wa = whatsappHref(p.phone_number)
    return (
      <div className={styles['contact-actions']}>
        {p.email && (
          <button
            type="button"
            className={styles['icon-btn']}
            onClick={() => { adminService.logContact(p.id, 'email'); openEmail([p]) }}
            title="Enviar email"
          >
            <Mail size={16} />
          </button>
        )}
        {wa && (
          <a
            className={styles['icon-btn']}
            href={wa}
            target="_blank"
            rel="noreferrer"
            onClick={() => adminService.logContact(p.id, 'whatsapp')}
            title="WhatsApp"
            style={{ color: '#25d366' }}
          >
            <MessageCircle size={16} />
          </a>
        )}
        <button
          type="button"
          className={styles['icon-btn']}
          onClick={() => {
            adminService.logContact(p.id, 'chat')
            navigate(`/chat?userId=${p.id}`)
          }}
          title="Chat interno"
          style={{ color: '#7c3aed' }}
        >
          <MessageSquare size={16} />
        </button>
      </div>
    )
  }

  const personRow = (p: ProfessionalLocation) => {
    const status = checkIns[p.id]
    return (
      <div
        key={p.id}
        className={styles['person-row']}
        style={status ? { borderLeft: `4px solid ${CHECKIN_COLOR[status]}` } : undefined}
      >
        <input
          type="checkbox"
          checked={selectedIds.has(p.id)}
          onChange={() => toggleId(p.id)}
          aria-label={`Seleccionar ${p.name}`}
        />
        <Avatar src={p.avatar} name={p.name} size="sm" />
        <div className={styles['person-info']}>
          <span
            className={styles['person-name']}
            onClick={() => focusProfessional(p)}
            title={personPos(p) ? `Ver en el mapa · ${[p.city, p.state, p.country].filter(Boolean).join(', ')}` : 'Sin ubicación'}
            style={{ cursor: personPos(p) ? 'pointer' : 'default' }}
          >
            {p.name}
            {!p.is_active && <span className={styles['inactive-tag']}>Inactivo</span>}
          </span>
          <span
            className={styles['person-meta']}
            onClick={() => focusProfessional(p)}
            style={{ cursor: personPos(p) ? 'pointer' : 'default' }}
          >
            {[p.company, p.city].filter(Boolean).join(' · ') || 'Sin datos'}
          </span>
          <select
            className={styles['checkin-select']}
            value={status ?? 'pendiente'}
            onChange={(e) => setCheckIn(p.id, e.target.value as CheckInStatus)}
            style={{ color: CHECKIN_COLOR[status ?? 'pendiente'] }}
          >
            {CHECKIN_OPTIONS.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
        </div>
        {contactButtons(p)}
      </div>
    )
  }

  return (
    <div className={styles['page']}>
      <div className={styles['header']}>
        <h1>
          <MapPin size={22} /> Mapa de Profesionales
        </h1>
      </div>

      <div className={styles['stats']}>
        <div className={styles['stat']}>
          <span className={styles['stat-num']}>{stats.total}</span>
          <span className={styles['stat-label']}>Profesionales</span>
        </div>
        <div className={styles['stat']}>
          <span className={styles['stat-num']}>{stats.located}</span>
          <span className={styles['stat-label']}>Con ubicación</span>
        </div>
        <div className={styles['stat']}>
          <span className={styles['stat-num']}>{stats.missing}</span>
          <span className={styles['stat-label']}>Sin ubicación</span>
        </div>
        <div className={styles['stat']}>
          <span className={styles['stat-num']}>{stats.coverage}%</span>
          <span className={styles['stat-label']}>Cobertura</span>
        </div>
        <div className={styles['stat-top']}>
          <span className={styles['stat-label']}>Top países</span>
          <span className={styles['stat-chips']}>
            {stats.topCountries.length === 0 && '—'}
            {stats.topCountries.map(([k, n]) => (
              <span key={k} className={styles['chip']}>
                {k} · {n}
              </span>
            ))}
          </span>
        </div>
        {useStateLevel && (
          <div className={styles['stat-top']}>
            <span className={styles['stat-label']}>Top estados</span>
            <span className={styles['stat-chips']}>
              {stats.topStates.length === 0 && '—'}
              {stats.topStates.map(([k, n]) => (
                <span key={k} className={styles['chip']}>
                  {k} · {n}
                </span>
              ))}
            </span>
          </div>
        )}
        <button
          type="button"
          className={styles['csv-btn']}
          style={{ marginLeft: 0 }}
          onClick={() => {
            const params = new URLSearchParams({ create: '1' })
            if (country) params.set('country', country)
            if (state) params.set('state', state)
            navigate(`/admin/incidentes?${params.toString()}`)
          }}
          title="Crear incidente con esta zona"
        >
          <AlertTriangle size={15} /> Crear incidente
        </button>
        <button type="button" className={styles['csv-btn']} onClick={exportCsv}>
          <Download size={15} /> Exportar CSV
        </button>
      </div>

      <div className={styles['filters']}>
        <div className={styles['filter']}>
          <label>País</label>
          <Select
            fullWidth
            value={country}
            onChange={(v) => onCountryChange(String(v))}
            placeholder="Todos los países"
            options={[{ value: '', label: 'Todos los países' }, ...COUNTRY_OPTIONS]}
          />
        </div>
        <div className={styles['filter']}>
          <label>Provincia / Estado</label>
          <Select
            fullWidth
            value={state}
            onChange={(v) => {
              setState(String(v))
              setSelectedRegion(null)
            }}
            placeholder="Todos los estados"
            options={stateOptions}
            disabled={!country || stateOptions.length <= 1}
          />
        </div>
        <div className={styles['filter']}>
          <label>Estado de cuenta</label>
          <Select
            fullWidth
            value={active}
            onChange={(v) => setActive(String(v) as ActiveFilter)}
            options={ACTIVE_OPTIONS}
          />
        </div>
        <div className={styles['filter']}>
          <label>Buscar</label>
          <div className={styles['search-box']}>
            <Search size={16} />
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Nombre o email..."
            />
          </div>
        </div>
      </div>

      <div className={styles['quake-bar']}>
        <label className={styles['toggle']}>
          <input
            type="checkbox"
            checked={showQuakes}
            onChange={(e) => setShowQuakes(e.target.checked)}
          />
          <Activity size={15} /> Mostrar sismos (USGS)
        </label>
        {showQuakes && (
          <>
            <select
              className={styles['quake-select']}
              value={quakeFeed}
              onChange={(e) => setQuakeFeed(e.target.value)}
            >
              {USGS_FEEDS.map((f) => (
                <option key={f.value} value={f.value}>
                  {f.label}
                </option>
              ))}
            </select>
            <span className={styles['radius-control']}>
              Radio: <strong>{radiusKm} km</strong>
              <input
                type="range"
                min={50}
                max={500}
                step={10}
                value={radiusKm}
                onChange={(e) => setRadiusKm(Number(e.target.value))}
              />
            </span>
            <span className={styles['quake-info']}>
              {quakesQuery.isLoading
                ? 'Cargando sismos…'
                : `${quakes.length} sismos · ${riskRegions.size} regiones en riesgo`}
            </span>
          </>
        )}
      </div>

      {selectedIds.size > 0 && (
        <div className={styles['select-toolbar']}>
          <span>{selectedIds.size} seleccionado{selectedIds.size === 1 ? '' : 's'}</span>
          <button type="button" className={styles['toolbar-btn']} onClick={() => openEmail(selectedProfessionals)}>
            <Send size={14} /> Contactar seleccionados
          </button>
          <button type="button" className={styles['toolbar-btn']} onClick={copyPhones}>
            <Copy size={14} /> Copiar teléfonos
          </button>
          <button
            type="button"
            className={styles['toolbar-btn-ghost']}
            onClick={() => setSelectedIds(new Set())}
          >
            Limpiar
          </button>
        </div>
      )}

      <div className={styles['content']}>
        <div className={styles['map-wrap']}>
          <MapContainer ref={mapRef} center={mapCenter} zoom={mapZoom} className={styles['map']} scrollWheelZoom>
            <TileLayer
              attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
              url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
            />
            {markers.map((m) => (
              <Marker
                key={m.region}
                position={m.pos!}
                icon={countBadgeIcon(m.people.length, riskRegions.has(m.region))}
                eventHandlers={{ click: () => focusRegion(m.region, m.pos!) }}
              >
                <Popup>
                  <strong>{m.region}</strong>
                  <br />
                  {m.people.length} profesional{m.people.length === 1 ? '' : 'es'}
                  {riskRegions.has(m.region) && (
                    <>
                      <br />
                      <span style={{ color: '#dc2626', fontWeight: 700 }}>⚠ En zona de riesgo</span>
                    </>
                  )}
                </Popup>
              </Marker>
            ))}
            {(() => {
              const fp = focusedId != null ? all.find((p) => p.id === focusedId) : undefined
              const fpos = fp ? personPos(fp) : undefined
              return fp && fpos ? (
                <Marker position={fpos} icon={personPinIcon(fp.name)} zIndexOffset={1000}>
                  <Popup>
                    <strong>{fp.name}</strong>
                    <br />
                    {[fp.company, fp.city, fp.state, fp.country].filter(Boolean).join(' · ') || 'Sin datos'}
                  </Popup>
                </Marker>
              ) : null
            })()}
            {showQuakes &&
              quakes.map((q) =>
                Number.isFinite(q.lat) && Number.isFinite(q.lng) ? (
                  <div key={q.id}>
                    <Circle
                      center={[q.lat, q.lng]}
                      radius={radiusKm * 1000}
                      pathOptions={{ color: '#dc2626', fillColor: '#dc2626', fillOpacity: 0.08, weight: 1 }}
                    />
                    <Marker position={[q.lat, q.lng]} icon={quakeIcon(q.mag)}>
                      <Popup>
                        <strong>Magnitud {q.mag}</strong>
                        <br />
                        {q.place}
                        <br />
                        {new Date(q.time).toLocaleString()}
                      </Popup>
                    </Marker>
                  </div>
                ) : null,
              )}
          </MapContainer>
        </div>

        <aside className={styles['panel']}>
          {isLoading && <div className={styles['state-msg']}>Cargando…</div>}
          {isError && <div className={styles['state-msg']}>Error al cargar los datos.</div>}

          {!isLoading && !isError && (
            <>
              <div className={styles['panel-head']}>
                <h3>{selectedRegion ?? 'Todas las regiones'}</h3>
                {selectedRegion && (
                  <button
                    type="button"
                    className={styles['clear-btn']}
                    onClick={() => setSelectedRegion(null)}
                  >
                    Ver todo
                  </button>
                )}
                <span className={styles['count']}>{selectedPeople.length}</span>
              </div>

              <div className={styles['progress-wrap']}>
                <div className={styles['progress-label']}>
                  {checkInProgress.ok}/{checkInProgress.total} confirmados OK
                </div>
                <div className={styles['progress-bar']}>
                  <div
                    className={styles['progress-fill']}
                    style={{ width: `${checkInProgress.pct}%` }}
                  />
                </div>
              </div>

              <div className={styles['list-tools']}>
                <label className={styles['select-all']}>
                  <input
                    type="checkbox"
                    checked={allVisibleSelected}
                    onChange={toggleAllVisible}
                    disabled={visibleIds.length === 0}
                  />
                  Seleccionar todos
                </label>
              </div>

              <div className={styles['list']}>
                {selectedPeople.length === 0 && (
                  <div className={styles['state-msg']}>Sin profesionales en esta selección.</div>
                )}
                {selectedPeople.map(personRow)}
              </div>

              {showQuakes && atRiskPeople.length > 0 && (
                <div className={styles['risk-zone']}>
                  <h4>
                    <AlertTriangle size={15} /> Profesionales en zona de riesgo ({atRiskPeople.length})
                  </h4>
                  <div className={styles['list']}>{atRiskPeople.map(personRow)}</div>
                </div>
              )}

              {noLocation.length > 0 && (
                <div className={styles['no-location']}>
                  <h4>Sin ubicación ({noLocation.length})</h4>
                  <div className={styles['list']}>{noLocation.map(personRow)}</div>
                </div>
              )}
            </>
          )}
        </aside>
      </div>

      {contactOpen && (
        <ContactModal
          professionals={contactList}
          onClose={() => setContactOpen(false)}
        />
      )}
    </div>
  )
}

function ContactModal({
  professionals,
  onClose,
}: {
  professionals: ProfessionalLocation[]
  onClose: () => void
}) {
  const { data: tplData } = useQuery({
    queryKey: ['emergency-templates'],
    queryFn: adminService.getEmergencyTemplates,
  })
  const templateOptions = useMemo(() => buildTemplateOptions(tplData?.templates ?? []), [tplData])

  const [templateValue, setTemplateValue] = useState(templateOptions[0]?.value ?? '')
  const [subject, setSubject] = useState(templateOptions[0]?.subject ?? '')
  const [body, setBody] = useState(templateOptions[0]?.body ?? '')
  const [manageOpen, setManageOpen] = useState(false)
  const [sending, setSending] = useState(false)
  const [result, setResult] = useState<{ sent: number; failed: { id: number; email: string; error: string }[] } | null>(null)

  const applyTemplate = (value: string) => {
    setTemplateValue(value)
    const opt = templateOptions.find((o) => o.value === value)
    if (opt) {
      setSubject(opt.subject)
      setBody(opt.body)
    }
  }

  const send = async () => {
    setSending(true)
    try {
      const res = await adminService.bulkEmailProfessionals({
        user_ids: professionals.map((p) => p.id),
        subject,
        body,
      })
      setResult(res)
    } catch {
      setResult({ sent: 0, failed: professionals.map((p) => ({ id: p.id, email: p.email, error: 'Error de red' })) })
    } finally {
      setSending(false)
    }
  }

  return (
    <>
    <div className={styles['modal-overlay']} onClick={onClose}>
      <div className={styles['modal']} onClick={(e) => e.stopPropagation()}>
        <div className={styles['modal-head']}>
          <h3>Contactar {professionals.length} profesional{professionals.length === 1 ? '' : 'es'}</h3>
          <button type="button" className={styles['icon-btn']} onClick={onClose}>
            <X size={18} />
          </button>
        </div>

        {result ? (
          <div className={styles['modal-body']}>
            <p className={styles['result-ok']}>Enviados: {result.sent}</p>
            {result.failed.length > 0 && (
              <div className={styles['result-fail']}>
                <strong>Fallidos ({result.failed.length}):</strong>
                <ul>
                  {result.failed.map((f) => (
                    <li key={f.id}>
                      {f.email} — {f.error}
                    </li>
                  ))}
                </ul>
              </div>
            )}
            <div className={styles['modal-actions']}>
              <button type="button" className={styles['toolbar-btn']} onClick={onClose}>
                Cerrar
              </button>
            </div>
          </div>
        ) : (
          <div className={styles['modal-body']}>
            <div className={styles['field-row']}>
              <label className={styles['field-label']}>Plantilla</label>
              <button type="button" className={styles['link-btn']} onClick={() => setManageOpen(true)}>
                Gestionar plantillas
              </button>
            </div>
            <select
              className={styles['quake-select']}
              value={templateValue}
              onChange={(e) => applyTemplate(e.target.value)}
            >
              {templateOptions.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>

            <label className={styles['field-label']}>Asunto</label>
            <input
              className={styles['field-input']}
              value={subject}
              onChange={(e) => setSubject(e.target.value)}
            />

            <label className={styles['field-label']}>Mensaje</label>
            <textarea
              className={styles['field-textarea']}
              rows={6}
              value={body}
              onChange={(e) => setBody(e.target.value)}
            />

            <div className={styles['modal-actions']}>
              <button type="button" className={styles['toolbar-btn-ghost']} onClick={onClose}>
                Cancelar
              </button>
              <button
                type="button"
                className={styles['toolbar-btn']}
                onClick={send}
                disabled={sending || !subject.trim() || professionals.length === 0}
              >
                <Send size={14} /> {sending ? 'Enviando…' : 'Enviar'}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
    <EmergencyTemplatesModal isOpen={manageOpen} onClose={() => setManageOpen(false)} />
    </>
  )
}
