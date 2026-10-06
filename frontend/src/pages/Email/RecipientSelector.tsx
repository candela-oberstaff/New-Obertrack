import { useState, useEffect, useMemo, useRef } from 'react'
import { createPortal } from 'react-dom'
import { Search, X, Users, UserCheck, Check, MapPin, Building2, AlertTriangle, Filter, SlidersHorizontal, ChevronDown } from 'lucide-react'
import { emailService } from '../../services/emailService'
import { audienceService, AudienceGroup } from '../../services/audienceService'
import {
  buildCompanyIndex,
  companyNameOf,
  companyOptions,
  matchesCompany,
} from '../../lib/recipientCompany'

export interface RecipientValue {
  userIds: number[]
  groupIds: number[]
  expressContacts: Array<{ name: string; email: string }>
}

interface User {
  id: number
  name: string
  email: string
  user_type: string
  is_manager: boolean
  is_supervisor: boolean
  is_superadmin: boolean
  is_active?: boolean
  country?: string
  // La empresa de un profesional no está en su fila: solo trae el id de su
  // empleador, y el nombre vive en la cuenta empleador. Ver lib/recipientCompany.
  company_name?: string
  empleador_id?: number
}

// Valor del filtro de país cuando se quiere ver a quienes NO tienen país
// cargado. No es un capricho: al filtrar por "Venezuela" esa gente desaparece
// del listado sin aviso, y en un envío masivo eso es quedarse fuera en silencio.
// Con esta opción se los puede encontrar (y arreglar su ficha) o incluirlos a
// propósito.
const NO_COUNTRY = '__sin_pais__'

// Todos los roles disponibles para filtrar
const ROLE_FILTERS = [
  { value: 'all',                  label: 'Todos',                  color: '#7c3aed', bg: '#f5f3ff' },
  { value: 'profesional_activo',   label: 'Profesionales activos',   color: '#0369a1', bg: '#e0f2fe' },
  { value: 'profesional_inactivo', label: 'Profesionales inactivos', color: '#475569', bg: '#f1f5f9' },
  { value: 'empleador',            label: 'Empresas',               color: '#0f766e', bg: '#ccfbf1' },
  { value: 'customer_success',     label: 'Customer Success',       color: '#b45309', bg: '#fef3c7' },
  { value: 'it_analyst',           label: 'Analista de IT',         color: '#7c3aed', bg: '#ede9fe' },
  { value: 'manager',              label: 'Manager',                color: '#3730a3', bg: '#e0e7ff' },
  { value: 'supervisor',           label: 'Supervisor',             color: '#be185d', bg: '#fce7f3' },
  { value: 'superadmin',           label: 'Superadmin',             color: '#dc2626', bg: '#fee2e2' },
]

interface Props {
  value: RecipientValue
  onChange: (v: RecipientValue) => void
}

type Tab = 'users' | 'groups'

