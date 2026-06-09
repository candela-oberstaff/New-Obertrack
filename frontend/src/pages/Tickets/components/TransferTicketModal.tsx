import { useState } from 'react'
import { X, ArrowRightLeft } from 'lucide-react'
import { Select } from '../../../components/ui/Select'

export interface TransferOption {
  value: string | number
  label: string
}

interface TransferTicketModalProps {
  options: TransferOption[]
  onClose: () => void
  /** Performs the transfer with the chosen target value + reason. */
  onTransfer: (value: string | number, reason: string) => Promise<void>
}

export default function TransferTicketModal({ options, onClose, onTransfer }: TransferTicketModalProps) {
  const [target, setTarget] = useState<string | number>('')
  const [reason, setReason] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = async () => {
    if (!target) return
    setSubmitting(true); setError(null)
    try {
      await onTransfer(target, reason.trim())
      onClose()
    } catch (err: any) {
      setError(err?.response?.data?.error ?? 'No se pudo traspasar el ticket.')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div onClick={onClose} style={{ position: 'fixed', inset: 0, background: 'rgba(15,23,42,0.45)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1100, padding: '1rem' }}>
      <div onClick={(e) => e.stopPropagation()} style={{ background: 'var(--bg-primary, #fff)', borderRadius: '16px', width: '100%', maxWidth: '440px', padding: '1.5rem', boxShadow: '0 20px 40px rgba(0,0,0,0.2)' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <ArrowRightLeft size={19} style={{ color: 'var(--primary)' }} />
            <h2 style={{ fontSize: '1.05rem', fontWeight: 700, margin: 0 }}>Traspasar ticket</h2>
          </div>
          <button onClick={onClose} style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--text-secondary)' }}><X size={20} /></button>
        </div>

        <label style={{ display: 'block', fontSize: '0.8rem', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '0.3rem' }}>Nuevo responsable</label>
        <div style={{ marginBottom: '1rem' }}>
          <Select
            fullWidth
            value={target}
            onChange={(v) => setTarget(v)}
            placeholder={options.length ? 'Selecciona un agente…' : 'No hay otros agentes disponibles'}
            options={options}
          />
        </div>

        <label style={{ display: 'block', fontSize: '0.8rem', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: '0.3rem' }}>Motivo (opcional)</label>
        <textarea
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          rows={3}
          placeholder="Motivo del traspaso…"
          style={{ width: '100%', padding: '0.6rem', borderRadius: '8px', border: '1px solid var(--border, #cbd5e1)', fontSize: '0.9rem', fontFamily: 'inherit', resize: 'vertical' }}
        />

        {error && <div style={{ color: '#dc2626', fontSize: '0.85rem', marginTop: '0.5rem' }}>{error}</div>}

        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.5rem', marginTop: '1.25rem' }}>
          <button onClick={onClose} style={{ padding: '0.55rem 1rem', borderRadius: '10px', border: '1px solid var(--border, #cbd5e1)', background: 'transparent', fontWeight: 600, cursor: 'pointer', fontSize: '0.85rem' }}>Cancelar</button>
          <button onClick={handleSubmit} disabled={submitting || !target}
            style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', padding: '0.55rem 1rem', borderRadius: '10px', border: 'none', background: 'var(--primary, #cc33cc)', color: '#fff', fontWeight: 600, cursor: 'pointer', fontSize: '0.85rem', opacity: (submitting || !target) ? 0.6 : 1 }}>
            <ArrowRightLeft size={15} /> {submitting ? 'Traspasando…' : 'Traspasar'}
          </button>
        </div>
      </div>
    </div>
  )
}
