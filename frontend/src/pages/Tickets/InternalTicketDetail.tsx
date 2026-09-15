import { useCallback, useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, RefreshCw, Mail, Phone, Building2, User as UserIcon, UserX, Calendar, FileText, Send, CheckCircle2, ArrowRightLeft, History, GraduationCap, ExternalLink, MessageSquare } from 'lucide-react'
import { inductionService } from '../../services/induction.service'
import { Ticket, TicketTransfer, SupportAgent, ticketService } from '../../services/ticket.service'
import TransferTicketModal from './components/TransferTicketModal'
import styles from './Tickets.module.css'
import { useAuth } from '../../context/AuthContext'
import { canEditModule, isSupportManager } from '../../lib/permissions'

const STAGE_OPTIONS: { id: string; label: string }[] = [
  { id: 'new', label: 'Nuevo' },
  { id: 'in_progress', label: 'En seguimiento' },
  { id: 'closed', label: 'Resuelto' },
]
const STAGE_LABEL: Record<string, string> = { new: 'Nuevo', in_progress: 'En seguimiento', waiting: 'En seguimiento', closed: 'Resuelto' }

// El título lleva el tipo de aviso delante; para el nombre sobra.
function professionalName(t: Ticket): string {
  return t.title?.replace(/^(Rechazo de horas|Alta desde Obersuite|Inducción no aprobada):\s*/i, '') || 'Profesional'
}

