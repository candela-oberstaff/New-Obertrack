import { useCallback, useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, RefreshCw, Mail, Phone, Building2, User as UserIcon, UserX, Calendar, FileText, Send, CheckCircle2, ArrowRightLeft, History } from 'lucide-react'
import { Ticket, TicketTransfer, SupportAgent, ticketService } from '../../services/ticket.service'
import TransferTicketModal from './components/TransferTicketModal'
import styles from './Tickets.module.css'

const STAGE_OPTIONS: { id: string; label: string }[] = [
  { id: 'new', label: 'Nuevo' },
  { id: 'in_progress', label: 'En seguimiento' },
  { id: 'closed', label: 'Resuelto' },
]
const STAGE_LABEL: Record<string, string> = { new: 'Nuevo', in_progress: 'En seguimiento', waiting: 'En seguimiento', closed: 'Resuelto' }

function professionalName(t: Ticket): string {
  return t.title?.replace(/^Rechazo de horas:\s*/i, '') || 'Profesional'
}

export default function InternalTicketDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [ticket, setTicket] = useState<Ticket | null>(null)
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)
  const [showTransfer, setShowTransfer] = useState(false)
  const [transfers, setTransfers] = useState<TicketTransfer[]>([])
  const [agents, setAgents] = useState<SupportAgent[]>([])

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

          <div className={styles.sidebarSection}>
            <h3 style={{ fontSize: '0.8rem', fontWeight: 700, color: 'var(--text-secondary)', textTransform: 'uppercase', margin: 0 }}>Detalle del rechazo</h3>
            <InfoRow icon={<UserX size={15} />} label="Rechazado por" value={ticket.rejected_by_name} />
            <InfoRow icon={<Calendar size={15} />} label="Fechas" value={ticket.work_dates} />
            <InfoRow icon={<FileText size={15} />} label="Motivo" value={ticket.reason} />
          </div>

          <div className={styles.sidebarSection}>
            <h3 style={{ fontSize: '0.8rem', fontWeight: 700, color: 'var(--text-secondary)', textTransform: 'uppercase', margin: 0 }}>Estado de seguimiento</h3>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.4rem' }}>
              {STAGE_OPTIONS.map(opt => (
                <button key={opt.id} onClick={() => changeStage(opt.id)} disabled={busy}
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
            <button onClick={openTransfer} disabled={busy}
              style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', gap: '0.4rem', padding: '0.5rem', borderRadius: '8px', border: '1px solid var(--primary, #cc33cc)', background: 'transparent', color: 'var(--primary)', fontWeight: 600, cursor: 'pointer', fontSize: '0.85rem' }}>
              <ArrowRightLeft size={15} /> Traspasar ticket
            </button>
          </div>
        </div>

        {/* Main: notes */}
        <div className={styles.detailMain}>
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

          {ticket.stage !== 'closed' && (
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
            .map(a => ({ value: a.id, label: `${a.name} (${a.email})` }))}
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
