import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Search, CheckCircle2, Mail, MessageCircle, MessageSquare, Eye, ChevronLeft, ChevronRight, X } from 'lucide-react'
import Avatar from '../Common/Avatar'
import { Select } from '../ui/Select'
import { Skeleton } from '../ui'
import { FollowUpCell } from './FollowUpCell'
import type { ComposerRecipient } from './EmailComposerModal'
import type { TeamInactivityItem, FollowUpInfo } from '../../hooks/useAdmin'
import { adminService } from '../../services/api'
import { openWaConversation, waDigits } from '../../lib/whatsappInbox'
import { useNotification } from '../../context/NotificationContext'
import styles from './Admin.module.css'

// Tramos de inactividad. Los dos primeros son los chips del semáforo (la regla
// que dispara la alerta a customer success); los siguientes parten ese "2+ días"
// en algo accionable, porque con decenas de personas en rojo no es lo mismo
// quien lleva tres días que quien lleva un mes.
const TEAM_TIERS = [
  { value: 'yellow', label: '🟡 1 día', match: (d: number) => d === 1 },
  { value: 'red', label: '🔴 2+ días', match: (d: number) => d >= 2 },
  { value: '2-5', label: '2–5 días', match: (d: number) => d >= 2 && d <= 5 },
  { value: '6-15', label: '6–15 días', match: (d: number) => d >= 6 && d <= 15 },
  { value: '16+', label: 'Más de 15 días', match: (d: number) => d >= 16 },
] as const

type TeamTier = '' | (typeof TEAM_TIERS)[number]['value']

// Estado de gestión. "Sin gestionar" es el que de verdad se usa a diario: son
// los que nadie ha tocado todavía.
const TEAM_FOLLOWUPS = [
  { value: 'none', label: 'Sin gestionar' },
  { value: 'contacted', label: '📞 Contactado' },
  { value: 'justified', label: '✅ Justificado' },
  { value: 'escalated', label: '⚠️ Escalado' },
] as const

type TeamFollowUp = '' | (typeof TEAM_FOLLOWUPS)[number]['value']

const PER_PAGE = 10

export interface TeamActivityPanelProps {
  items: TeamInactivityItem[]
  loading?: boolean
  /** Gestión vigente por profesional (user_id → info). */
  followUps: Record<number, FollowUpInfo>
  onSetFollowUp: (userId: number, status: string) => void
  onCompose: (recipient: ComposerRecipient, body: string) => void
  /** El detalle recorre la lista de la que se entró; por eso viaja el orden. */
  onOpenUser: (userId: number, sequence: number[]) => void
  /** En la ficha de una empresa sobran la columna y el filtro de empresa. */
  showCompany?: boolean
  /**
   * Filtro por persona. Por defecto aparece donde no hay filtro de empresa (la
   * ficha de una empresa): ahí la lista es corta y ver quién está en ella es
   * más rápido que escribir el nombre. Por defecto: !showCompany.
   */
  showPerson?: boolean
  description?: string
  dataTour?: string
}

/** Mensaje de seguimiento prellenado para email y WhatsApp. */
export const inactivityFollowUpMessage = (u: { name: string; days_inactive: number }) =>
  `Hola ${u.name?.split(' ')[0] || ''}, te escribimos del equipo de Obertrack: notamos que no registras horas desde hace ${u.days_inactive} día${u.days_inactive === 1 ? '' : 's'}. ¿Está todo bien? Si tienes algún inconveniente cuéntanos para ayudarte.`

/**
 * Semáforo de inactividad: profesionales sin registrar horas, con la gestión de
 * customer success y las vías de contacto. Lo usan el panel de administración
 * (todas las empresas) y la ficha de una empresa (solo la suya); si cada una
 * tuviera su copia, se irían separando al tocar un filtro.
 */
