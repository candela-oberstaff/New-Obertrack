import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Inbox, AlertTriangle, ChevronRight } from 'lucide-react'
import { adminService } from '../../services/api'
import { Skeleton } from '../../components/ui'
import { ticketOrigin, TICKET_STAGE, ticketPath } from './ticketStyle'
import styles from './Tenants.module.css'

// Cómo respondió el profesional a una incidencia. 'pendiente' es el estado con
// el que nace toda respuesta al difundir el aviso: no es que no le importe, es
// que todavía nadie ha registrado nada por él.
const RESPONSE_PILL: Record<string, { label: string; className: string }> = {
  pendiente: { label: 'Pendiente', className: styles.pillWarn },
  contactado: { label: 'Contactado', className: styles.pillInfo },
  ok: { label: 'Sin novedad', className: styles.pillSuccess },
  sin_respuesta: { label: 'Sin respuesta', className: styles.pillDanger },
}

const INCIDENT_KIND: Record<string, string> = {
  sismo: 'Sismo',
  huracan: 'Huracán',
  apagon: 'Apagón',
  inundacion: 'Inundación',
  otro: 'Otro',
}

/**
 * Soporte de un profesional: los tickets que van sobre él y las incidencias en
 * las que se le incluyó. Juntos porque son las dos formas en que esta persona
 * puede estar necesitando algo del equipo ahora mismo.
 */
export function EmployeeSoporte({ userId }: { userId: number }) {
  const navigate = useNavigate()

  const { data: tickets = [], isLoading: ticketsLoading, error: ticketsError } = useQuery({
    queryKey: ['employee-tickets', userId],
    queryFn: () => adminService.getEmployeeTickets(userId),
    enabled: !!userId,
  })

  const { data: incidents = [], isLoading: incLoading, error: incError } = useQuery({
    queryKey: ['employee-incidents', userId],
    queryFn: () => adminService.getEmployeeIncidents(userId),
    enabled: !!userId,
  })

  return (
    <div className={styles.stackSections}>
      <section>
        <h2 className={styles.sectionTitle}>
          Tickets
          {tickets.length > 0 && <span className={styles.sectionCount}>({tickets.length})</span>}
        </h2>
        {ticketsLoading ? (
          <div className={styles.stack}>
            {Array.from({ length: 3 }).map((_, i) => <Skeleton key={i} height={52} radius={12} />)}
          </div>
        ) : ticketsError ? (
          <p className={styles.errorMsg}>No se pudieron cargar los tickets de este profesional.</p>
        ) : tickets.length === 0 ? (
          <div className={styles.empty}>
            <Inbox size={40} />
            <p>Sin tickets</p>
            <span className={styles.emptyHint}>
              Aquí aparecen las alertas que la plataforma abre sobre esta persona
              (por ejemplo, un rechazo de horas) y las conversaciones que se le asignen.
            </span>
          </div>
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr><th>Ticket</th><th>Origen</th><th>Responsable</th><th>Últ. movimiento</th><th>Estado</th><th></th></tr>
              </thead>
              <tbody>
                {tickets.map(tk => {
                  const st = ticketOrigin(tk.origin)
                  const OriginIcon = st.icon
                  const updated = new Date(tk.updated_at)
                  const isOpen = tk.status === 'open'
                  return (
                    <tr key={tk.id} className={styles.row} onClick={() => navigate(ticketPath(tk))}>
                      <td>
                        <div className={styles.ownerCell}>
                          <span>{tk.title?.trim() || `Ticket #${tk.id}`}</span>
                          <small>#{tk.id}{tk.stage ? ` · ${TICKET_STAGE[tk.stage] || tk.stage}` : ''}</small>
                        </div>
                      </td>
                      <td>
                        <span className={styles.timelineTag} style={{ color: st.color }}>
                          <OriginIcon size={13} /> {st.label}
                        </span>
                      </td>
                      <td>{tk.assignee?.trim() || <span className={styles.fieldEmpty}>Sin asignar</span>}</td>
                      <td>{isNaN(updated.getTime()) ? '—' : updated.toLocaleDateString('es-ES')}</td>
                      <td>
                        <span className={`${styles.badge} ${isOpen ? styles.badgeActive : styles.badgeSuspended}`}>
                          {isOpen ? 'Abierto' : 'Cerrado'}
                        </span>
                      </td>
                      <td><div className={styles.rowActions}><ChevronRight size={18} className={styles.chevron} /></div></td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section>
        <h2 className={styles.sectionTitle}>
          Incidencias
          {incidents.length > 0 && <span className={styles.sectionCount}>({incidents.length})</span>}
        </h2>
        {incLoading ? (
          <div className={styles.stack}>
            {Array.from({ length: 2 }).map((_, i) => <Skeleton key={i} height={52} radius={12} />)}
          </div>
        ) : incError ? (
          <p className={styles.errorMsg}>No se pudieron cargar las incidencias de este profesional.</p>
        ) : incidents.length === 0 ? (
          <div className={styles.empty}>
            <AlertTriangle size={40} />
            <p>Sin incidencias</p>
            <span className={styles.emptyHint}>
              Aparecerán las emergencias de su zona en las que se le haya incluido,
              junto con lo que se registró al contactarle.
            </span>
          </div>
        ) : (
          <div className={styles.stack}>
            {incidents.map(inc => {
              const resp = RESPONSE_PILL[inc.response_status] ?? { label: inc.response_status || '—', className: styles.pillNeutral }
              const opened = new Date(inc.created_at)
              const place = [inc.state, inc.country].filter(Boolean).join(', ')
              // Sin enlace al detalle: el módulo de incidencias es una lista sin
              // pantalla propia por incidencia, y llevar allí desde una ficha
              // individual perdería a la persona de vista.
              return (
                <div key={inc.id} className={styles.entry}>
                  <div className={styles.entryHead}>
                    <span className={styles.entryTitle}>{inc.title}</span>
                    {inc.kind && <span className={styles.typeBadge}>{INCIDENT_KIND[inc.kind] || inc.kind}</span>}
                    <span className={`${styles.badge} ${inc.status === 'open' ? styles.badgeActive : styles.badgeSuspended}`}>
                      {inc.status === 'open' ? 'Abierta' : 'Cerrada'}
                    </span>
                    {/* Cómo quedó ESTA persona: es el dato por el que se mira una
                        incidencia desde una ficha individual. */}
                    <span className={`${styles.pill} ${resp.className}`}>{resp.label}</span>
                    <span className={styles.entryMeta}>
                      {place && `${place} · `}{isNaN(opened.getTime()) ? '—' : opened.toLocaleDateString('es-ES')}
                    </span>
                  </div>
                  {inc.response_note && <p className={styles.entryBody}>{inc.response_note}</p>}
                </div>
              )
            })}
          </div>
        )}
      </section>
    </div>
  )
}