// ─── WaChatTranscript ────────────────────────────────────────────────────────
// Renderiza el historial de chat embebido en description de un ticket de
// transferencia WA. El texto tiene la forma:
//
//   "Chat de WhatsApp transferido…\n\nMotivo: …\n\nHistorial del chat:\n<líneas>"
//
// Se intenta extraer solo la parte del historial; si no se encuentra la
// separación conocida se muestra la descripción completa.
function WaChatTranscript({ description }: { description: string }) {
  const HISTORIAL_MARKER = 'Historial del chat:'
  const MOTIVO_MARKER = 'Motivo:'

  const historialIdx = description.indexOf(HISTORIAL_MARKER)
  const motivoIdx = description.indexOf(MOTIVO_MARKER)

  const preamble = description.slice(0, motivoIdx > 0 ? motivoIdx : (historialIdx > 0 ? historialIdx : description.length)).trim()
  const motivo = motivoIdx > 0
    ? description.slice(motivoIdx + MOTIVO_MARKER.length, historialIdx > 0 ? historialIdx : undefined).trim()
    : ''
  const historial = historialIdx > 0
    ? description.slice(historialIdx + HISTORIAL_MARKER.length).trim()
    : ''

  // Cada línea del historial tiene formato "[HH:MM] Nombre: mensaje" o similar.
  // Se intenta colorear por sentido: líneas del candidato vs. del manager.
  // Si no se puede parsear, se muestra como texto plano.
  const lines = historial ? historial.split('\n').filter(l => l.trim()) : []

  // Heurística simple: la primera palabra que aparece como "Nombre:" distinta
  // al manager se asume contacto. No es perfecta pero mejora la lectura.
  const agentNames = new Set<string>()
  const contactNames = new Set<string>()
  // El preamble dice "transferido por X": ese es el manager/agente.
  const byMatch = preamble.match(/transferido (?:desde Obersuite )?por ([^.]+)\./i)
  if (byMatch) {
    byMatch[1].trim().split(/\s+/).slice(0, 2).forEach(w => agentNames.add(w.toLowerCase()))
  }

  function classifyLine(line: string): 'agent' | 'contact' | 'system' {
    const colonIdx = line.indexOf(':')
    if (colonIdx < 0 || colonIdx > 40) return 'system'
    const speaker = line.slice(0, colonIdx).replace(/^\[\d{1,2}:\d{2}(?::\d{2})?\]\s*/, '').trim().toLowerCase()
    if (!speaker) return 'system'
    for (const n of agentNames) { if (speaker.includes(n)) return 'agent' }
    for (const n of contactNames) { if (speaker.includes(n)) return 'contact' }
    // Primer hablante desconocido → contacto; segundo → agente (heurística).
    if (contactNames.size === 0) { contactNames.add(speaker); return 'contact' }
    agentNames.add(speaker)
    return 'agent'
  }

  return (
    <div style={{ borderBottom: '1px solid var(--glass-border, #e2e8f0)', padding: '1.25rem' }}>
      <h3 style={{ fontSize: '0.9rem', fontWeight: 700, margin: '0 0 0.75rem', display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
        <MessageSquare size={15} style={{ color: '#25d366' }} />
        Historial del chat transferido
      </h3>

      {/* Preamble: quién transfirió y candidato */}
      <p style={{ fontSize: '0.82rem', color: 'var(--text-secondary)', margin: '0 0 0.5rem', lineHeight: 1.4 }}>{preamble}</p>

      {/* Motivo */}
      {motivo && (
        <div style={{
          background: 'rgba(245,158,11,0.08)',
          border: '1px solid rgba(245,158,11,0.2)',
          borderRadius: '8px',
          padding: '0.5rem 0.75rem',
          fontSize: '0.85rem',
          color: '#92400e',
          marginBottom: '0.75rem',
        }}>
          <strong>Motivo:</strong> {motivo}
        </div>
      )}

      {/* Transcripción */}
      {lines.length > 0 ? (
        <div style={{
          background: 'var(--bg-secondary, #f8fafc)',
          border: '1px solid var(--glass-border, #e2e8f0)',
          borderRadius: '10px',
          padding: '0.75rem',
          maxHeight: '420px',
          overflowY: 'auto',
          display: 'flex',
          flexDirection: 'column',
          gap: '0.3rem',
        }}>
          {lines.map((line, i) => {
            const type = classifyLine(line)
            const colonIdx = line.indexOf(':')
            const hasPrefix = colonIdx > 0 && colonIdx < 40
            const prefix = hasPrefix ? line.slice(0, colonIdx + 1) : ''
            const body = hasPrefix ? line.slice(colonIdx + 1).trim() : line

            const isAgent = type === 'agent'
            const isSystem = type === 'system'

            return (
              <div key={i} style={{
                display: 'flex',
                justifyContent: isSystem ? 'center' : isAgent ? 'flex-end' : 'flex-start',
              }}>
                <div style={{
                  maxWidth: '80%',
                  background: isSystem
                    ? 'transparent'
                    : isAgent
                      ? 'rgba(204,51,204,0.08)'
                      : '#fff',
                  border: isSystem ? 'none' : '1px solid var(--glass-border, #e2e8f0)',
                  borderRadius: '10px',
                  padding: isSystem ? '0.1rem 0.4rem' : '0.4rem 0.65rem',
                  fontSize: '0.83rem',
                  color: isSystem ? 'var(--gray-400)' : 'var(--text-primary)',
                  fontStyle: isSystem ? 'italic' : 'normal',
                  lineHeight: 1.45,
                  wordBreak: 'break-word',
                }}>
                  {prefix && !isSystem && (
                    <div style={{ fontSize: '0.72rem', fontWeight: 700, color: isAgent ? 'var(--primary)' : '#059669', marginBottom: '1px' }}>
                      {prefix}
                    </div>
                  )}
                  {body}
                </div>
              </div>
            )
          })}
        </div>
      ) : (
        // Sin marcador de historial: mostrar la descripción como texto plano
        <pre style={{
          background: 'var(--bg-secondary, #f8fafc)',
          border: '1px solid var(--glass-border, #e2e8f0)',
          borderRadius: '10px',
          padding: '0.75rem',
          fontSize: '0.83rem',
          color: 'var(--text-primary)',
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-word',
          maxHeight: '360px',
          overflowY: 'auto',
          margin: 0,
        }}>
          {description}
        </pre>
      )}
    </div>
  )
}

export default function InternalTicketDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { user } = useAuth()
  const canEditTickets = canEditModule(user, 'tickets')
  const [ticket, setTicket] = useState<Ticket | null>(null)
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)
  const [showTransfer, setShowTransfer] = useState(false)
  const [transfers, setTransfers] = useState<TicketTransfer[]>([])
  const [agents, setAgents] = useState<SupportAgent[]>([])
  const [inductionBusy, setInductionBusy] = useState(false)

  // Un alta de Obersuite se acompaña, no se responde: por eso tiene su propio
  // atajo para reenviar la capacitación.
  const isObersuite = ticket?.origin === 'obersuite'
  // Las transferencias de WA desde Obersuite traen el historial del chat en
  // description. Se identifican por el título; las altas no tienen transcripción.
  const isWaTransfer = isObersuite && (ticket?.title ?? '').startsWith('WA transferido:')

  // Mismo comportamiento que el botón de la ficha del profesional: el enlace
  // viejo se invalida siempre, porque reenviar el mismo token no resuelve nada
  // si el correo no llegó.
  const sendInduction = async () => {
    if (!ticket?.user_id) return
    setInductionBusy(true)
    setError(null)
    try {
      let registered = true
      try { await inductionService.getUserStatus(ticket.user_id) } catch { registered = false }
      if (registered) await inductionService.resetUser(ticket.user_id)
      else await inductionService.inviteUser(ticket.user_id)
      await ticketService.addInternalNote(ticket.id, 'Se reenvió la capacitación desde el ticket.')
      await fetchTicket(true)
    } catch (e: any) {
      setError(e?.response?.data?.error || 'No se pudo reenviar la capacitación.')
    } finally { setInductionBusy(false) }
  }

  const openTransfer = async () => {
    try { setAgents(await ticketService.getSupportAgents()) } catch { /* ignore */ }
    setShowTransfer(true)
  }

  const fetchTicket = useCallback(async (silent = false) => {
    if (!id) return
    silent ? setRefreshing(true) : setLoading(true)
    try {
      const data = await ticketService.getInternalTicket(Number(id))
      setTicket(data)
      ticketService.getTicketTransfers('internal', Number(id)).then(setTransfers).catch(() => {})
    } catch (err: any) {
      setError(err?.response?.data?.error ?? 'No se pudo cargar el ticket.')
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }, [id])

  useEffect(() => { fetchTicket() }, [fetchTicket])

  const changeStage = async (stage: string) => {
    if (!ticket || stage === ticket.stage) return
    setBusy(true); setError(null)
    try {
      await ticketService.updateInternalTicket(ticket.id, { stage, status: stage === 'closed' ? 'closed' : 'open' })
      await fetchTicket(true)
    } catch (err: any) {
      setError(err?.response?.data?.error ?? 'No se pudo actualizar el estado.')
    } finally { setBusy(false) }
  }

  const addNote = async () => {
    const content = note.trim()
    if (!ticket || !content) return
    setBusy(true); setError(null)
    try {
      await ticketService.addInternalNote(ticket.id, content)
      setNote('')
      await fetchTicket(true)
    } catch (err: any) {
      setError(err?.response?.data?.error ?? 'No se pudo agregar la nota.')
    } finally { setBusy(false) }
  }

  if (loading) {
    return <div className={styles.container} style={{ justifyContent: 'center', alignItems: 'center', paddingTop: '4rem' }}>
      <p style={{ color: 'var(--gray-400)' }}>Cargando ticket interno…</p>
    </div>
  }
  if (!ticket) {
    return <div className={styles.container} style={{ justifyContent: 'center', alignItems: 'center', paddingTop: '4rem' }}>
      <p style={{ color: 'var(--danger)' }}>Ticket no encontrado.</p>
      <button onClick={() => navigate('/tickets')} className={styles.channelBtn} style={{ marginTop: '1rem' }}>Volver a Tickets</button>
    </div>
  }

  const notes = (ticket.messages ?? []).filter(m => m.channel === 'note')

  const InfoRow = ({ icon, label, value }: { icon: React.ReactNode; label: string; value?: string }) => (
    <div style={{ display: 'flex', alignItems: 'flex-start', gap: '0.6rem' }}>
      <span style={{ color: 'var(--text-secondary)', marginTop: '2px' }}>{icon}</span>
      <div>
        <div style={{ fontSize: '0.72rem', color: 'var(--text-secondary)', fontWeight: 600 }}>{label}</div>
        <div style={{ fontSize: '0.9rem', color: 'var(--text-primary)', fontWeight: 600 }}>{value || '—'}</div>
      </div>
    </div>
  )

  return (
    <div className={styles.container}>
      {/* Header */}
      <div className={styles.header} style={{ marginBottom: '0.5rem' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', flexWrap: 'wrap' }}>
          <button onClick={() => navigate('/tickets')} className={styles.channelBtn} style={{ padding: '0.5rem', borderRadius: '50%' }} aria-label="Volver">
            <ArrowLeft size={17} />
          </button>
          <h1 style={{ margin: 0, fontSize: '1.35rem' }}>{ticket.title}</h1>
          <span style={{ fontSize: '0.72rem', fontWeight: 700, padding: '0.2rem 0.6rem', borderRadius: '99px', background: 'rgba(217,119,6,0.12)', color: '#b45309' }}>Interno</span>
          <span style={{ fontSize: '0.75rem', fontWeight: 700, padding: '0.25rem 0.7rem', borderRadius: '99px', background: 'rgba(204,51,204,0.1)', color: 'var(--primary)' }}>{STAGE_LABEL[ticket.stage] || ticket.stage}</span>
        </div>
        <button onClick={() => fetchTicket(true)} className={styles.channelBtn} disabled={refreshing} style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', padding: '0.5rem 0.9rem' }}>
          <RefreshCw size={13} style={{ animation: refreshing ? 'spin 1s linear infinite' : 'none' }} />
          {refreshing ? 'Actualizando…' : 'Actualizar'}
        </button>
      </div>

      {error && (
        <div style={{ padding: '0.75rem 1.1rem', borderRadius: 'var(--radius-sm)', background: 'rgba(239,68,68,0.08)', border: '1px solid rgba(239,68,68,0.2)', color: '#dc2626', fontSize: '0.875rem' }}>⚠️ {error}</div>
      )}

      {/* Body */}
      <div className={styles.detailContainer}>
        {/* Sidebar */}
        <div className={styles.detailSidebar}>
          <div className={styles.sidebarSection}>
            <h3 style={{ fontSize: '0.8rem', fontWeight: 700, color: 'var(--text-secondary)', textTransform: 'uppercase', margin: 0 }}>Datos del profesional</h3>
            <InfoRow icon={<UserIcon size={15} />} label="Nombre" value={professionalName(ticket)} />
            <InfoRow icon={<Mail size={15} />} label="Email" value={ticket.professional_email} />
            <InfoRow icon={<Phone size={15} />} label="Teléfono" value={ticket.professional_phone} />
            <InfoRow icon={<Building2 size={15} />} label="Empresa" value={ticket.company_name} />
          </div>

          {/* Atajos de la incorporación: es todo lo que se hace con ella. Ver la
              ficha vale para cualquier aviso que apunte a alguien; reenviar la
              capacitación solo tiene sentido en un alta de Obersuite. */}
          {!!ticket.user_id && (
            <div className={styles.sidebarSection}>
              <h3 style={{ fontSize: '0.8rem', fontWeight: 700, color: 'var(--text-secondary)', textTransform: 'uppercase', margin: 0 }}>Acciones</h3>
              <button onClick={() => navigate(`/admin/users/${ticket.user_id}`)} className={styles.channelBtn}
                style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', padding: '0.5rem 0.9rem', justifyContent: 'center', color: '#6d28d9', borderColor: 'rgba(124,58,237,0.3)' }}>
                <ExternalLink size={15} /> Ver ficha del profesional
              </button>
              {isObersuite && (
                <button onClick={sendInduction} disabled={inductionBusy} className={styles.channelBtn}
                  style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', padding: '0.5rem 0.9rem', justifyContent: 'center' }}
                  title="Rota el enlace y reinicia sus intentos">
                  <GraduationCap size={15} /> {inductionBusy ? 'Enviando…' : 'Reenviar capacitación'}
                </button>
              )}
            </div>
          )}

          {/* Solo los rechazos traen estos datos; en un alta la sección saldría vacía. */}
          {(ticket.rejected_by_name || ticket.work_dates) && (
            <div className={styles.sidebarSection}>
              <h3 style={{ fontSize: '0.8rem', fontWeight: 700, color: 'var(--text-secondary)', textTransform: 'uppercase', margin: 0 }}>Detalle del rechazo</h3>
              <InfoRow icon={<UserX size={15} />} label="Rechazado por" value={ticket.rejected_by_name} />
              <InfoRow icon={<Calendar size={15} />} label="Fechas" value={ticket.work_dates} />
              <InfoRow icon={<FileText size={15} />} label="Motivo" value={ticket.reason} />
            </div>
          )}

          <div className={styles.sidebarSection}>
            <h3 style={{ fontSize: '0.8rem', fontWeight: 700, color: 'var(--text-secondary)', textTransform: 'uppercase', margin: 0 }}>Estado de seguimiento</h3>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.4rem' }}>
              {STAGE_OPTIONS.map(opt => (
                <button key={opt.id} onClick={() => changeStage(opt.id)} disabled={busy || !canEditTickets}
                  style={{
                    padding: '0.5rem', borderRadius: '8px', cursor: 'pointer', fontSize: '0.85rem', fontWeight: 600, textAlign: 'left',
                    border: ticket.stage === opt.id ? '2px solid var(--primary)' : '1px solid var(--glass-border, #cbd5e1)',
                    background: ticket.stage === opt.id ? 'rgba(204,51,204,0.08)' : 'transparent',
                    color: ticket.stage === opt.id ? 'var(--primary)' : 'var(--text-secondary)',
                  }}>
                  {opt.label}
                </button>
              ))}
            </div>
          </div>

          <div className={styles.sidebarSection}>
            <h3 style={{ fontSize: '0.8rem', fontWeight: 700, color: 'var(--text-secondary)', textTransform: 'uppercase', margin: 0 }}>Responsable</h3>
            <InfoRow icon={<UserIcon size={15} />} label="Asignado a" value={ticket.assignee_name || 'Sin asignar'} />
            {isSupportManager(user) && (
              <button onClick={openTransfer} disabled={busy}
                style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', gap: '0.4rem', padding: '0.5rem', borderRadius: '8px', border: '1px solid var(--primary, #cc33cc)', background: 'transparent', color: 'var(--primary)', fontWeight: 600, cursor: 'pointer', fontSize: '0.85rem' }}>
                <ArrowRightLeft size={15} /> Traspasar ticket
              </button>
            )}
          </div>
        </div>

        {/* Main: chat transcript + notes */}
        <div className={styles.detailMain}>
          {/* Historial del chat transferido: solo en transferencias WA desde Obersuite */}
          {isWaTransfer && ticket.description && (
            <WaChatTranscript description={ticket.description} />
          )}

          <div style={{ padding: '1.25rem', display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
            <h3 style={{ fontSize: '0.9rem', fontWeight: 700, margin: 0 }}>Notas de seguimiento</h3>
            {notes.length === 0 ? (
              <div style={{ textAlign: 'center', color: 'var(--gray-400)', padding: '2.5rem 0' }}>No hay notas en este ticket aún.</div>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '0.6rem' }}>
                {notes.map(n => (
                  <div key={n.id} style={{ background: 'var(--bg-secondary, #f8fafc)', borderRadius: '10px', padding: '0.7rem 0.85rem' }}>
                    <div style={{ fontSize: '0.9rem', color: 'var(--text-primary)', whiteSpace: 'pre-wrap' }}>{n.content}</div>
                    <div style={{ fontSize: '0.72rem', color: 'var(--gray-400)', marginTop: '4px' }}>{new Date(n.created_at).toLocaleString()}</div>
                  </div>
                ))}
              </div>
            )}
          </div>

          {canEditTickets ? (
            <div style={{ borderTop: '1px solid var(--glass-border, #e2e8f0)', padding: '1rem 1.25rem', display: 'flex', gap: '0.5rem', alignItems: 'flex-end' }}>
              <textarea
                value={note}
                onChange={(e) => setNote(e.target.value)}
                onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); addNote() } }}
                placeholder="Escribe una nota de seguimiento… (Enter para guardar)"
                rows={2}
                style={{ flex: 1, resize: 'vertical', padding: '0.6rem 0.75rem', borderRadius: '10px', border: '1px solid var(--glass-border, #cbd5e1)', fontSize: '0.9rem', fontFamily: 'inherit' }}
              />
              <button onClick={addNote} disabled={busy || !note.trim()} className={styles.sendBtn}
                style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', opacity: (busy || !note.trim()) ? 0.6 : 1 }}>
                <Send size={15} /> Agregar
              </button>
            </div>
          ) : (
            <div style={{ borderTop: '1px solid var(--glass-border, #e2e8f0)', padding: '0.85rem 1.25rem', fontSize: '0.85rem', color: 'var(--gray-400)', textAlign: 'center' }}>
              Tu rol tiene acceso de solo lectura en Tickets
            </div>
          )}

          {canEditTickets && ticket.stage !== 'closed' && (
            <div style={{ padding: '0 1.25rem 1.25rem', display: 'flex', justifyContent: 'flex-end' }}>
              <button onClick={() => changeStage('closed')} disabled={busy}
                style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', padding: '0.55rem 1rem', borderRadius: '10px', border: 'none', background: '#10b981', color: '#fff', fontWeight: 600, cursor: 'pointer', fontSize: '0.85rem' }}>
                <CheckCircle2 size={15} /> Marcar como resuelta
              </button>
            </div>
          )}

          {transfers.length > 0 && (
            <div style={{ borderTop: '1px solid var(--glass-border, #e2e8f0)', padding: '1rem 1.25rem' }}>
              <h3 style={{ fontSize: '0.9rem', fontWeight: 700, margin: '0 0 0.5rem', display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
                <History size={15} /> Historial de traspasos
              </h3>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '0.4rem' }}>
                {transfers.map(tr => (
                  <div key={tr.id} style={{ fontSize: '0.82rem', color: 'var(--text-secondary)' }}>
                    <strong style={{ color: 'var(--text-primary)' }}>{tr.from_name || 'Sin asignar'} → {tr.to_name}</strong>
                    {' '}por {tr.by_name} · {new Date(tr.created_at).toLocaleString()}
                    {tr.reason ? ` — ${tr.reason}` : ''}
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>

      {showTransfer && (
        <TransferTicketModal
          options={agents
            .filter(a => a.id !== ticket.assigned_to)
            .map(a => ({ value: a.id, label: a.name }))}
          onClose={() => setShowTransfer(false)}
          onTransfer={async (value, reason) => {
            await ticketService.transferInternalTicket(ticket.id, Number(value), reason)
            await fetchTicket(true)
          }}
        />
      )}

      <style>{`@keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }`}</style>
    </div>
  )
}
