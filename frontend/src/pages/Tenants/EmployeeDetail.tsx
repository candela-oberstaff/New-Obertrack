import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, Clock, CheckSquare, Ban, CheckCircle2, RefreshCw, User as UserIcon, Activity, ChevronLeft, ChevronRight, MessageSquare, FileText, Eye } from 'lucide-react'
import { useEmployeeTracking, usePersonActivity, useTenantDetail } from '../../hooks'
import { adminService } from '../../services/api'
import { useAuth } from '../../context/AuthContext'
import Avatar from '../../components/Common/Avatar'
import { RecordPager, Skeleton } from '../../components/ui'
import { useConfirm } from '../../components/ui/ConfirmProvider'
import { ExpedienteModal } from '../../components/Admin/ExpedienteModal'
import { WorkHourDetailModal } from '../../components/WorkHours/Modals/WorkHourDetailModal'
import type { WorkHour } from '../../types'
import { formatDateOnly } from '../../utils/date'
import { htmlToText } from '../../utils/sanitize'
import { WORK_TYPE } from '../../utils/workHours'
import { groupByDay } from './activityGrouping'
import { ACTIVITY_STYLE, ACTIVITY_LABEL, ACTIVITY_FALLBACK, CONTACT_STYLE } from './activityStyle'
import { EmployeeFicha, EmployeeKpis, timeSince } from './EmployeeFicha'
import { EmployeeSeguimiento } from './EmployeeSeguimiento'
import { EmployeeSoporte } from './EmployeeSoporte'
import styles from './Tenants.module.css'

const ACTIVITY_PER_PAGE = 20