// ─── Multi-select dropdown con buscador y checkboxes ─────────────────────────
interface MSOption { value: string; label: string }
interface MultiSelectFilterProps {
  options: MSOption[]
  selected: string[]
  onChange: (v: string[]) => void
  placeholder: string
  icon?: React.ReactNode
}
function MultiSelectFilter({ options, selected, onChange, placeholder, icon }: MultiSelectFilterProps) {
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const triggerRef = useRef<HTMLButtonElement>(null)
  const dropRef = useRef<HTMLDivElement>(null)
  const [dropPos, setDropPos] = useState({ top: 0, left: 0, width: 0 })

  // Cierra al hacer clic fuera
  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (
        !triggerRef.current?.contains(e.target as Node) &&
        !dropRef.current?.contains(e.target as Node)
      ) setOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open])

  const openDropdown = () => {
    if (triggerRef.current) {
      const rect = triggerRef.current.getBoundingClientRect()
      setDropPos({ top: rect.bottom + 4, left: rect.left, width: Math.max(rect.width, 230) })
    }
    setOpen(v => !v)
  }

  const filtered = options.filter(o => o.label.toLowerCase().includes(search.toLowerCase()))

  const toggle = (val: string) => {
    if (selected.includes(val)) onChange(selected.filter(v => v !== val))
    else onChange([...selected, val])
  }

  const selectAllFiltered = () => {
    const vals = filtered.map(o => o.value)
    onChange(Array.from(new Set([...selected, ...vals])))
  }

  const clearAll = () => onChange([])

  const label = selected.length === 0
    ? placeholder
    : `${selected.length} seleccionado${selected.length > 1 ? 's' : ''}`

  return (
    <>
      <button
        ref={triggerRef}
        onClick={openDropdown}
        style={{
          width: '100%', display: 'flex', alignItems: 'center', gap: 6,
          padding: '6px 9px', borderRadius: 8,
          border: open ? '1.5px solid #7c3aed' : '1px solid #e2e8f0',
          background: '#fff', cursor: 'pointer',
          fontSize: 11, color: selected.length > 0 ? '#7c3aed' : '#64748b',
          fontWeight: selected.length > 0 ? 700 : 400,
          transition: 'all 0.12s',
        }}
      >
        <span style={{ color: '#94a3b8', flexShrink: 0 }}>{icon}</span>
        <span style={{ flex: 1, textAlign: 'left', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{label}</span>
        <ChevronDown size={11} style={{ color: '#94a3b8', flexShrink: 0, transform: open ? 'rotate(180deg)' : 'none', transition: 'transform 0.15s' }} />
      </button>

      {open && createPortal(
        <div
          ref={dropRef}
          style={{
            position: 'fixed',
            top: dropPos.top,
            left: dropPos.left,
            width: dropPos.width,
            zIndex: 99999,
            background: '#fff',
            border: '1px solid #e2e8f0',
            borderRadius: 10,
            boxShadow: '0 8px 28px rgba(0,0,0,0.13)',
            overflow: 'hidden',
          }}
        >
          {/* Buscador */}
          <div style={{ padding: '8px 10px', borderBottom: '1px solid #f1f5f9', display: 'flex', alignItems: 'center', gap: 6 }}>
            <Search size={12} style={{ color: '#94a3b8', flexShrink: 0 }} />
            <input
              autoFocus
              value={search}
              onChange={e => setSearch(e.target.value)}
              placeholder="Buscar..."
              style={{ border: 'none', outline: 'none', fontSize: 12, flex: 1, color: '#1e293b', background: 'transparent' }}
            />
            {search && <X size={11} style={{ color: '#94a3b8', cursor: 'pointer' }} onClick={() => setSearch('')} />}
          </div>
          {/* Acciones bulk */}
          <div style={{ display: 'flex', gap: 0, borderBottom: '1px solid #f1f5f9' }}>
            <button
              onClick={selectAllFiltered}
              style={{ flex: 1, padding: '5px 10px', fontSize: 10, fontWeight: 700, color: '#7c3aed', background: 'none', border: 'none', cursor: 'pointer', borderRight: '1px solid #f1f5f9' }}
            >
              Seleccionar todos
            </button>
            <button
              onClick={clearAll}
              style={{ flex: 1, padding: '5px 10px', fontSize: 10, fontWeight: 600, color: '#94a3b8', background: 'none', border: 'none', cursor: 'pointer' }}
            >
              Limpiar
            </button>
          </div>
          {/* Lista con checkboxes */}
          <div style={{ maxHeight: 200, overflowY: 'auto' }}>
            {filtered.length === 0 ? (
              <div style={{ padding: '12px 10px', textAlign: 'center', fontSize: 11, color: '#94a3b8' }}>Sin resultados</div>
            ) : filtered.map(o => {
              const isChecked = selected.includes(o.value)
              return (
                <label
                  key={o.value}
                  onClick={() => toggle(o.value)}
                  style={{
                    display: 'flex', alignItems: 'center', gap: 8,
                    padding: '6px 10px', cursor: 'pointer',
                    background: isChecked ? '#faf5ff' : 'transparent',
                    transition: 'background 0.1s',
                  }}
                  onMouseEnter={e => { if (!isChecked) (e.currentTarget as HTMLLabelElement).style.background = '#f8fafc' }}
                  onMouseLeave={e => { (e.currentTarget as HTMLLabelElement).style.background = isChecked ? '#faf5ff' : 'transparent' }}
                >
                  <div style={{
                    width: 14, height: 14, borderRadius: 3, flexShrink: 0,
                    border: isChecked ? '2px solid #7c3aed' : '1.5px solid #cbd5e1',
                    background: isChecked ? '#7c3aed' : '#fff',
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                    transition: 'all 0.1s',
                  }}>
                    {isChecked && <Check size={8} strokeWidth={3} color="#fff" />}
                  </div>
                  <span style={{ fontSize: 12, color: '#1e293b', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{o.label}</span>
                </label>
              )
            })}
          </div>
        </div>,
        document.body
      )}
    </>
  )
}

export default function RecipientSelector({ value, onChange }: Props) {
  const [tab, setTab] = useState<Tab>('users')
  const [query, setQuery] = useState('')
  const [roleFilter, setRoleFilter] = useState<string>('all')
  const [countryFilters, setCountryFilters] = useState<string[]>([])
  const [companyFilters, setCompanyFilters] = useState<string[]>([])
  const [allUsers, setAllUsers] = useState<User[]>([])
  const [groups, setGroups] = useState<AudienceGroup[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    setLoading(true)
    Promise.all([
      emailService.getAvailableRecipients().catch(() => ({ data: [] })),
      audienceService.getGroups().catch(() => []),
    ]).then(([usersResp, grps]) => {
      const users: User[] = Array.isArray(usersResp) ? usersResp : (usersResp?.data ?? usersResp?.users ?? [])
      setAllUsers(users)
      setGroups(grps)
    }).finally(() => setLoading(false))
  }, [])

  // El filtro de rol se separó del resto para poder contar cuánta gente hay por
  // país DENTRO del rol elegido: "Profesionales → Venezuela (312)" es el dato
  // que hace falta antes de lanzar un envío masivo.
  const usersByRole = useMemo(() => {
    return allUsers.filter(u => {
      if (roleFilter === 'superadmin')       return u.is_superadmin || u.user_type === 'superadmin'
      if (roleFilter === 'customer_success') return u.user_type === 'customer_success'
      if (roleFilter === 'empleador')        return u.user_type === 'empleador'
      if (roleFilter === 'manager')          return u.is_manager === true && !u.is_supervisor
      if (roleFilter === 'supervisor')       return u.is_supervisor === true
      if (roleFilter === 'it_analyst')       return u.user_type === 'it_analyst' || u.user_type === 'analista_it'
      if (roleFilter === 'profesional_activo' || roleFilter === 'profesional') {
        // Profesionales activos: user_type='profesional' pero NO managers ni supervisores, y no inactivos (is_active !== false)
        return u.user_type === 'profesional' && !u.is_manager && !u.is_supervisor && u.is_active !== false
      }
      if (roleFilter === 'profesional_inactivo') {
        // Profesionales inactivos: user_type='profesional' pero NO managers ni supervisores, e is_active === false
        return u.user_type === 'profesional' && !u.is_manager && !u.is_supervisor && u.is_active === false
      }
      return true
    })
  }, [allUsers, roleFilter])

  // Los países salen de la gente que ya hay según el rol, ordenados por cantidad.
  const countryOptions = useMemo(() => {
    const counts = new Map<string, number>()
    let sinPais = 0
    for (const u of usersByRole) {
      const c = (u.country ?? '').trim()
      if (c) counts.set(c, (counts.get(c) ?? 0) + 1)
      else sinPais++
    }
    const options = Array.from(counts.entries())
      .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0], 'es'))
      .map(([name, count]) => ({ value: name, label: `${name} (${count})` }))
    if (sinPais > 0) {
      options.push({ value: NO_COUNTRY, label: `Sin país registrado (${sinPais})` })
    }
    return options
  }, [usersByRole])

  // El índice se arma con TODA la gente, no con usersByRole: los nombres de
  // empresa salen de las cuentas empleador, y filtrando por "Profesionales"
  // esas cuentas ya no están en la lista.
  const companyIndex = useMemo(() => buildCompanyIndex(allUsers), [allUsers])

  // Opciones de empresa para el filtro (todas, sin depender de selección actual)
  const companySelectOptions = useMemo(
    () => companyOptions(usersByRole, companyIndex, 'all'),
    [usersByRole, companyIndex],
  )

  const filteredUsers = useMemo(() => {
    const q = query.toLowerCase()
    return usersByRole.filter(u => {
      // La búsqueda mira también la empresa
      const matchQuery =
        u.name?.toLowerCase().includes(q) ||
        u.email?.toLowerCase().includes(q) ||
        companyNameOf(u, companyIndex).toLowerCase().includes(q)

      if (!matchQuery) return false

      // Filtro multi-empresa: si hay selección, el usuario debe pertenecer a alguna
      if (companyFilters.length > 0) {
        const matchCompany = companyFilters.some(cf => matchesCompany(u, companyIndex, cf))
        if (!matchCompany) return false
      }

      // Filtro multi-país
      if (countryFilters.length > 0) {
        const country = (u.country ?? '').trim()
        const matchCountry = countryFilters.some(cf =>
          cf === NO_COUNTRY ? country === '' : country === cf
        )
        if (!matchCountry) return false
      }

      return true
    })
  }, [usersByRole, query, countryFilters, companyFilters, companyIndex])

  const toggleUser = (id: number) => {
    const ids = value.userIds.includes(id)
      ? value.userIds.filter(x => x !== id)
      : [...value.userIds, id]
    onChange({ ...value, userIds: ids })
  }

  const selectAllFiltered = () => {
    const visibleIds = filteredUsers.map(u => u.id)
    const newIds = Array.from(new Set([...value.userIds, ...visibleIds]))
    onChange({ ...value, userIds: newIds })
  }

  const deselectAllFiltered = () => {
    const visibleIds = filteredUsers.map(u => u.id)
    const newIds = value.userIds.filter(id => !visibleIds.includes(id))
    onChange({ ...value, userIds: newIds })
  }

  const toggleGroup = (id: number) => {
    const ids = (value.groupIds ?? []).includes(id)
      ? (value.groupIds ?? []).filter(x => x !== id)
      : [...(value.groupIds ?? []), id]
    onChange({ ...value, groupIds: ids })
  }

  const totalSelected = value.userIds.length + (value.groupIds?.length ?? 0) + (value.expressContacts?.length ?? 0)

  // A qué empresas le va a llegar el envío, y quién está elegido pero oculto
  // por el filtro actual.
  //
  // "Seleccionar todos" suma a lo ya marcado, así que acotar a un cliente,
  // marcar, y cambiar de cliente deja a los dos dentro sin que nada lo diga:
  // los primeros desaparecen de la lista pero no del envío. En una campaña eso
  // son correos reales a gente que no tocaba.
  const selectionByCompany = useMemo(() => {
    const chosen = new Set(value.userIds)
    const counts = new Map<string, number>()
    for (const u of allUsers) {
      if (!chosen.has(u.id)) continue
      const name = companyNameOf(u, companyIndex) || 'Sin empresa'
      counts.set(name, (counts.get(name) ?? 0) + 1)
    }
    return Array.from(counts.entries()).sort((a, b) => b[1] - a[1])
  }, [value.userIds, allUsers, companyIndex])

  const hiddenSelected = useMemo(() => {
    const visible = new Set(filteredUsers.map(u => u.id))
    return value.userIds.filter(id => !visible.has(id))
  }, [value.userIds, filteredUsers])

  const dropHiddenSelected = () => {
    const visible = new Set(filteredUsers.map(u => u.id))
    onChange({ ...value, userIds: value.userIds.filter(id => visible.has(id)) })
  }

  return (
    <div className="border border-slate-200 rounded-xl overflow-hidden bg-white shadow-sm">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 bg-slate-50 border-b border-slate-200">
        <div className="flex items-center gap-2">
          <Users size={15} className="text-purple-600" />
          <span className="text-sm font-semibold text-slate-700">Seleccionar destinatarios</span>
        </div>
        {totalSelected > 0 && (
          <span className="bg-purple-600 text-white text-xs font-bold rounded-full px-2.5 py-1">
            {totalSelected} seleccionado{totalSelected !== 1 ? 's' : ''}
          </span>
        )}
      </div>

      {/* Tabs */}
      <div className="flex border-b border-slate-200 bg-white">
        {(['users', 'groups'] as Tab[]).map(t => {
          const isActive = tab === t
          return (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={`flex-1 py-3 text-xs font-semibold border-b-2 transition-all flex items-center justify-center gap-1.5 ${
                isActive ? 'border-purple-600 text-purple-600 bg-purple-50/30' : 'border-transparent text-slate-500 hover:text-slate-700'
              }`}
            >
              {t === 'users' && <Users size={14} />}
              {t === 'groups' && <UserCheck size={14} />}
              {t === 'users' ? 'Usuarios' : 'Grupos'}
            </button>
          )
        })}
      </div>

      {/* Users tab: two-column layout */}
      {tab === 'users' && (
        <div style={{ display: 'flex', minHeight: 380, maxHeight: 440 }}>

          {/* ── LEFT: Filters panel ── */}
          <div style={{
            width: 250, minWidth: 250, borderRight: '1px solid #e2e8f0',
            background: '#f8fafc', display: 'flex', flexDirection: 'column', overflowY: 'auto',
          }}>
            <div style={{ padding: '10px 14px 8px', display: 'flex', alignItems: 'center', gap: 6, borderBottom: '1px solid #e2e8f0' }}>
              <SlidersHorizontal size={12} style={{ color: '#7c3aed' }} />
              <span style={{ fontSize: 10, fontWeight: 800, color: '#64748b', textTransform: 'uppercase', letterSpacing: '0.07em' }}>Filtros</span>
            </div>

            <div style={{ padding: '10px', display: 'flex', flexDirection: 'column', gap: 14, flex: 1 }}>
              {/* Roles */}
              <div>
                <div style={{ fontSize: 9, fontWeight: 800, color: '#94a3b8', textTransform: 'uppercase', letterSpacing: '0.07em', marginBottom: 5, display: 'flex', alignItems: 'center', gap: 3 }}>
                  <Filter size={9} /> Rol
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
                  {ROLE_FILTERS.map(r => {
                    const isRoleActive = roleFilter === r.value
                    return (
                      <button
                        key={r.value}
                        onClick={() => setRoleFilter(r.value)}
                        style={{
                          padding: '5px 9px', borderRadius: 7, border: 'none',
                          background: isRoleActive ? r.bg : 'transparent',
                          color: isRoleActive ? r.color : '#475569',
                          fontSize: 12, fontWeight: isRoleActive ? 700 : 500,
                          cursor: 'pointer', textAlign: 'left',
                          transition: 'all 0.12s',
                          outline: isRoleActive ? `1.5px solid ${r.color}33` : 'none',
                        }}
                      >
                        {r.label}
                      </button>
                    )
                  })}
                </div>
              </div>

              {/* Empresa — multi-select con buscador */}
              <div>
                <div style={{ fontSize: 9, fontWeight: 800, color: '#94a3b8', textTransform: 'uppercase', letterSpacing: '0.07em', marginBottom: 6, display: 'flex', alignItems: 'center', gap: 3 }}>
                  <Building2 size={9} /> Empresa
                </div>
                <MultiSelectFilter
                  options={companySelectOptions}
                  selected={companyFilters}
                  onChange={setCompanyFilters}
                  placeholder="Todas las empresas"
                  icon={<Building2 size={12} />}
                />
                {companyFilters.length > 0 && (
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: 3, marginTop: 5 }}>
                    {companyFilters.map(cf => (
                      <span key={cf} style={{ fontSize: 10, background: '#ede9fe', color: '#6d28d9', borderRadius: 4, padding: '1px 6px', display: 'flex', alignItems: 'center', gap: 3 }}>
                        {companySelectOptions.find(o => o.value === cf)?.label?.split(' (')[0] ?? cf}
                        <X size={9} style={{ cursor: 'pointer' }} onClick={() => setCompanyFilters(f => f.filter(v => v !== cf))} />
                      </span>
                    ))}
                  </div>
                )}
              </div>

              {/* País — multi-select con buscador */}
              <div>
                <div style={{ fontSize: 9, fontWeight: 800, color: '#94a3b8', textTransform: 'uppercase', letterSpacing: '0.07em', marginBottom: 6, display: 'flex', alignItems: 'center', gap: 3 }}>
                  <MapPin size={9} /> País
                </div>
                <MultiSelectFilter
                  options={countryOptions}
                  selected={countryFilters}
                  onChange={setCountryFilters}
                  placeholder="Todos los países"
                  icon={<MapPin size={12} />}
                />
                {countryFilters.length > 0 && (
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: 3, marginTop: 5 }}>
                    {countryFilters.map(cf => (
                      <span key={cf} style={{ fontSize: 10, background: '#e0f2fe', color: '#0369a1', borderRadius: 4, padding: '1px 6px', display: 'flex', alignItems: 'center', gap: 3 }}>
                        {cf === '__sin_pais__' ? 'Sin país' : cf}
                        <X size={9} style={{ cursor: 'pointer' }} onClick={() => setCountryFilters(f => f.filter(v => v !== cf))} />
                      </span>
                    ))}
                  </div>
                )}
              </div>

              {/* Resumen de empresas en la selección actual */}
              {selectionByCompany.length > 0 && (
                <div>
                  <div style={{ fontSize: 9, fontWeight: 800, color: '#94a3b8', textTransform: 'uppercase', letterSpacing: '0.07em', marginBottom: 5 }}>
                    En selección
                  </div>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                    {selectionByCompany.map(([name, count]) => (
                      <div key={name} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', fontSize: 10, color: '#5b21b6', background: '#f5f3ff', borderRadius: 5, padding: '3px 7px' }}>
                        <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 120 }}>{name}</span>
                        <strong style={{ flexShrink: 0, marginLeft: 4 }}>{count}</strong>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </div>

          {/* ── RIGHT: User list ── */}
          <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden', minWidth: 0 }}>
            {/* Search bar + acciones bulk */}
            <div style={{ padding: '10px 12px', borderBottom: '1px solid #f1f5f9', background: '#fff', flexShrink: 0 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 7, background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: 8, padding: '6px 11px', marginBottom: 7 }}>
                <Search size={13} style={{ color: '#94a3b8', flexShrink: 0 }} />
                <input
                  value={query}
                  onChange={e => setQuery(e.target.value)}
                  placeholder="Buscar por nombre o email..."
                  style={{ border: 'none', background: 'transparent', outline: 'none', fontSize: 12, flex: 1, color: '#1e293b' }}
                />
                {query && <X size={13} style={{ color: '#94a3b8', cursor: 'pointer' }} onClick={() => setQuery('')} />}
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 5 }}>
                  {roleFilter !== 'all' && (() => {
                    const r = ROLE_FILTERS.find(x => x.value === roleFilter)!
                    return <span style={{ fontSize: 10, fontWeight: 700, color: r.color, background: r.bg, borderRadius: 999, padding: '2px 8px' }}>{r.label}</span>
                  })()}
                  <span style={{ fontSize: 11, color: '#94a3b8', fontWeight: 600 }}>{filteredUsers.length} encontrados</span>
                </div>
                <div style={{ display: 'flex', gap: 10 }}>
                  <button onClick={selectAllFiltered} style={{ fontSize: 11, fontWeight: 700, color: '#7c3aed', background: 'none', border: 'none', cursor: 'pointer', padding: 0 }}>Seleccionar todos</button>
                  <button onClick={deselectAllFiltered} style={{ fontSize: 11, fontWeight: 600, color: '#94a3b8', background: 'none', border: 'none', cursor: 'pointer', padding: 0 }}>Limpiar visibles</button>
                </div>
              </div>

              {/* Aviso: destinatarios ocultos por filtro pero incluidos en selección.
                  Ver comentario en hiddenSelected arriba para entender el riesgo. */}
              {hiddenSelected.length > 0 && (
                <div style={{ display: 'flex', alignItems: 'center', gap: 7, marginTop: 8, padding: '6px 10px', borderRadius: 8, background: '#fffbeb', border: '1px solid #fcd34d', fontSize: 11, color: '#92400e' }}>
                  <AlertTriangle size={13} style={{ flexShrink: 0 }} />
                  <span style={{ flex: 1 }}>
                    {hiddenSelected.length} destinatario{hiddenSelected.length === 1 ? '' : 's'} seleccionado{hiddenSelected.length === 1 ? '' : 's'} oculto{hiddenSelected.length === 1 ? '' : 's'} por el filtro. Se enviará igual.
                  </span>
                  <button onClick={dropHiddenSelected} style={{ flexShrink: 0, fontSize: 10, fontWeight: 700, color: '#92400e', background: '#fff', border: '1px solid #fcd34d', borderRadius: 5, padding: '2px 8px', cursor: 'pointer' }}>
                    Quitarlos
                  </button>
                </div>
              )}
            </div>

            {/* Lista de usuarios con scroll */}
            <div style={{ flex: 1, overflowY: 'auto' }}>
              {loading ? (
                <div style={{ padding: 40, textAlign: 'center', fontSize: 12, color: '#94a3b8' }}>Cargando destinatarios...</div>
              ) : filteredUsers.length === 0 ? (
                <div style={{ padding: 40, textAlign: 'center', fontSize: 12, color: '#94a3b8' }}>Sin resultados para el filtro actual</div>
              ) : (
                filteredUsers.map(u => {
                  const isSelected = value.userIds.includes(u.id)
                  const badge: { label: string; color: string; bg: string } | null = (() => {
                    if (u.is_superadmin || u.user_type === 'superadmin') return { label: 'Superadmin', color: '#dc2626', bg: '#fee2e2' }
                    if (u.is_supervisor) return { label: 'Supervisor', color: '#ea580c', bg: '#fff7ed' }
                    if (u.is_manager) return { label: 'Manager', color: '#3730a3', bg: '#e0e7ff' }
                    if (u.user_type === 'customer_success') return { label: 'CS', color: '#b45309', bg: '#fef3c7' }
                    if (u.user_type === 'it_analyst' || u.user_type === 'analista_it') return { label: 'IT', color: '#7c3aed', bg: '#ede9fe' }
                    if (u.user_type === 'empleador') return { label: 'Empresa', color: '#0f766e', bg: '#ccfbf1' }
                    if (u.is_active === false) return { label: 'Inactivo', color: '#64748b', bg: '#f1f5f9' }
                    return null
                  })()
                  const empresa = companyNameOf(u, companyIndex)
                  return (
                    <div
                      key={u.id}
                      onClick={() => toggleUser(u.id)}
                      style={{
                        display: 'flex', alignItems: 'center', gap: 10,
                        padding: '8px 14px', cursor: 'pointer',
                        borderBottom: '1px solid #f8fafc',
                        background: isSelected ? '#faf5ff' : 'transparent',
                        transition: 'background 0.1s',
                      }}
                      onMouseEnter={e => { if (!isSelected) (e.currentTarget as HTMLDivElement).style.background = '#f8fafc' }}
                      onMouseLeave={e => { (e.currentTarget as HTMLDivElement).style.background = isSelected ? '#faf5ff' : 'transparent' }}
                    >
                      <div style={{
                        width: 16, height: 16, borderRadius: 4, flexShrink: 0,
                        border: isSelected ? '2px solid #7c3aed' : '1.5px solid #cbd5e1',
                        background: isSelected ? '#7c3aed' : '#fff',
                        display: 'flex', alignItems: 'center', justifyContent: 'center', transition: 'all 0.12s',
                      }}>
                        {isSelected && <Check size={9} strokeWidth={3} color="#fff" />}
                      </div>
                      <div style={{ flex: 1, minWidth: 0 }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 5, flexWrap: 'wrap' }}>
                          <span style={{ fontSize: 12, fontWeight: 600, color: '#1e293b', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', maxWidth: 180 }}>
                            {u.name}
                          </span>
                          {badge && (
                            <span style={{ fontSize: 9, fontWeight: 800, textTransform: 'uppercase', padding: '1px 5px', borderRadius: 4, color: badge.color, background: badge.bg, letterSpacing: '0.04em', flexShrink: 0 }}>
                              {badge.label}
                            </span>
                          )}
                        </div>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginTop: 1 }}>
                          <span style={{ fontSize: 10, color: '#94a3b8', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 170 }}>
                            {u.email}
                          </span>
                          {empresa && u.user_type === 'profesional' && (
                            <span style={{ fontSize: 10, color: '#0f766e', whiteSpace: 'nowrap', flexShrink: 0, display: 'flex', alignItems: 'center', gap: 2 }}>
                              <Building2 size={9} /> {empresa}
                            </span>
                          )}
                          {u.country?.trim() && (
                            <span style={{ fontSize: 10, color: '#94a3b8', whiteSpace: 'nowrap', flexShrink: 0, display: 'flex', alignItems: 'center', gap: 2 }}>
                              <MapPin size={9} /> {u.country}
                            </span>
                          )}
                        </div>
                      </div>
                    </div>
                  )
                })
              )}
            </div>
          </div>
        </div>
      )}

      {/* Groups tab */}
      {tab === 'groups' && (
        <div style={{ maxHeight: 440, overflowY: 'auto' }}>
          {loading ? (
            <div style={{ padding: 40, textAlign: 'center', fontSize: 12, color: '#94a3b8' }}>Cargando grupos...</div>
          ) : groups.length === 0 ? (
            <div style={{ padding: 40, textAlign: 'center', fontSize: 12, color: '#94a3b8' }}>No hay grupos creados en la audiencia</div>
          ) : (
            groups.map(g => {
              const isSelected = (value.groupIds ?? []).includes(g.id!)
              return (
                <div
                  key={g.id}
                  onClick={() => toggleGroup(g.id!)}
                  style={{
                    display: 'flex', alignItems: 'center', gap: 12,
                    padding: '10px 16px', cursor: 'pointer',
                    borderBottom: '1px solid #f8fafc',
                    background: isSelected ? '#faf5ff' : 'transparent',
                    transition: 'background 0.1s',
                  }}
                  onMouseEnter={e => { if (!isSelected) (e.currentTarget as HTMLDivElement).style.background = '#f8fafc' }}
                  onMouseLeave={e => { (e.currentTarget as HTMLDivElement).style.background = isSelected ? '#faf5ff' : 'transparent' }}
                >
                  <div style={{
                    width: 16, height: 16, borderRadius: 4, flexShrink: 0,
                    border: isSelected ? '2px solid #7c3aed' : '1.5px solid #cbd5e1',
                    background: isSelected ? '#7c3aed' : '#fff',
                    display: 'flex', alignItems: 'center', justifyContent: 'center', transition: 'all 0.12s',
                  }}>
                    {isSelected && <Check size={9} strokeWidth={3} color="#fff" />}
                  </div>
                  <div>
                    <div style={{ fontSize: 13, fontWeight: 600, color: '#1e293b' }}>{g.name}</div>
                    <div style={{ fontSize: 11, color: '#94a3b8' }}>{g.description || 'Sin descripción'}</div>
                  </div>
                </div>
              )
            })
          )}
        </div>
      )}
    </div>
  )
}

