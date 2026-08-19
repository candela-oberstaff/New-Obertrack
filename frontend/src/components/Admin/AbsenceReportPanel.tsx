import { Fragment, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Search, CalendarX, Mail, MessageCircle, MessageSquare, Eye, ChevronLeft, ChevronRight } from 'lucide-react'
import Avatar from '../Common/Avatar'
import { Select } from '../ui/Select'
import { FollowUpCell } from './FollowUpCell'
import type { AbsenceReportItem, FollowUpInfo } from '../../hooks/useAdmin'
import { adminService } from '../../services/api'
import { openWaConversation, waDigits } from '../../lib/whatsappInbox'
import { useNotification } from '../../context/NotificationContext'
import styles from './Admin.module.css'

const PER_PAGE = 10

/** Una ausencia sin resolver es distinta de una ya aprobada o rechazada. */
export function absenceStatus(item: { approved?: boolean; rejected?: boolean }) {
  if (item.rejected) return { label: 'Rechazada', className: 'danger' as const }
  if (item.approved) return { label: 'Aprobada', className: 'success' as const }
  return { label: 'Pendiente', className: 'warning' as const }
}

const absenceFollowUp = (g: { name: string; count: number }) =>
  `Hola ${g.name?.split(' ')[0] || ''}, te escribimos del equipo de Obertrack por el seguimiento de tus ${g.count} ausencia${g.count === 1 ? '' : 's'} registrada${g.count === 1 ? '' : 's'} este mes. ¿Está todo bien? Si necesitas apoyo, cuéntanos.`

export interface AbsenceGroup {
  user_id: number
  name: string
  email: string
  phone_number?: string
  avatar?: string
  tenant_id: number
  company: string
  count: number
  totalHours: number
  pending: number
  lastDate: string
  lastReason: string
  items: AbsenceReportItem[]
}

/** Agrupa las ausencias sueltas del mes por profesional. */
export function groupAbsences(items: AbsenceReportItem[]): AbsenceGroup[] {
  const map = new Map<number, AbsenceGroup>()
  for (const item of items) {
    const g = map.get(item.user_id) || {
      user_id: item.user_id, name: item.user, email: item.email, phone_number: item.phone_number,
      avatar: item.avatar, tenant_id: item.tenant_id, company: item.company,
      count: 0, totalHours: 0, pending: 0, lastDate: '', lastReason: '', items: [] as AbsenceReportItem[],
    }
    g.count++
    g.totalHours += item.absence_hours || 0
    if (!item.approved && !item.rejected) g.pending++
    if (!g.lastDate || (item.work_date || '') > g.lastDate) {
      g.lastDate = item.work_date
      g.lastReason = item.absence_reason
    }
    g.items.push(item)
    map.set(item.user_id, g)
  }
  return [...map.values()].sort((a, b) => b.count - a.count || (a.name || '').localeCompare(b.name || '', 'es', { sensitivity: 'base' }))
}

export interface AbsenceReportPanelProps {
  items: AbsenceReportItem[]
  followUps: Record<number, FollowUpInfo>
  onSetFollowUp: (userId: number, status: string) => void
  onOpenUser: (userId: number, sequence: number[]) => void
  /** En la ficha de una empresa sobra la columna de empresa. */
  showCompany?: boolean
  /** Filtro por persona; por defecto donde no hay empresa que filtrar. */
  showPerson?: boolean
  description?: string
  dataTour?: string
}

/**
 * Ausencias del mes agrupadas por profesional, con su detalle desplegable y la
 * gestión de customer success. Compartido entre el panel de administración
 * (todas las empresas) y la pestaña de actividad de una empresa.
 */