export default function EmployeeDetail() {
  const { id, eid } = useParams<{ id: string; eid: string }>()
  const navigate = useNavigate()
  const tenantId = Number(id)
  const employeeId = Number(eid)
  const { tracking, isLoading, error, toggleStatus, resetPassword } = useEmployeeTracking(employeeId)
  // La plantilla de la empresa ya está en caché al llegar desde su ficha; se usa
  // para poner nombre al manager, que en el tracking solo viaja como id.
  const { employees } = useTenantDetail(tenantId)
  const { user: viewer } = useAuth()
  const canManage = !!viewer?.is_superadmin

  // Empleos del profesional. El expediente y la bitácora cuelgan del EMPLEO, no
  // de la persona: alguien que pasó por dos empresas tiene dos expedientes, y
  // desde esta ficha solo interesa el de la empresa por la que se entró.
  const { data: employments = [] } = useQuery({
    queryKey: ['user-employments', employeeId],
    queryFn: () => adminService.getUserEmployments(employeeId),
    enabled: !!employeeId,
  })
  const employment =
    employments.find((e: any) => e.company_id === tenantId && e.status === 'active') ??
    employments.find((e: any) => e.company_id === tenantId)

  const [tab, setTab] = useState<'expediente' | 'jornadas' | 'tareas' | 'seguimiento' | 'soporte' | 'actividad'>('expediente')
  // Jornada abierta en el detalle (null = ninguna).
  const [detailId, setDetailId] = useState<number | null>(null)
  const [actPage, setActPage] = useState(1)
  const { activity, total: actTotal, isLoading: actLoading, isFetching: actFetching, error: actError } =
    usePersonActivity(tenantId, employeeId, actPage, ACTIVITY_PER_PAGE)
  const actTotalPages = Math.max(1, Math.ceil(actTotal / ACTIVITY_PER_PAGE))
  const confirm = useConfirm()

  const handleReset = async () => {
    const ok = await confirm({
      title: 'Resetear contraseña',
      message: '¿Resetear la contraseña a "temporary123"?',
      confirmLabel: 'Resetear',
      variant: 'primary',
    })
    if (ok) await resetPassword('temporary123')
  }

  const getWorkHourStatus = (wh: any) => {
    if (wh.approved) return { className: styles.badgeActive, label: 'Aprobada' }
    if (wh.rejected) return { className: styles.badgeRejected, label: 'Rechazada' }
    return { className: styles.badgePending, label: 'Pendiente' }
  }

  if (isLoading) {
    return (
      <div className={styles.page}>
        <div className={styles.loading}>
          <div className={styles.spinner} />
          <p>Cargando profesional...</p>
        </div>
      </div>
    )
  }

  if (error || !tracking) {
    return (
      <div className={styles.page}>
        <button className={styles.backBtn} onClick={() => navigate(`/admin/tenants/${id}`)}>
          <ArrowLeft size={18} /> Volver
        </button>
        <div className={styles.empty}>
          <UserIcon size={40} />
          <p>{error || 'Profesional no encontrado'}</p>
        </div>
      </div>
    )
  }

  const { user, summary, work_hours, tasks } = tracking

  const manager = user.manager_id ? employees.find(e => e.id === user.manager_id) : null

  // El modal de detalle habla el tipo completo de WorkHour; el tracking manda
  // un subconjunto. Se completa con lo que esta pantalla ya sabe (de quién es
  // la jornada) en vez de volver a pedir el registro entero.
  const detailRecord = ((): WorkHour | null => {
    const wh = detailId === null ? undefined : work_hours.find(w => w.id === detailId)
    if (!wh) return null
    return {
      ...wh,
      user_id: employeeId,
      user,
      work_type: wh.work_type as WorkHour['work_type'],
      created_at: wh.work_date,
      updated_at: wh.work_date,
    }
  })()
  const phone = user.phone_number?.trim()
  const waNumber = phone?.replace(/\D/g, '')
  const seniority = timeSince(user.created_at)

  return (
    <div className={styles.page}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', marginBottom: '20px' }}>
        <button className={styles.backBtn} style={{ marginBottom: 0 }} onClick={() => navigate(`/admin/tenants/${id}`)}>
          <ArrowLeft size={18} /> Profesionales
        </button>
        <RecordPager
          scope={`tenant-employees:${Number(id)}`}
          currentId={employeeId}
          toPath={eid => `/admin/tenants/${id}/employees/${eid}`}
          noun="profesional"
        />
      </div>

      <div className={styles.detailHeader}>
        <div className={styles.detailIdentity}>
          <Avatar src={user.avatar} name={user.name} size="lg" />
          <div>
            <div className={styles.detailTitleRow}>
              <h1>{user.name}</h1>
              <span className={`${styles.badge} ${user.is_active ? styles.badgeActive : styles.badgeSuspended}`}>
                {user.is_active ? 'Activo' : 'Inactivo'}
              </span>
            </div>
            <div className={styles.detailMeta}>
              <span>{user.email}</span>
              <span className={styles.typeBadge}>{user.is_manager ? 'manager' : user.user_type}</span>
              {user.job_title?.trim() && <span>{user.job_title}</span>}
            </div>
          </div>
        </div>
        <div className={styles.headerActions}>
          {/* El teléfono es un dato muerto si hay que copiarlo a mano: desde
              aquí se abre la conversación directamente. */}
          {waNumber && (
            <a
              className={styles.secondaryBtn}
              href={`https://wa.me/${waNumber}`}
              target="_blank"
              rel="noreferrer noopener"
              title={`Escribir a ${phone} por WhatsApp`}
            >
              <MessageSquare size={16} /> WhatsApp
            </a>
          )}
          <button className={styles.secondaryBtn} onClick={handleReset}>
            <RefreshCw size={16} /> Resetear contraseña
          </button>
          {user.is_active ? (
            <button className={styles.dangerBtn} onClick={toggleStatus}>
              <Ban size={16} /> Desactivar
            </button>
          ) : (
            <button className={styles.successBtn} onClick={toggleStatus}>
              <CheckCircle2 size={16} /> Activar
            </button>
          )}
        </div>
      </div>

      {/* Ficha del profesional. Estos datos ya viajaban en la respuesta del
          tracking; enseñarlos evita tener que abrir el modal de edición solo
          para saber quién es esta persona y quién la supervisa. */}
      <EmployeeFicha user={user} managerName={manager?.name} />

      <EmployeeKpis summary={summary} />

      <div className={styles.subTabs}>
        <button className={tab === 'expediente' ? styles.subTabActive : styles.subTab} onClick={() => setTab('expediente')}>Expediente</button>
        <button className={tab === 'jornadas' ? styles.subTabActive : styles.subTab} onClick={() => setTab('jornadas')}>Jornadas ({work_hours.length})</button>
        <button className={tab === 'tareas' ? styles.subTabActive : styles.subTab} onClick={() => setTab('tareas')}>Tareas ({tasks.length})</button>
        <button className={tab === 'seguimiento' ? styles.subTabActive : styles.subTab} onClick={() => setTab('seguimiento')}>Seguimiento</button>
        <button className={tab === 'soporte' ? styles.subTabActive : styles.subTab} onClick={() => setTab('soporte')}>Soporte</button>
        <button className={tab === 'actividad' ? styles.subTabActive : styles.subTab} onClick={() => setTab('actividad')}>Actividad ({actTotal})</button>
      </div>

      {/* Expediente laboral del empleo en ESTA empresa: resumen, ausencias,
          gestiones, contactos, notas/evaluaciones y documentos. Es el mismo
          componente que usan la ficha global y la del empleador, incrustado en
          vez de en modal: si se maquetara aparte, las tres se irían separando. */}
      {tab === 'expediente' && (
        employment ? (
          <ExpedienteModal
            inline
            userId={employeeId}
            employment={employment}
            canManage={canManage}
            onClose={() => {}}
          />
        ) : (
          <div className={styles.empty}>
            <FileText size={40} />
            <p>Sin empleo registrado en esta empresa</p>
            <span className={styles.emptyHint}>
              El expediente cuelga del empleo. Este profesional aparece en la plantilla
              pero no tiene una membresía registrada aquí, así que no hay expediente que abrir.
            </span>
          </div>
        )
      )}

      {tab === 'jornadas' && (
        work_hours.length === 0 ? (
          <div className={styles.empty}><Clock size={40} /><p>Sin jornadas registradas</p></div>
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr><th>Fecha</th><th>Tipo</th><th>Horas</th><th>Estado</th><th>Actividades</th><th></th></tr>
              </thead>
              <tbody>
                {work_hours.map(wh => {
                  // Las actividades se escriben en un editor enriquecido y se
                  // guardan como HTML; en una celda hay que aplanarlas o se lee
                  // el markup en vez del texto.
                  const activities = htmlToText(wh.activities)
                  return (
                  <tr key={wh.id} className={styles.row} onClick={() => setDetailId(wh.id)} title="Ver el detalle de la jornada">
                    <td>{wh.work_date ? new Date(wh.work_date).toLocaleDateString('es-ES') : '—'}</td>
                    <td><span className={styles.typeBadge}>{WORK_TYPE[wh.work_type] || wh.work_type}</span></td>
                    <td>{wh.hours_worked?.toFixed(1)} h</td>
                    <td>
                      <span className={`${styles.badge} ${getWorkHourStatus(wh).className}`}>
                        {getWorkHourStatus(wh).label}
                      </span>
                    </td>
                    {/* Truncada en la celda, entera en el modal: la columna no
                        puede crecer, y el texto de una jornada suele ser un
                        parte de varias líneas. */}
                    <td className={styles.truncate} title={activities || undefined}>{activities || '—'}</td>
                    <td>
                      <div className={styles.rowActions}>
                        <button
                          className={styles.iconBtn}
                          onClick={(e) => { e.stopPropagation(); setDetailId(wh.id) }}
                          title="Ver el detalle de la jornada"
                          aria-label={`Ver el detalle de la jornada del ${wh.work_date ? new Date(wh.work_date).toLocaleDateString('es-ES') : 'registro'}`}
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
          </div>
        )
      )}

      {tab === 'tareas' && (
        tasks.length === 0 ? (
          <div className={styles.empty}><CheckSquare size={40} /><p>Sin tareas asignadas</p></div>
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr><th>Tarea</th><th>Tablero</th><th>Estado</th><th>Vencimiento</th></tr>
              </thead>
              <tbody>
                {tasks.map(t => (
                  <tr key={t.id}>
                    <td>{t.title}</td>
                    <td>{t.board_name || '—'}</td>
                    <td>
                      <span className={`${styles.badge} ${t.completed ? styles.badgeActive : styles.badgePending}`}>
                        {t.completed ? 'Finalizada' : t.status}
                      </span>
                    </td>
                    <td>{t.end_date ? formatDateOnly(t.end_date) : '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )
      )}

      {tab === 'seguimiento' && (
        <EmployeeSeguimiento
          userId={employeeId}
          employmentId={employment?.id}
          isProfessional={user.user_type === 'profesional'}
          canManage={canManage}
        />
      )}

      {tab === 'soporte' && <EmployeeSoporte userId={employeeId} />}

      {/* Lo que esta persona ha hecho constar en el expediente de su empresa:
          su incorporación, sus jornadas, las gestiones y notas que la nombran.
          Es de solo lectura; se escribe desde el expediente de la empresa. */}
      {tab === 'actividad' && (
        actLoading ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.6rem' }}>
            {Array.from({ length: 4 }).map((_, i) => <Skeleton key={i} height={56} radius={12} />)}
          </div>
        ) : actError ? (
          <p className={styles.errorMsg}>{actError}</p>
        ) : activity.length === 0 ? (
          <div className={styles.empty}>
            <Activity size={40} />
            <p>Sin movimientos en el expediente</p>
            <span className={styles.emptyHint}>
              {seniority
                ? `Se dio de alta ${seniority} y todavía no ha dejado rastro en el expediente de la empresa.`
                : 'Aquí aparecerán sus jornadas, su incorporación y las gestiones que le mencionen.'}
            </span>
          </div>
        ) : (
          <>
            {/* Mientras llega la página nueva se atenúa la anterior: se ve que
                algo está pasando sin vaciar la lista. */}
            <div className={styles.timeline} style={{ opacity: actFetching ? 0.6 : 1, transition: 'opacity 0.15s ease' }}>
              {groupByDay(activity).map(group => (
                <div key={group.key} className={styles.timelineDay}>
                  <div className={styles.timelineDayLabel}>
                    <span className={styles.timelineDayChip}>
                      {group.label}
                      <span className={styles.timelineDayCount}>
                        {group.items.length} {group.items.length === 1 ? 'movimiento' : 'movimientos'}
                      </span>
                    </span>
                  </div>
                  {group.items.map((a, i) => {
                    const date = new Date(a.timestamp)
                    const valid = !isNaN(date.getTime())
                    const st = ACTIVITY_STYLE[a.type] || ACTIVITY_FALLBACK
                    const Icon = st.icon
                    return (
                      <div key={a.event_id ? `ev-${a.event_id}` : `act-${actPage}-${group.key}-${i}`} className={styles.timelineItem}>
                        <div className={styles.timelineNode} style={{ color: st.color }}>
                          <Icon size={15} />
                        </div>
                        <div className={styles.timelineCard}>
                          <div style={{ flex: 1, minWidth: 0 }}>
                            <p className={styles.timelineText}>{a.details}</p>
                            <div className={styles.timelineMeta}>
                              <span className={styles.timelineTag} style={{ color: st.color }}>
                                {ACTIVITY_LABEL[a.type] || a.type}
                              </span>
                              {a.channel && CONTACT_STYLE[a.channel] && (
                                <span className={styles.timelineTag} style={{ color: st.color, display: 'inline-flex', alignItems: 'center', gap: 4 }}>
                                  {(() => { const CI = CONTACT_STYLE[a.channel!].icon; return <CI size={11} /> })()}
                                  {CONTACT_STYLE[a.channel].label}
                                </span>
                              )}
                              {/* La fecha vive en la cabecera del día: aquí
                                  basta la hora. */}
                              <span className={styles.timelineTime}>
                                {valid ? date.toLocaleTimeString('es-ES', { hour: '2-digit', minute: '2-digit' }) : '—'}
                              </span>
                            </div>
                          </div>
                        </div>
                      </div>
                    )
                  })}
                </div>
              ))}
            </div>

            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', padding: '14px 4px' }}>
              <span style={{ fontSize: '13px', color: '#64748b' }}>
                Mostrando {(actPage - 1) * ACTIVITY_PER_PAGE + 1}–{Math.min(actPage * ACTIVITY_PER_PAGE, actTotal)} de {actTotal} movimientos
              </span>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <button
                  type="button"
                  className={styles.iconBtn}
                  onClick={() => setActPage(p => Math.max(1, p - 1))}
                  disabled={actPage <= 1}
                  style={{ opacity: actPage <= 1 ? 0.4 : 1, cursor: actPage <= 1 ? 'not-allowed' : 'pointer' }}
                  title="Página anterior"
                >
                  <ChevronLeft size={16} />
                </button>
                <span style={{ fontSize: '13px', fontWeight: 600, color: '#334155', whiteSpace: 'nowrap' }}>
                  Página {actPage} de {actTotalPages}
                </span>
                <button
                  type="button"
                  className={styles.iconBtn}
                  onClick={() => setActPage(p => Math.min(actTotalPages, p + 1))}
                  disabled={actPage >= actTotalPages}
                  style={{ opacity: actPage >= actTotalPages ? 0.4 : 1, cursor: actPage >= actTotalPages ? 'not-allowed' : 'pointer' }}
                  title="Página siguiente"
                >
                  <ChevronRight size={16} />
                </button>
              </div>
            </div>
          </>
        )
      )}

      {/* Detalle de una jornada. Es el MISMO modal del módulo de Horas, en modo
          lectura: aprobar o rechazar se hace desde allí, con la cola entera
          delante, y no de una en una desde la ficha de alguien. */}
      <WorkHourDetailModal
        workHour={detailRecord}
        onClose={() => setDetailId(null)}
        canApprove={false}
        canEdit={false}
        onApprove={async () => {}}
        onReject={async () => {}}
        onEdit={() => {}}
      />
    </div>
  )
}
