import { useState } from 'react'
import { ArrowRightLeft } from 'lucide-react'
import { Select } from '../../../components/ui/Select'
import { Modal, Button } from '../../../components/ui'

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
    <Modal
      isOpen
      onClose={onClose}
      size="sm"
      title={
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: '0.5rem' }}>
          <ArrowRightLeft size={19} style={{ color: 'var(--primary)' }} />
          Traspasar ticket
        </span>
      }
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={submitting}>Cancelar</Button>
          <Button onClick={handleSubmit} loading={submitting} disabled={!target} leftIcon={<ArrowRightLeft size={15} />}>
            Traspasar
          </Button>
        </>
      }
    >
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
    </Modal>
  )
}