export function AbsenceReportPanel({
  items,
  followUps,
  onSetFollowUp,
  onOpenUser,
  showCompany = true,
  showPerson,
  description = 'Ausencias del mes agrupadas por profesional. Haz clic en una fila para ver el detalle.',
  dataTour,
}: AbsenceReportPanelProps) {
  const navigate = useNavigate()
  const notify = useNotification()
  const withPerson = showPerson ?? !showCompany
  const [person, setPerson] = useState<number | ''>('')
  const [search, setSearch] = useState('')
  const [page, setPage] = useState(1)
  const [expandedUserId, setExpandedUserId] = useState<number | null>(null)

  useEffect(() => {
    setPage(1)
  }, [search, person])

  const groups = groupAbsences(items)
  // Solo quien tiene ausencias este mes: ofrecer a toda la plantilla llevaría a
  // elegir a alguien y ver la tabla vacía.
  const personOptions = [...groups]
    .sort((a, b) => (a.name || '').localeCompare(b.name || '', 'es', { sensitivity: 'base' }))
    .map(g => ({ value: g.user_id, label: g.name }))

  const filtered = groups.filter(g => {
    if (withPerson && person !== '' && g.user_id !== person) return false
    const q = search.trim().toLowerCase()
    return !q || g.name?.toLowerCase().includes(q) || g.email?.toLowerCase().includes(q) || g.company?.toLowerCase().includes(q)
  })
  const totalPages = Math.max(1, Math.ceil(filtered.length / PER_PAGE))
  const currentPage = Math.min(page, totalPages)
  const paginated = filtered.slice((currentPage - 1) * PER_PAGE, currentPage * PER_PAGE)

  const emailHref = (g: AbsenceGroup) =>
    `mailto:${g.email}?subject=${encodeURIComponent('Seguimiento de ausencias en Obertrack')}&body=${encodeURIComponent(absenceFollowUp(g))}`

  // Igual que el semáforo de inactividad: el seguimiento se hace por nuestra
  // bandeja, no por el WhatsApp personal de quien hace clic, para que la
  // respuesta del profesional quede registrada. wa.me solo como respaldo.
  const openWhatsApp = async (g: AbsenceGroup) => {
    if (!waDigits(g.phone_number)) return
    adminService.logContact(g.user_id, 'whatsapp')
    const ok = await openWaConversation(g.phone_number, g.name, navigate, { draft: absenceFollowUp(g) })
    if (!ok) notify.error('No se pudo abrir la conversación de WhatsApp. Revisa la bandeja e inténtalo de nuevo.')
  }

  const columns = showCompany ? 7 : 6

  return (
    <div style={{ marginBottom: '32px' }} data-tour={dataTour}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', marginBottom: '14px' }}>
        <div>
          <h3 style={{ margin: 0, fontSize: '17px', fontWeight: 800, color: '#0f172a' }}>Reporte de ausencias</h3>
          <p style={{ margin: '2px 0 0', fontSize: '13px', color: '#64748b' }}>{description}</p>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap' }}>
          {withPerson && (
            <div style={{ minWidth: 240 }}>
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

      {filtered.length === 0 ? (
        <div className={styles['empty-state']} style={{ padding: '28px' }}>
          <CalendarX size={34} />
          <p>{items.length === 0 ? 'Sin ausencias registradas este mes' : 'Sin resultados con los filtros aplicados'}</p>
        </div>
      ) : (
        <div className={styles['users-table']}>
          <table>
            <thead>
              <tr>
                <th>Profesional</th>
                {showCompany && <th>Empresa</th>}
                <th>Ausencias</th>
                <th>Horas</th>
                <th>Última ausencia</th>
                <th>Gestión</th>
                <th>Acciones</th>
              </tr>
            </thead>
            <tbody>
              {paginated.map(g => {
                const expanded = expandedUserId === g.user_id
                const hasPhone = !!waDigits(g.phone_number)
                return (
                  <Fragment key={g.user_id}>
                    <tr
                      onClick={() => setExpandedUserId(expanded ? null : g.user_id)}
                      style={{ cursor: 'pointer', background: expanded ? 'rgba(204,51,204,0.05)' : undefined }}
                      title={expanded ? 'Ocultar detalle' : 'Ver detalle de las ausencias'}
                    >
                      <td>
                        <div className={styles['user-cell']}>
                          <Avatar src={g.avatar} name={g.name} size="sm" />
                          <div>
                            <span style={{ display: 'block' }}>{g.name}</span>
                            <small style={{ color: '#94a3b8' }}>{g.email}</small>
                          </div>
                        </div>
                      </td>
                      {showCompany && (
                        <td>
                          {g.tenant_id > 0 ? (
                            <button
                              type="button"
                              onClick={(e) => { e.stopPropagation(); navigate(`/admin/tenants/${g.tenant_id}`) }}
                              style={{ background: 'none', border: 'none', padding: 0, color: '#5a52e6', fontWeight: 600, cursor: 'pointer', textDecoration: 'underline', fontSize: 'inherit' }}
                            >
                              {g.company}
                            </button>
                          ) : (g.company || '—')}
                        </td>
                      )}
                      <td>
                        <span style={{ fontWeight: 700 }}>{g.count}</span>
                        {g.pending > 0 && (
                          <span style={{ marginLeft: 8, padding: '2px 8px', borderRadius: '999px', fontSize: '11px', fontWeight: 700, background: 'rgba(245,158,11,0.14)', color: '#b45309' }}>
                            {g.pending} pendiente{g.pending === 1 ? '' : 's'}
                          </span>
                        )}
                      </td>
                      <td>{g.totalHours.toFixed(1)} h</td>
                      <td>
                        {g.lastDate ? new Date(g.lastDate).toLocaleDateString('es-ES') : '—'}
                        <small style={{ display: 'block', color: '#94a3b8' }}>{g.lastReason}</small>
                      </td>
                      <td>
                        <FollowUpCell info={followUps[g.user_id]} onChange={status => onSetFollowUp(g.user_id, status)} />
                      </td>
                      <td>
                        <div className={styles['action-buttons']} onClick={(e) => e.stopPropagation()}>
                          <a href={emailHref(g)} onClick={() => adminService.logContact(g.user_id, 'email')} className={styles['btn-icon']} title={`Enviar email a ${g.email}`} style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center' }}>
                            <Mail size={16} />
                          </a>
                          {hasPhone ? (
                            <button type="button" onClick={() => openWhatsApp(g)} className={styles['btn-icon']} title={`Abrir la conversación de WhatsApp en la bandeja (${g.phone_number})`} style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', color: '#16a34a' }}>
                              <MessageCircle size={16} />
                            </button>
                          ) : (
                            <span className={styles['btn-icon']} title="Sin teléfono registrado" style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', opacity: 0.35, cursor: 'not-allowed' }}>
                              <MessageCircle size={16} />
                            </span>
                          )}
                          <button className={styles['btn-icon']} onClick={() => { adminService.logContact(g.user_id, 'chat'); navigate(`/chat?userId=${g.user_id}`) }} title="Chat interno" style={{ color: '#7c3aed' }}>
                            <MessageSquare size={16} />
                          </button>
                          <button className={styles['btn-icon']} onClick={() => onOpenUser(g.user_id, filtered.map(x => x.user_id))} title="Ver detalle del profesional">
                            <Eye size={16} />
                          </button>
                        </div>
                      </td>
                    </tr>
                    {expanded && (
                      <tr>
                        <td colSpan={columns} style={{ background: 'rgba(204,51,204,0.03)', padding: '10px 18px' }}>
                          <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                            {g.items.map(item => {
                              const status = absenceStatus(item)
                              return (
                                <div key={item.id} style={{ display: 'flex', alignItems: 'center', gap: '14px', fontSize: '13px', color: '#475569' }}>
                                  <span style={{ minWidth: 90, fontWeight: 600 }}>{new Date(item.work_date).toLocaleDateString('es-ES')}</span>
                                  <span style={{ flex: 1 }}>{item.absence_reason}</span>
                                  <span>{(item.absence_hours || 0).toFixed(1)} h</span>
                                  <span style={{ padding: '2px 10px', borderRadius: '999px', fontSize: '11px', fontWeight: 700, background: status.className === 'success' ? 'rgba(16,185,129,0.12)' : status.className === 'danger' ? 'rgba(239,68,68,0.12)' : 'rgba(245,158,11,0.14)', color: status.className === 'success' ? '#047857' : status.className === 'danger' ? '#b91c1c' : '#b45309' }}>
                                    {status.label}
                                  </span>
                                </div>
                              )
                            })}
                          </div>
                        </td>
                      </tr>
                    )}
                  </Fragment>
                )
              })}
            </tbody>
          </table>

          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', padding: '12px 16px' }}>
            <span style={{ fontSize: '13px', color: '#64748b' }}>
              Mostrando {(currentPage - 1) * PER_PAGE + 1}–{Math.min(currentPage * PER_PAGE, filtered.length)} de {filtered.length} profesionales con ausencias
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
