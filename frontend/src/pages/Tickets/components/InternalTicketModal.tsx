import { useState } from 'react'
import { X, CheckCircle2, AlertCircle, Calendar, Mail, Building2, UserX, FileText, Send } from 'lucide-react'
import { Ticket, ticketService } from '../../../services/ticket.service'

interface InternalTicketModalProps {
  ticket: Ticket
  onClose: () => void
  onChanged: () => void
}

const STAGE_OPTIONS: { id: string; label: string }[] = [
  { id: 'new', label: 'Nuevo' },
  { id: 'in_progress', label: 'En seguimiento' },
  { id: 'closed', label: 'Resuelto' },
]

export default function InternalTicketModal({ ticket, onClose, onChanged }: InternalTicketModalProps) {
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [note, setNote] = useState('')
  const [addingNote, setAddingNote] = useState(false)

  const notes = (ticket.messages ?? []).filter(m => m.channel === 'note')

  const handleStage = async (stage: string) => {
    if (stage === ticket.stage) return
    setSubmitting(true)
    setError(null)
    try {
      await ticketService.updateInternalTicket(ticket.id, { stage, status: stage === 'closed' ? 'closed' : 'open' })
      onChanged()
    } catch (err: any) {
      setError(err?.response?.data?.error ?? 'No se pudo actualizar el estado.')
    } finally {
      setSubmitting(false)
    }
  }

  const handleAddNote = async () => {
    const content = note.trim()
    if (!content) return
    setAddingNote(true)
    setError(null)
    try {
      await ticketService.addInternalNote(ticket.id, content)
      setNote('')
      onChanged()
    } catch (err: any) {
      setError(err?.response?.data?.error ?? 'No se pudo agregar la nota.')
    } finally {
      setAddingNote(false)
    }
  }

  const Row = ({ icon, label, value }: { icon: React.ReactNode; label: string; value?: string }) => (
    value ? (
      <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
        <span style={{ color: 'var(--text-secondary)' }}>{icon}</span>
        <span style={{ fontWeight: 600 }}>{label}:</span>
        <span style={{ color: 'var(--text-primary)' }}>{value}</span>
      </div>
    ) : null
  )

  return (
    <div
      onClick={onClose}
      style={{
        position: 'fixed', inset: 0, background: 'rgba(15,23,42,0.45)',
        display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000, padding: '1rem',
      }}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        style={{
          background: 'var(--bg-primary, #fff)', borderRadius: '16px', width: '100%', maxWidth: '520px',
          padding: '1.5rem', boxShadow: '0 20px 40px rgba(0,0,0,0.2)', maxHeight: '90vh', overflowY: 'auto',
        }}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <AlertCircle size={20} style={{ color: '#d97706' }} />
            <h2 style={{ fontSize: '1.1rem', fontWeight: 700, margin: 0 }}>Alerta interna</h2>
            <span style={{
              fontSize: '0.7rem', fontWeight: 700, padding: '2px 8px', borderRadius: '99px',
              background: 'rgba(217,119,6,0.12)', color: '#b45309',
            }}>Interno</span>
          </div>
          <button onClick={onClose} style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--text-secondary)' }}>
            <X size={20} />
          </button>
        </div>

        <h3 style={{ fontSize: '1rem', fontWeight: 600, margin: '0 0 0.75rem' }}>{ticket.title}</h3>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.4rem', marginBottom: '1rem' }}>
          <Row icon={<Mail size={14} />} label="Email" value={ticket.professional_email} />
          <Row icon={<Building2 size={14} />} label="Empresa" value={ticket.company_name} />
          <Row icon={<UserX size={14} />} label="Rechazado por" value={ticket.rejected_by_name} />
          <Row icon={<Calendar size={14} />} label="Fechas" value={ticket.work_dates} />
          <Row icon={<FileText size={14} />} label="Motivo" value={ticket.reason} />
        </div>

        {/* Seguimiento: estado */}
        <div style={{ marginBottom: '1rem' }}>
          <div style={{ fontSize: '0.78rem', fontWeight: 700, color: 'var(--text-secondary)', textTransform: 'uppercase', marginBottom: '0.4rem' }}>
            Estado de seguimiento
          </div>
          <div style={{ display: 'flex', gap: '0.4rem' }}>
            {STAGE_OPTIONS.map(opt => (
              <button
                key={opt.id}
                onClick={() => handleStage(opt.id)}
                disabled={submitting}
                style={{
                  flex: 1, padding: '0.5rem', borderRadius: '8px', cursor: 'pointer', fontSize: '0.82rem', fontWeight: 600,
                  border: ticket.stage === opt.id ? '2px solid var(--primary)' : '1px solid var(--border, #cbd5e1)',
                  background: ticket.stage === opt.id ? 'rgba(204,51,204,0.08)' : 'transparent',
                  color: ticket.stage === opt.id ? 'var(--primary)' : 'var(--text-secondary)',
                }}
              >
                {opt.label}
              </button>
            ))}
          </div>
        </div>

        {/* Notas */}
        <div style={{ marginBottom: '1rem' }}>
          <div style={{ fontSize: '0.78rem', fontWeight: 700, color: 'var(--text-secondary)', textTransform: 'uppercase', marginBottom: '0.4rem' }}>
            Notas de seguimiento
          </div>
          {notes.length === 0 ? (
            <p style={{ fontSize: '0.82rem', color: 'var(--gray-400)', margin: '0 0 0.5rem' }}>Sin notas todavía.</p>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.4rem', marginBottom: '0.5rem' }}>
              {notes.map(n => (
                <div key={n.id} style={{
                  background: 'var(--bg-secondary, #f8fafc)', borderRadius: '8px', padding: '0.5rem 0.65rem',
                  fontSize: '0.85rem', color: 'var(--text-primary)',
                }}>
                  <div style={{ whiteSpace: 'pre-wrap' }}>{n.content}</div>
                  <div style={{ fontSize: '0.7rem', color: 'var(--gray-400)', marginTop: '2px' }}>
                    {new Date(n.created_at).toLocaleString()}
                  </div>
                </div>
              ))}
            </div>
          )}
          <div style={{ display: 'flex', gap: '0.4rem' }}>
            <input
              value={note}
              onChange={(e) => setNote(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') handleAddNote() }}
              placeholder="Agregar una nota..."
              style={{
                flex: 1, padding: '0.5rem 0.65rem', borderRadius: '8px', border: '1px solid var(--border, #cbd5e1)',
                fontSize: '0.85rem',
              }}
            />
            <button
              onClick={handleAddNote}
              disabled={addingNote || !note.trim()}
              style={{
                display: 'inline-flex', alignItems: 'center', gap: '0.3rem', padding: '0.5rem 0.75rem',
                borderRadius: '8px', border: 'none', background: 'var(--primary, #cc33cc)', color: '#fff',
                fontWeight: 600, cursor: 'pointer', fontSize: '0.82rem', opacity: (addingNote || !note.trim()) ? 0.6 : 1,
              }}
            >
              <Send size={14} /> Agregar
            </button>
          </div>
        </div>

        {error && (
          <div style={{ color: '#dc2626', fontSize: '0.85rem', marginBottom: '0.75rem' }}>{error}</div>
        )}

        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.5rem' }}>
          <button
            onClick={onClose}
            style={{
              padding: '0.55rem 1rem', borderRadius: '10px', border: '1px solid var(--border, #cbd5e1)',
              background: 'transparent', fontWeight: 600, cursor: 'pointer', fontSize: '0.85rem',
            }}
          >
            Cerrar
          </button>
          {ticket.stage !== 'closed' && (
            <button
              onClick={() => handleStage('closed')}
              disabled={submitting}
              style={{
                display: 'inline-flex', alignItems: 'center', gap: '0.4rem',
                padding: '0.55rem 1rem', borderRadius: '10px', border: 'none',
                background: '#10b981', color: '#fff', fontWeight: 600, cursor: 'pointer', fontSize: '0.85rem',
                opacity: submitting ? 0.7 : 1,
              }}
            >
              <CheckCircle2 size={15} />
              {submitting ? 'Guardando...' : 'Marcar como resuelta'}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
