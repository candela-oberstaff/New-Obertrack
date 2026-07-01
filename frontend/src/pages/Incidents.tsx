import { useState, useEffect, useMemo } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Siren,
  Plus,
  MapPin,
  Calendar,
  Send,
  Mail,
  MessageCircle,
  MessageSquare,
  X,
  Lock,
} from 'lucide-react'
import {
  adminService,
  type Incident,
  type IncidentProfessional,
  type IncidentStatus,
} from '../services/admin.service'
import { COUNTRY_OPTIONS, getStatesForCountry } from '../components/Auth/countries'
import { Select } from '../components/ui/Select'
import { buildTemplateOptions } from '../lib/emergencyTemplates'
import { EmergencyTemplatesModal } from '../components/Admin/EmergencyTemplatesModal'
import styles from './Incidents.module.css'

const KIND_OPTIONS = [
  { value: 'Sismo', label: 'Sismo' },
  { value: 'Eventos conflictivos', label: 'Eventos conflictivos' },
  { value: 'Caídas de internet', label: 'Caídas de internet' },
  { value: 'Apagones', label: 'Apagones' },
  { value: 'Incendios forestales', label: 'Incendios forestales' },
  { value: 'Tormentas', label: 'Tormentas' },
  { value: 'Inundaciones', label: 'Inundaciones' },
  { value: 'Volcanes', label: 'Volcanes' },
  { value: 'Sequía', label: 'Sequía' },
  { value: 'UAP', label: 'UAP' },
  { value: 'Otros', label: 'Otros' },
]

const STATUS_OPTIONS: { value: IncidentStatus; label: string }[] = [
  { value: 'pendiente', label: 'Pendiente' },
  { value: 'contactado', label: 'Contactado' },
  { value: 'ok', label: 'OK' },
  { value: 'sin_respuesta', label: 'Sin respuesta' },
]

const STATUS_COLOR: Record<IncidentStatus, string> = {
  ok: '#16a34a',
  contactado: '#d97706',
  sin_respuesta: '#dc2626',
  pendiente: '#94a3b8',
}

const whatsappHref = (phone: string) => {
  const digits = (phone || '').replace(/\D/g, '')
  return digits ? `https://wa.me/${digits}` : null
}

const mailtoHref = (email: string) => (email ? `mailto:${email}` : null)

const fmtDate = (iso: string | null) =>
  iso ? new Date(iso).toLocaleDateString('es', { day: '2-digit', month: 'short', year: 'numeric' }) : '—'

const isOpen = (i: Incident) => !i.closed_at && i.status !== 'closed'

export default function Incidents() {
  const qc = useQueryClient()
  const [createOpen, setCreateOpen] = useState(false)
  const [createInitial, setCreateInitial] = useState<{ country?: string; state?: string; title?: string }>({})
  const [detailId, setDetailId] = useState<number | null>(null)
  const [searchParams, setSearchParams] = useSearchParams()

  useEffect(() => {
    if (searchParams.get('create') !== '1') return
    const country = searchParams.get('country') || ''
    const state = searchParams.get('state') || ''
    const zone = [state, country].filter(Boolean).join(', ')
    setCreateInitial({ country, state, title: zone ? `Incidente en ${zone}` : '' })
    setCreateOpen(true)
    setSearchParams({}, { replace: true })
  }, [searchParams, setSearchParams])

  const { data, isLoading, isError } = useQuery({
    queryKey: ['incidents'],
    queryFn: () => adminService.getIncidents(),
  })

  const incidents = data?.incidents ?? []

  return (
    <div className={styles['page']}>
      <div className={styles['header']}>
        <h1>
          <Siren size={22} /> Incidentes
        </h1>
        <button type="button" className={styles['btn']} onClick={() => { setCreateInitial({}); setCreateOpen(true) }}>
          <Plus size={16} /> Crear incidente
        </button>
      </div>

      {isLoading && <div className={styles['state-msg']}>Cargando…</div>}
      {isError && <div className={styles['state-msg']}>Error al cargar los incidentes.</div>}

      {!isLoading && !isError && incidents.length === 0 && (
        <div className={styles['state-msg']}>No hay incidentes registrados.</div>
      )}

      <div className={styles['grid']}>
        {incidents.map((inc) => (
          <IncidentCard key={inc.id} incident={inc} onOpen={() => setDetailId(inc.id)} />
        ))}
      </div>

      {createOpen && (
        <CreateIncidentModal
          initial={createInitial}
          onClose={() => { setCreateOpen(false); setCreateInitial({}) }}
          onCreated={() => {
            setCreateOpen(false)
            setCreateInitial({})
            qc.invalidateQueries({ queryKey: ['incidents'] })
          }}
        />
      )}

      {detailId !== null && (
        <IncidentDetailModal id={detailId} onClose={() => setDetailId(null)} />
      )}
    </div>
  )
}

