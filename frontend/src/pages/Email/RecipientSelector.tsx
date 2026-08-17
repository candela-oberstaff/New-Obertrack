import { useState, useEffect, useMemo } from 'react'
import { Search, X, Users, Mail, UserCheck, Check, MapPin } from 'lucide-react'
import { emailService } from '../../services/emailService'
import { audienceService, AudienceGroup } from '../../services/audienceService'
import { Select } from '../../components/ui/Select'

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
  is_superadmin: boolean
  country?: string
}

// Valor del filtro de país cuando se quiere ver a quienes NO tienen país
// cargado. No es un capricho: al filtrar por "Venezuela" esa gente desaparece
// del listado sin aviso, y en un envío masivo eso es quedarse fuera en silencio.
// Con esta opción se los puede encontrar (y arreglar su ficha) o incluirlos a
// propósito.
const NO_COUNTRY = '__sin_pais__'

interface Props {
  value: RecipientValue
  onChange: (v: RecipientValue) => void
}

type Tab = 'users' | 'groups' | 'express'

export default function RecipientSelector({ value, onChange }: Props) {
  const [tab, setTab] = useState<Tab>('users')
  const [query, setQuery] = useState('')
  const [roleFilter, setRoleFilter] = useState<string>('all')
  const [countryFilter, setCountryFilter] = useState<string>('all')
  const [allUsers, setAllUsers] = useState<User[]>([])
  const [groups, setGroups] = useState<AudienceGroup[]>([])
  const [expressRaw, setExpressRaw] = useState('')
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
      if (roleFilter === 'superadmin') return u.is_superadmin || u.user_type === 'superadmin'
      if (roleFilter === 'customer_success') return u.user_type === 'customer_success'
      if (roleFilter === 'empleador') return u.user_type === 'empleador'
      if (roleFilter === 'profesional') return u.user_type === 'profesional'
      if (roleFilter === 'manager') return u.is_manager
      return true
    })
  }, [allUsers, roleFilter])

  // Los países salen de la gente que hay, no de un catálogo fijo: el catálogo
  // tiene ~250 países y casi todos estarían vacíos. Se ordenan por cantidad
  // porque el país con más gente es casi siempre el que se busca.
  const countryOptions = useMemo(() => {
    const counts = new Map<string, number>()
    let sinPais = 0
    for (const u of usersByRole) {
      const c = (u.country ?? '').trim()
      if (c) counts.set(c, (counts.get(c) ?? 0) + 1)
      else sinPais++
    }
    // El país elegido nunca desaparece del desplegable aunque el filtro de rol
    // lo deje en cero: si desapareciera, el selector saltaría solo a otro valor
    // y el listado cambiaría sin que nadie lo haya tocado.
    if (countryFilter !== 'all' && countryFilter !== NO_COUNTRY && !counts.has(countryFilter)) {
      counts.set(countryFilter, 0)
    }
    const options = Array.from(counts.entries())
      .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0], 'es'))
      .map(([name, count]) => ({ value: name, label: `${name} (${count})` }))

    if (sinPais > 0 || countryFilter === NO_COUNTRY) {
      options.push({ value: NO_COUNTRY, label: `Sin país registrado (${sinPais})` })
    }
    return options
  }, [usersByRole, countryFilter])

  const filteredUsers = useMemo(() => {
    const q = query.toLowerCase()
    return usersByRole.filter(u => {
      const matchQuery =
        u.name?.toLowerCase().includes(q) ||
        u.email?.toLowerCase().includes(q)

      if (!matchQuery) return false

      const country = (u.country ?? '').trim()
      if (countryFilter === NO_COUNTRY) return country === ''
      if (countryFilter !== 'all') return country === countryFilter

      return true
    })
  }, [usersByRole, query, countryFilter])

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

  const applyExpress = () => {
    const lines = expressRaw.split(/[\n,;]/).map(l => l.trim()).filter(Boolean)
    const contacts = lines.map(line => {
      const emailMatch = line.match(/[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}/)
      if (!emailMatch) return null
      const email = emailMatch[0]
      const name = line.replace(email, '').replace(/[<>]/g, '').trim() || email.split('@')[0]
      return { name, email }
    }).filter(Boolean) as Array<{ name: string; email: string }>
    onChange({ ...value, expressContacts: contacts })
  }

  const totalSelected = value.userIds.length + (value.groupIds?.length ?? 0) + (value.expressContacts?.length ?? 0)

  return (
    <div className="border border-slate-200 rounded-xl overflow-hidden bg-white shadow-sm">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 bg-slate-50 border-b border-slate-200">
        <span className="text-sm font-semibold text-slate-700">Seleccionar destinatarios</span>
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
              {t === 'express' && <Mail size={14} />}
              {t === 'users' ? 'Usuarios' : t === 'groups' ? 'Grupos' : 'Libres'}
            </button>
          )
        })}
      </div>

      {/* Content */}
      <div className="max-h-[300px] overflow-y-auto">
        {tab === 'users' && (
          <>
            {/* Search and Role filter bar */}
            <div className="p-3 border-b border-slate-100 bg-white sticky top-0 z-10 flex flex-col gap-2 shadow-sm">
              <div className="flex items-center gap-2 bg-slate-50 border border-slate-200 rounded-lg px-3 py-1.5">
                <Search size={14} className="text-slate-400" />
                <input
                  value={query}
                  onChange={e => setQuery(e.target.value)}
                  placeholder="Buscar por nombre o email..."
                  className="border-none bg-transparent outline-none text-xs flex-1 text-slate-800 placeholder-slate-400"
                />
                {query && <X size={14} className="text-slate-400 cursor-pointer hover:text-slate-600" onClick={() => setQuery('')} />}
              </div>

              {/* Roles Badges Filters */}
              <div className="flex flex-wrap gap-1.5 pt-1">
                {[
                  { value: 'all', label: 'Todos' },
                  { value: 'profesional', label: 'Profesionales' },
                  { value: 'empleador', label: 'Empleadores' },
                  { value: 'customer_success', label: 'Customer Success' },
                  { value: 'manager', label: 'Managers' },
                  { value: 'superadmin', label: 'Superadmins' }
                ].map(r => (
                  <button
                    key={r.value}
                    onClick={() => setRoleFilter(r.value)}
                    className={`px-2.5 py-1 rounded-full text-[10px] font-bold transition-all border ${
                      roleFilter === r.value
                        ? 'bg-purple-600 border-purple-600 text-white shadow-sm'
                        : 'bg-white border-slate-200 text-slate-600 hover:bg-slate-50'
                    }`}
                  >
                    {r.label}
                  </button>
                ))}
              </div>

              {/* Filtro por país. Usa el Select del proyecto y no un <select>
                  nativo: el nativo dibuja el menú con el estilo del sistema
                  operativo (que no se puede tocar) y además lo recortaba el
                  overflow del panel. Este monta el menú en un portal, trae
                  buscador solo. */}
              <div className="flex items-center gap-2 pt-1">
                <div className="flex-1 min-w-0">
                  <Select
                    options={[{ value: 'all', label: 'Todos los países' }, ...countryOptions]}
                    value={countryFilter}
                    onChange={v => setCountryFilter(String(v))}
                    leftIcon={<MapPin size={13} />}
                    className="ui-select__trigger--compact"
                    ariaLabel="Filtrar por país"
                    fullWidth
                  />
                </div>
                {countryFilter !== 'all' && (
                  <button
                    onClick={() => setCountryFilter('all')}
                    className="text-[10px] font-bold text-slate-500 hover:text-slate-700 shrink-0"
                  >
                    Quitar
                  </button>
                )}
              </div>

              {/* Bulk actions inside filters */}
              <div className="flex justify-between items-center pt-1 border-t border-slate-100 mt-1">
                <span className="text-[10px] text-slate-400 font-semibold">{filteredUsers.length} encontrados</span>
                <div className="flex gap-2">
                  <button onClick={selectAllFiltered} className="text-[10px] font-bold text-purple-600 hover:text-purple-700">
                    Seleccionar todos
                  </button>
                  <button onClick={deselectAllFiltered} className="text-[10px] font-bold text-slate-500 hover:text-slate-700">
                    Limpiar visibles
                  </button>
                </div>
              </div>
            </div>

            {loading ? (
              <div className="p-8 text-center text-xs text-slate-400">Cargando destinatarios...</div>
            ) : filteredUsers.length === 0 ? (
              <div className="p-8 text-center text-xs text-slate-400">Sin resultados para el filtro actual</div>
            ) : (
              filteredUsers.map(u => {
                const isSelected = value.userIds.includes(u.id)
                return (
                  <div
                    key={u.id}
                    onClick={() => toggleUser(u.id)}
                    className={`flex items-center gap-3 px-4 py-2.5 cursor-pointer border-b border-slate-50 transition-colors ${
                      isSelected ? 'bg-purple-50/40 hover:bg-purple-50/60' : 'hover:bg-slate-50/50'
                    }`}
                  >
                    <div className={`w-4 h-4 rounded border flex items-center justify-center transition-all ${
                      isSelected ? 'bg-purple-600 border-purple-600 text-white' : 'border-slate-300 bg-white'
                    }`}>
                      {isSelected && <Check size={10} strokeWidth={3} />}
                    </div>
                    <div className="flex-1">
                      <div className="text-xs font-semibold text-slate-800 flex items-center gap-2">
                        {u.name}
                        {u.is_manager && (
                          <span className="text-[8px] bg-indigo-100 text-indigo-700 px-1.5 py-0.5 rounded font-bold uppercase">
                            Manager
                          </span>
                        )}
                        {(u.is_superadmin || u.user_type === 'superadmin') && (
                          <span className="text-[8px] bg-red-100 text-red-700 px-1.5 py-0.5 rounded font-bold uppercase">
                            Admin
                          </span>
                        )}
                        {u.user_type && u.user_type !== 'profesional' && u.user_type !== 'superadmin' && (
                          <span className="text-[8px] bg-slate-100 text-slate-600 px-1.5 py-0.5 rounded font-bold uppercase">
                            {u.user_type}
                          </span>
                        )}
                      </div>
                      <div className="text-[10px] text-slate-400 flex items-center gap-1.5">
                        <span className="truncate">{u.email}</span>
                        {u.country?.trim() && (
                          <span className="text-slate-400 shrink-0 flex items-center gap-0.5">
                            <MapPin size={9} /> {u.country}
                          </span>
                        )}
                      </div>
                    </div>
                  </div>
                )
              })
            )}
          </>
        )}

        {tab === 'groups' && (
          loading ? (
            <div className="p-8 text-center text-xs text-slate-400">Cargando grupos...</div>
          ) : groups.length === 0 ? (
            <div className="p-8 text-center text-xs text-slate-400">No hay grupos creados en la audiencia</div>
          ) : (
            groups.map(g => {
              const isSelected = (value.groupIds ?? []).includes(g.id!)
              return (
                <div
                  key={g.id}
                  onClick={() => toggleGroup(g.id!)}
                  className={`flex items-center gap-3 px-4 py-2.5 cursor-pointer border-b border-slate-50 transition-colors ${
                    isSelected ? 'bg-purple-50/40 hover:bg-purple-50/60' : 'hover:bg-slate-50/50'
                  }`}
                >
                  <div className={`w-4 h-4 rounded border flex items-center justify-center transition-all ${
                    isSelected ? 'bg-purple-600 border-purple-600 text-white' : 'border-slate-300 bg-white'
                  }`}>
                    {isSelected && <Check size={10} strokeWidth={3} />}
                  </div>
                  <div>
                    <div className="text-xs font-semibold text-slate-800">{g.name}</div>
                    <div className="text-[10px] text-slate-400">{g.description || 'Sin descripción'}</div>
                  </div>
                </div>
              )
            })
          )
        )}

        {tab === 'express' && (
          <div className="p-4">
            <p className="text-[11px] text-slate-500 mb-2.5">
              Ingresa los correos electrónicos destinatarios separados por comas, punto y coma o saltos de línea.
              <br />
              Formatos válidos: <code>Nombre &lt;ejemplo@dominio.com&gt;</code> o simplemente <code>ejemplo@dominio.com</code>.
            </p>
            <textarea
              value={expressRaw}
              onChange={e => setExpressRaw(e.target.value)}
              rows={4}
              placeholder="juan@empresa.com&#10;Maria Garcia <maria@empresa.com>"
              className="w-full border border-slate-200 rounded-lg p-2.5 text-xs outline-none focus:border-purple-600 resize-none text-slate-800 placeholder-slate-400"
            />
            <button
              onClick={applyExpress}
              className="mt-2.5 px-4 py-1.5 bg-purple-600 hover:bg-purple-700 text-white text-xs font-semibold rounded-lg shadow-sm transition-all"
            >
              Cargar contactos ({expressRaw.split(/[\n,;]/).filter(l => l.trim().includes('@')).length})
            </button>
            {(value.expressContacts?.length ?? 0) > 0 && (
              <div className="mt-2.5 text-xs text-emerald-600 font-semibold flex items-center gap-1">
                <Check size={14} strokeWidth={3} /> {value.expressContacts.length} contacto{value.expressContacts.length !== 1 ? 's' : ''} listo{value.expressContacts.length !== 1 ? 's' : ''}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