export function TeamActivityPanel({
  items,
  loading = false,
  followUps,
  onSetFollowUp,
  onCompose,
  onOpenUser,
  showCompany = true,
  showPerson,
  description = 'Profesionales sin registrar horas. Los de 2+ días disparan alerta automática al equipo de customer success.',
  dataTour,
}: TeamActivityPanelProps) {
  const navigate = useNavigate()
  const notify = useNotification()
  const withPerson = showPerson ?? !showCompany
  const [person, setPerson] = useState<number | ''>('')
  const [search, setSearch] = useState('')
  // Los dos chips y el desplegable de inactividad escriben en el mismo estado:
  // así no pueden contradecirse y el desplegable siempre enseña lo aplicado.
  const [tier, setTier] = useState<TeamTier>('')
  // 0 = "Sin empresa" (profesionales sin tenant), '' = todas.
  const [company, setCompany] = useState<number | ''>('')
  const [followUpFilter, setFollowUpFilter] = useState<TeamFollowUp>('')
  const [page, setPage] = useState(1)

  useEffect(() => {
    setPage(1)
  }, [search, tier, company, followUpFilter, person])

  // Las personas de la propia lista, ordenadas por nombre y solo con el nombre:
  // dos homónimos salen como dos opciones iguales, pero la fila que queda en la
  // tabla al elegir sí lleva el correo debajo.
  const personOptions = useMemo(() =>
    [...items]
      .sort((a, b) => (a.name || '').localeCompare(b.name || '', 'es', { sensitivity: 'base' }))
      .map(u => ({ value: u.id, label: u.name })),
    [items])

  // Empresas presentes en la lista, con cuántos inactivos tiene cada una: el
  // dato que se busca al abrir el desplegable es precisamente ese.
  const companyOptions = useMemo(() => {
    const byTenant = new Map<number, { label: string; count: number }>()
    for (const u of items) {
      const key = u.tenant_id || 0
      // El backend manda '-' cuando no hay empresa que mostrar; como etiqueta de
      // un filtro no dice nada.
      const name = u.company && u.company !== '-' ? u.company : ''
      const entry = byTenant.get(key) ?? { label: key === 0 ? 'Sin empresa' : (name || `Empresa #${key}`), count: 0 }
      entry.count++
      byTenant.set(key, entry)
    }
    return [...byTenant.entries()]
      .sort((a, b) => a[1].label.localeCompare(b[1].label, 'es', { sensitivity: 'base' }))
      .map(([value, { label, count }]) => ({ value, label: `${label} (${count})` }))
  }, [items])

  // Todo menos el tramo: los contadores de los chips se calculan sobre esta base
  // para que al filtrar por empresa no digan un número que la tabla no enseña.
  const base = items.filter(u => {
    const q = search.trim().toLowerCase()
    if (q && !(u.name?.toLowerCase().includes(q) || u.email?.toLowerCase().includes(q) || u.company?.toLowerCase().includes(q))) return false
    if (withPerson && person !== '' && u.id !== person) return false
    if (showCompany && company !== '' && (u.tenant_id || 0) !== company) return false
    if (followUpFilter) {
      const status = followUps[u.id]?.status
      if (followUpFilter === 'none' ? !!status : status !== followUpFilter) return false
    }
    return true
  })

  const filtered = base
    .filter(u => {
      const active = TEAM_TIERS.find(t => t.value === tier)
      return !active || active.match(u.days_inactive)
    })
    .sort((a, b) => b.days_inactive - a.days_inactive || (a.name || '').localeCompare(b.name || '', 'es', { sensitivity: 'base' }))

  const yellowCount = base.filter(u => u.days_inactive === 1).length
  const redCount = base.filter(u => u.days_inactive >= 2).length
  const hasFilters = !!(search.trim() || tier || company !== '' || followUpFilter || person !== '')
  const clearFilters = () => { setSearch(''); setTier(''); setCompany(''); setFollowUpFilter(''); setPerson('') }

  const totalPages = Math.max(1, Math.ceil(filtered.length / PER_PAGE))
  const currentPage = Math.min(page, totalPages)
  const paginated = filtered.slice((currentPage - 1) * PER_PAGE, currentPage * PER_PAGE)

  // Abre el seguimiento por WhatsApp DENTRO de la app.
  //
  // Antes esto era un enlace a wa.me: sacaba a la persona de Obertrack y el
  // mensaje se enviaba desde el WhatsApp personal de quien hacía clic, así que
  // la respuesta del profesional no quedaba en ninguna parte. Ahora abre —o
  // crea— la conversación en nuestra bandeja, que es donde el equipo la sigue.
  //
  // Es el mismo camino que ya usa la ficha de empresa (TenantDetail).
  const openWhatsApp = async (u: TeamInactivityItem) => {
    if (!waDigits(u.phone_number)) return
    adminService.logContact(u.id, 'whatsapp')
    // El mensaje de seguimiento viaja para que llegue escrito y solo haya que
    // revisarlo: es lo que hacía el ?text= del enlace a wa.me que había antes.
    const ok = await openWaConversation(u.phone_number, u.name, navigate, {
      draft: inactivityFollowUpMessage(u),
    })
    if (!ok) notify.error('No se pudo abrir la conversación de WhatsApp. Revisa la bandeja e inténtalo de nuevo.')
  }

  return (
    <div style={{ marginBottom: '32px' }} data-tour={dataTour}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', marginBottom: '14px' }}>
        <div>
          <h3 style={{ margin: 0, fontSize: '17px', fontWeight: 800, color: '#0f172a' }}>Actividad de equipo</h3>
          <p style={{ margin: '2px 0 0', fontSize: '13px', color: '#64748b' }}>{description}</p>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap' }}>
          {/* Los chips cuentan sobre lo que ya filtraron empresa, gestión y
              búsqueda: si dijeran el total global, contradirían a la tabla que
              tienen justo debajo. */}
          <button
            type="button"
            onClick={() => setTier(tier === 'yellow' ? '' : 'yellow')}
            style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '6px 12px', borderRadius: '999px', border: tier === 'yellow' ? '1px solid #f59e0b' : '1px solid #e2e8f0', background: 'rgba(245,158,11,0.1)', color: '#b45309', fontSize: '13px', fontWeight: 700, cursor: 'pointer' }}
          >
            🟡 1 día: {yellowCount}
          </button>
          <button
            type="button"
            onClick={() => setTier(tier === 'red' ? '' : 'red')}
            style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '6px 12px', borderRadius: '999px', border: tier === 'red' ? '1px solid #ef4444' : '1px solid #e2e8f0', background: 'rgba(239,68,68,0.1)', color: '#b91c1c', fontSize: '13px', fontWeight: 700, cursor: 'pointer' }}
          >
            🔴 2+ días: {redCount}
          </button>
          <div className={styles['search-box']}>
            <Search size={16} />
            <input
              type="text"
              placeholder={showCompany ? 'Buscar por nombre, correo o empresa...' : 'Buscar por nombre o correo...'}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </div>
        </div>
      </div>

      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap', marginBottom: '12px' }}>
        {showCompany && (
          <div style={{ minWidth: 220 }}>
            <Select
              fullWidth
              clearable
              placeholder="Todas las empresas"
              value={company}
              onChange={v => setCompany(v === '' ? '' : Number(v))}
              options={companyOptions}
            />
          </div>
        )}
        {withPerson && (
          <div style={{ minWidth: 230 }}>
            <Select
              fullWidth
              clearable
              searchable
              placeholder="Todos los profesionales"
              value={person}
              onChange={v => setPerson(v === '' ? '' : Number(v))}
              options={personOptions}
            />
          </div>
        )}
        <div style={{ minWidth: 190 }}>
          <Select
            fullWidth
            clearable
            placeholder="Cualquier gestión"
            value={followUpFilter}
            onChange={v => setFollowUpFilter(String(v) as TeamFollowUp)}
            options={TEAM_FOLLOWUPS.map(f => ({ value: f.value, label: f.label }))}
          />
        </div>
        <div style={{ minWidth: 180 }}>
          <Select
            fullWidth
            clearable
            placeholder="Cualquier inactividad"
            value={tier}
            onChange={v => setTier(String(v) as TeamTier)}
            options={TEAM_TIERS.map(t => ({ value: t.value, label: t.label }))}
          />
        </div>
        {hasFilters && (
          <>
            <button
              type="button"
              onClick={clearFilters}
              style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '9px 14px', border: '1px solid #e2e8f0', borderRadius: '10px', background: 'transparent', color: '#64748b', fontSize: '13px', fontWeight: 600, cursor: 'pointer', whiteSpace: 'nowrap' }}
              title="Quitar todos los filtros"
            >
              <X size={14} /> Limpiar filtros
            </button>
            <span style={{ fontSize: '13px', color: '#64748b' }}>
              {filtered.length} de {items.length} profesionales
            </span>
          </>
        )}
      </div>

      {loading ? (
        <Skeleton height={180} radius={12} />
      ) : filtered.length === 0 ? (
        <div className={styles['empty-state']} style={{ padding: '28px' }}>
          <CheckCircle2 size={34} />
          <p>{items.length === 0 ? 'Todo el equipo está registrando horas 🎉' : 'Sin resultados con los filtros aplicados'}</p>
        </div>
      ) : (
        <div className={styles['users-table']}>
          <table>
            <thead>
              <tr>
                <th>Profesional</th>
                {showCompany && <th>Empresa</th>}
                <th>Últ. actividad</th>
                <th>Inactividad</th>
                <th>Gestión</th>
                <th>Acciones</th>
              </tr>
            </thead>
            <tbody>
              {paginated.map(u => {
                const isRed = u.days_inactive >= 2
                const hasPhone = !!waDigits(u.phone_number)
                return (
                  <tr key={u.id} style={{ background: isRed ? 'rgba(239,68,68,0.07)' : 'rgba(245,158,11,0.07)' }}>
                    <td>
                      <div className={styles['user-cell']} style={{ cursor: 'pointer' }} onClick={() => onOpenUser(u.id, filtered.map(x => x.id))} title="Ver detalle del profesional">
                        <Avatar src={u.avatar} name={u.name} size="sm" />
                        <div>
                          <span style={{ display: 'block' }}>{u.name}</span>
                          <small style={{ color: '#94a3b8' }}>{u.email}{u.job_title ? ` · ${u.job_title}` : ''}</small>
                        </div>
                      </div>
                    </td>
                    {showCompany && (
                      <td>
                        {u.tenant_id > 0 ? (
                          <button
                            type="button"
                            onClick={() => navigate(`/admin/tenants/${u.tenant_id}`)}
                            style={{ background: 'none', border: 'none', padding: 0, color: '#5a52e6', fontWeight: 600, cursor: 'pointer', textDecoration: 'underline', fontSize: 'inherit' }}
                            title="Ver detalle de la empresa"
                          >
                            {u.company}
                          </button>
                        ) : (u.company || '—')}
                      </td>
                    )}
                    <td>{u.last_active ? new Date(u.last_active).toLocaleDateString('es-ES') : '—'}</td>
                    <td>
                      <span style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '4px 10px', borderRadius: '999px', fontSize: '12px', fontWeight: 700, background: isRed ? 'rgba(239,68,68,0.12)' : 'rgba(245,158,11,0.14)', color: isRed ? '#b91c1c' : '#b45309' }}>
                        {isRed ? '🔴' : '🟡'} {u.days_inactive} día{u.days_inactive === 1 ? '' : 's'} háb.
                      </span>
                    </td>
                    <td>
                      <FollowUpCell info={followUps[u.id]} onChange={status => onSetFollowUp(u.id, status)} />
                    </td>
                    <td>
                      <div className={styles['action-buttons']}>
                        <button
                          type="button"
                          onClick={() => onCompose({ id: u.id, name: u.name, email: u.email }, inactivityFollowUpMessage(u))}
                          className={styles['btn-icon']}
                          title={`Enviar email a ${u.email}`}
                          style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center' }}
                        >
                          <Mail size={16} />
                        </button>
                        {hasPhone ? (
                          <button
                            type="button"
                            onClick={() => openWhatsApp(u)}
                            className={styles['btn-icon']}
                            title={`Abrir la conversación de WhatsApp en la bandeja (${u.phone_number})`}
                            style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', color: '#16a34a' }}
                          >
                            <MessageCircle size={16} />
                          </button>
                        ) : (
                          <span className={styles['btn-icon']} title="Sin teléfono registrado" style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', opacity: 0.35, cursor: 'not-allowed' }}>
                            <MessageCircle size={16} />
                          </span>
                        )}
                        <button
                          className={styles['btn-icon']}
                          onClick={() => { adminService.logContact(u.id, 'chat'); navigate(`/chat?userId=${u.id}`) }}
                          title="Chat interno"
                          style={{ color: '#7c3aed' }}
                        >
                          <MessageSquare size={16} />
                        </button>
                        <button
                          className={styles['btn-icon']}
                          onClick={() => onOpenUser(u.id, filtered.map(x => x.id))}
                          title="Ver detalle"
                        >
                          <Eye size={16} />
                        </button>
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>

          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', padding: '12px 16px' }}>
            <span style={{ fontSize: '13px', color: '#64748b' }}>
              Mostrando {(currentPage - 1) * PER_PAGE + 1}–{Math.min(currentPage * PER_PAGE, filtered.length)} de {filtered.length} profesionales
            </span>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <button type="button" className={styles['btn-icon']} onClick={() => setPage(p => Math.max(1, p - 1))} disabled={currentPage <= 1} style={{ opacity: currentPage <= 1 ? 0.4 : 1 }} title="Página anterior">
                <ChevronLeft size={16} />
              </button>
              <span style={{ fontSize: '13px', fontWeight: 600, color: '#334155', whiteSpace: 'nowrap' }}>
                Página {currentPage} de {totalPages}
              </span>
              <button type="button" className={styles['btn-icon']} onClick={() => setPage(p => Math.min(totalPages, p + 1))} disabled={currentPage >= totalPages} style={{ opacity: currentPage >= totalPages ? 0.4 : 1 }} title="Página siguiente">
                <ChevronRight size={16} />
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