function IncidentCard({ incident, onOpen }: { incident: Incident; onOpen: () => void }) {
  const open = isOpen(incident)
  const { affected, ok, contactado, sin_respuesta, pendiente } = incident.counts
  const pct = affected ? Math.round((ok / affected) * 100) : 0
  return (
    <div className={styles['card']} onClick={onOpen}>
      <div className={styles['card-head']}>
        <h3 className={styles['card-title']}>{incident.title}</h3>
        <span className={`${styles['badge']} ${styles['badge-kind']}`}>{incident.kind}</span>
        <span className={`${styles['badge']} ${open ? styles['badge-open'] : styles['badge-closed']}`}>
          {open ? 'Abierto' : 'Cerrado'}
        </span>
      </div>
      <div className={styles['card-meta']}>
        <span>
          <MapPin size={13} /> {[incident.country, incident.state].filter(Boolean).join(' / ') || 'Sin zona'}
        </span>
        <span>
          <Calendar size={13} /> {fmtDate(incident.created_at)}
        </span>
      </div>
      <div className={styles['counts']}>
        <span className={styles['count-chip']}>{affected} afectados</span>
        <span className={styles['count-chip']} style={{ color: STATUS_COLOR.ok }}>✅ {ok}</span>
        <span className={styles['count-chip']} style={{ color: STATUS_COLOR.contactado }}>🟡 {contactado}</span>
        <span className={styles['count-chip']} style={{ color: STATUS_COLOR.sin_respuesta }}>🔴 {sin_respuesta}</span>
        <span className={styles['count-chip']} style={{ color: STATUS_COLOR.pendiente }}>⚪ {pendiente}</span>
      </div>
      <div className={styles['progress-label']}>{ok}/{affected} confirmados OK</div>
      <div className={styles['progress-bar']}>
        <div className={styles['progress-fill']} style={{ width: `${pct}%` }} />
      </div>
    </div>
  )
}

function CreateIncidentModal({ onClose, onCreated, initial }: { onClose: () => void; onCreated: () => void; initial?: { country?: string; state?: string; title?: string } }) {
  const [title, setTitle] = useState(initial?.title ?? '')
  const [kind, setKind] = useState('Sismo')
  const [country, setCountry] = useState(initial?.country ?? '')
  const [state, setState] = useState(initial?.state ?? '')
  const [description, setDescription] = useState('')

  const stateOptions = [{ value: '', label: 'Sin estado / provincia' }, ...getStatesForCountry(country)]

  const mutation = useMutation({
    mutationFn: () => adminService.createIncident({ title, kind, country, state, description }),
    onSuccess: onCreated,
  })

  return (
    <div className={styles['modal-overlay']} onClick={onClose}>
      <div className={styles['modal']} onClick={(e) => e.stopPropagation()}>
        <div className={styles['modal-head']}>
          <h3><Siren size={18} /> Crear incidente</h3>
          <button type="button" className={styles['icon-btn']} onClick={onClose}><X size={18} /></button>
        </div>
        <div className={styles['modal-body']}>
          <label className={styles['field-label']}>Título</label>
          <input className={styles['field-input']} value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Ej: Sismo en Caracas" />

          <label className={styles['field-label']}>Tipo</label>
          <Select fullWidth value={kind} onChange={(v) => setKind(String(v))} options={KIND_OPTIONS} />

          <label className={styles['field-label']}>País</label>
          <Select
            fullWidth
            value={country}
            onChange={(v) => { setCountry(String(v)); setState('') }}
            placeholder="Seleccionar país"
            options={[{ value: '', label: 'Seleccionar país' }, ...COUNTRY_OPTIONS]}
          />

          <label className={styles['field-label']}>Provincia / Estado</label>
          <Select
            fullWidth
            value={state}
            onChange={(v) => setState(String(v))}
            options={stateOptions}
            disabled={!country || stateOptions.length <= 1}
          />

          <label className={styles['field-label']}>Descripción</label>
          <textarea className={styles['field-textarea']} rows={4} value={description} onChange={(e) => setDescription(e.target.value)} />

          {mutation.isError && <p className={styles['result-fail']}>No se pudo crear el incidente.</p>}

          <div className={styles['modal-actions']}>
            <button type="button" className={styles['btn-ghost']} onClick={onClose}>Cancelar</button>
            <button
              type="button"
              className={styles['btn']}
              onClick={() => mutation.mutate()}
              disabled={mutation.isPending || !title.trim() || !country}
            >
              {mutation.isPending ? 'Creando…' : 'Crear incidente'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

function IncidentDetailModal({ id, onClose }: { id: number; onClose: () => void }) {
  const qc = useQueryClient()
  const navigate = useNavigate()
  const [broadcastOpen, setBroadcastOpen] = useState(false)

  const { data, isLoading, isError } = useQuery({
    queryKey: ['incident', id],
    queryFn: () => adminService.getIncident(id),
  })

  const incident = data?.incident
  const professionals = data?.professionals ?? []
  const open = incident ? isOpen(incident) : false

  const okCount = professionals.filter((p) => p.status === 'ok').length
  const pct = professionals.length ? Math.round((okCount / professionals.length) * 100) : 0

  const statusMutation = useMutation({
    mutationFn: ({ userId, status }: { userId: number; status: IncidentStatus }) =>
      adminService.setIncidentResponse(id, userId, { status }),
    onMutate: async ({ userId, status }) => {
      await qc.cancelQueries({ queryKey: ['incident', id] })
      const prev = qc.getQueryData(['incident', id])
      qc.setQueryData(['incident', id], (old: typeof data) =>
        old
          ? { ...old, professionals: old.professionals.map((p) => (p.id === userId ? { ...p, status } : p)) }
          : old,
      )
      return { prev }
    },
    onError: (_e, _v, ctx) => {
      if (ctx?.prev) qc.setQueryData(['incident', id], ctx.prev)
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: ['incident', id] })
      qc.invalidateQueries({ queryKey: ['incidents'] })
    },
  })

  const closeMutation = useMutation({
    mutationFn: () => adminService.closeIncident(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['incident', id] })
      qc.invalidateQueries({ queryKey: ['incidents'] })
    },
  })

  const contactButtons = (p: IncidentProfessional) => {
    const wa = whatsappHref(p.phone_number)
    const mail = mailtoHref(p.email)
    return (
      <div className={styles['contact-actions']}>
        {mail && (
          <a className={styles['icon-btn']} href={mail} onClick={() => adminService.logContact(p.id, 'email')} title="Email">
            <Mail size={15} />
          </a>
        )}
        {wa && (
          <a className={styles['icon-btn']} href={wa} target="_blank" rel="noreferrer" onClick={() => adminService.logContact(p.id, 'whatsapp')} title="WhatsApp" style={{ color: '#25d366' }}>
            <MessageCircle size={15} />
          </a>
        )}
        <button
          type="button"
          className={styles['icon-btn']}
          onClick={() => { adminService.logContact(p.id, 'chat'); navigate(`/chat?userId=${p.id}`) }}
          title="Chat interno"
          style={{ color: '#7c3aed' }}
        >
          <MessageSquare size={15} />
        </button>
      </div>
    )
  }

  return (
    <div className={styles['modal-overlay']} onClick={onClose}>
      <div className={`${styles['modal']} ${styles['modal-lg']}`} onClick={(e) => e.stopPropagation()}>
        <div className={styles['modal-head']}>
          <h3>
            <Siren size={18} /> {incident?.title ?? 'Incidente'}
            {incident && (
              <span className={`${styles['badge']} ${open ? styles['badge-open'] : styles['badge-closed']}`}>
                {open ? 'Abierto' : 'Cerrado'}
              </span>
            )}
          </h3>
          <button type="button" className={styles['icon-btn']} onClick={onClose}><X size={18} /></button>
        </div>

        {isLoading && <div className={styles['state-msg']}>Cargando…</div>}
        {isError && <div className={styles['state-msg']}>Error al cargar el incidente.</div>}

        {incident && (
          <>
            <div className={styles['card-meta']}>
              <span><MapPin size={13} /> {[incident.country, incident.state].filter(Boolean).join(' / ') || 'Sin zona'}</span>
              <span><Calendar size={13} /> {fmtDate(incident.created_at)}</span>
              <span className={`${styles['badge']} ${styles['badge-kind']}`}>{incident.kind}</span>
            </div>

            <div className={styles['detail-bar']}>
              <div className={styles['detail-progress']}>
                <div className={styles['progress-label']}>{okCount}/{professionals.length} confirmados OK</div>
                <div className={styles['progress-bar']}>
                  <div className={styles['progress-fill']} style={{ width: `${pct}%` }} />
                </div>
              </div>
              <button type="button" className={styles['btn']} onClick={() => setBroadcastOpen(true)} disabled={professionals.length === 0}>
                <Send size={15} /> Broadcast
              </button>
              <button
                type="button"
                className={styles['btn']}
                onClick={() => {
                  const params = new URLSearchParams()
                  if (incident.country) params.set('country', incident.country)
                  if (incident.state) params.set('state', incident.state)
                  navigate(`/admin/mapa?${params.toString()}`)
                }}
              >
                <MapPin size={15} /> Ver en el mapa
              </button>
              {open && (
                <button
                  type="button"
                  className={`${styles['btn']} ${styles['btn-danger']}`}
                  onClick={() => closeMutation.mutate()}
                  disabled={closeMutation.isPending}
                >
                  <Lock size={15} /> {closeMutation.isPending ? 'Cerrando…' : 'Cerrar incidente'}
                </button>
              )}
            </div>

            <div className={styles['table-wrap']}>
              <table className={styles['table']}>
                <thead>
                  <tr>
                    <th>Profesional</th>
                    <th>Empresa</th>
                    <th>Ciudad</th>
                    <th>Estado</th>
                    <th>Contacto</th>
                  </tr>
                </thead>
                <tbody>
                  {professionals.length === 0 && (
                    <tr><td colSpan={5} className={styles['state-msg']}>Sin profesionales afectados.</td></tr>
                  )}
                  {professionals.map((p) => (
                    <tr key={p.id}>
                      <td>
                        <span className={styles['cell-name']}>{p.name}</span>
                        {!p.is_active && <span className={styles['inactive-tag']}>Inactivo</span>}
                        <div className={styles['cell-meta']}>{p.email}</div>
                      </td>
                      <td>{p.company || '—'}</td>
                      <td>{p.city || '—'}</td>
                      <td>
                        <select
                          className={styles['status-select']}
                          value={p.status}
                          onChange={(e) => statusMutation.mutate({ userId: p.id, status: e.target.value as IncidentStatus })}
                          style={{ color: STATUS_COLOR[p.status] }}
                        >
                          {STATUS_OPTIONS.map((o) => (
                            <option key={o.value} value={o.value}>{o.label}</option>
                          ))}
                        </select>
                      </td>
                      <td>{contactButtons(p)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </>
        )}

        {broadcastOpen && (
          <BroadcastModal id={id} onClose={() => setBroadcastOpen(false)} />
        )}
      </div>
    </div>
  )
}

function BroadcastModal({ id, onClose }: { id: number; onClose: () => void }) {
  const { data: tplData } = useQuery({
    queryKey: ['emergency-templates'],
    queryFn: adminService.getEmergencyTemplates,
  })
  const templateOptions = useMemo(() => buildTemplateOptions(tplData?.templates ?? []), [tplData])

  const [templateValue, setTemplateValue] = useState(templateOptions[0]?.value ?? '')
  const [subject, setSubject] = useState(templateOptions[0]?.subject ?? '')
  const [body, setBody] = useState(templateOptions[0]?.body ?? '')
  const [manageOpen, setManageOpen] = useState(false)

  const applyTemplate = (value: string) => {
    setTemplateValue(value)
    const opt = templateOptions.find((o) => o.value === value)
    if (opt) {
      setSubject(opt.subject)
      setBody(opt.body)
    }
  }

  const mutation = useMutation({
    mutationFn: () => adminService.broadcastIncident(id, { subject, body }),
  })
  const result = mutation.data

  return (
    <>
    <div className={styles['modal-overlay']} onClick={onClose}>
      <div className={styles['modal']} onClick={(e) => e.stopPropagation()}>
        <div className={styles['modal-head']}>
          <h3><Send size={18} /> Broadcast a afectados</h3>
          <button type="button" className={styles['icon-btn']} onClick={onClose}><X size={18} /></button>
        </div>

        {result ? (
          <div className={styles['modal-body']}>
            <p className={styles['result-ok']}>Enviados: {result.sent}</p>
            {result.failed.length > 0 && (
              <div className={styles['result-fail']}>
                <strong>Fallidos ({result.failed.length}):</strong>
                <ul>
                  {result.failed.map((f) => (
                    <li key={f.id}>{f.email} — {f.error}</li>
                  ))}
                </ul>
              </div>
            )}
            <div className={styles['modal-actions']}>
              <button type="button" className={styles['btn']} onClick={onClose}>Cerrar</button>
            </div>
          </div>
        ) : (
          <div className={styles['modal-body']}>
            <div className={styles['field-row']}>
              <label className={styles['field-label']}>Plantilla</label>
              <button type="button" className={styles['link-btn']} onClick={() => setManageOpen(true)}>Gestionar plantillas</button>
            </div>
            <select className={styles['field-select']} value={templateValue} onChange={(e) => applyTemplate(e.target.value)}>
              {templateOptions.map((o) => (
                <option key={o.value} value={o.value}>{o.label}</option>
              ))}
            </select>

            <label className={styles['field-label']}>Asunto</label>
            <input className={styles['field-input']} value={subject} onChange={(e) => setSubject(e.target.value)} />

            <label className={styles['field-label']}>Mensaje</label>
            <textarea className={styles['field-textarea']} rows={6} value={body} onChange={(e) => setBody(e.target.value)} />

            {mutation.isError && <p className={styles['result-fail']}>No se pudo enviar el broadcast.</p>}

            <div className={styles['modal-actions']}>
              <button type="button" className={styles['btn-ghost']} onClick={onClose}>Cancelar</button>
              <button
                type="button"
                className={styles['btn']}
                onClick={() => mutation.mutate()}
                disabled={mutation.isPending || !subject.trim() || !body.trim()}
              >
                <Send size={15} /> {mutation.isPending ? 'Enviando…' : 'Enviar'}
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
