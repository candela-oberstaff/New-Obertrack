import { useState } from 'react'
import { X, Clock, AlertCircle, Loader2 } from 'lucide-react'
import { Select } from '../../ui/Select'
import styles from '../../../pages/WorkHours.module.css'

interface RecoverHoursModalProps {
  isOpen: boolean
  onClose: () => void
  onSubmit: (date: string, hours: number, comments: string) => Promise<void>
  today: string
  absenceHoursToRecover: number
}

export function RecoverHoursModal({
  isOpen,
  onClose,
  onSubmit,
  today,
  absenceHoursToRecover
}: RecoverHoursModalProps) {
  const [date, setDate] = useState(today)
  const [hours, setHours] = useState<number>(1)
  const [comments, setComments] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)

  if (!isOpen) return null

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setIsSubmitting(true)
    try {
      await onSubmit(date, hours, comments)
      setComments('')
      setHours(1)
      onClose()
    } catch (err: any) {
      const msg = err?.response?.data?.error || err?.message || 'Error al guardar la recuperación. Intenta de nuevo.'
      setError(msg)
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className={styles['modal-overlay']}>
      <div className={styles['modal']}>
        <div className={styles['modal-header']}>
          <h2>Recuperar Horas</h2>
          <button className={styles['close-btn']} onClick={onClose}><X size={20} /></button>
        </div>

        <div style={{ backgroundColor: '#eff6ff', border: '1px solid #bfdbfe', color: '#1e40af', padding: '12px', borderRadius: '8px', marginBottom: '16px', fontSize: '14px', display: 'flex', gap: '8px' }}>
          <Clock size={20} style={{ flexShrink: 0 }} />
          <div>
            <strong>Tienes {absenceHoursToRecover.toFixed(1)}h pendientes de recuperar.</strong><br/>
            Registra aquí las horas extras que has trabajado para compensar ausencias anteriores.
          </div>
        </div>

        {error && (
          <div style={{ backgroundColor: '#fef2f2', border: '1px solid #fecaca', color: '#991b1b', padding: '12px', borderRadius: '8px', marginBottom: '16px', fontSize: '14px', display: 'flex', alignItems: 'flex-start', gap: '8px' }}>
            <AlertCircle size={18} style={{ flexShrink: 0, marginTop: '1px' }} />
            <div>{error}</div>
          </div>
        )}

        <form onSubmit={handleSubmit}>
          <div className={styles['form-group']}>
            <label>Fecha de recuperación</label>
            <input
              type="date"
              value={date}
              max={today}
              onChange={(e) => { setDate(e.target.value); setError(null) }}
              required
            />
          </div>

          <div className={styles['form-group']}>
            <label>Horas recuperadas</label>
            <Select
              fullWidth
              required
              value={hours}
              onChange={(v) => { setHours(Number(v)); setError(null) }}
              options={[1, 2, 3, 4, 5, 6, 7, 8].map(h => ({ value: h, label: `${h} ${h === 1 ? 'hora' : 'horas'}` }))}
            />
          </div>

          <div className={styles['form-group']}>
            <label>Actividades / Comentarios</label>
            <textarea
              value={comments}
              onChange={(e) => { setComments(e.target.value); setError(null) }}
              placeholder="Describe brevemente qué tareas realizaste..."
              rows={3}
              required
              style={{ width: '100%', padding: '10px', borderRadius: '8px', border: '1px solid #e2e8f0', resize: 'vertical' }}
            />
          </div>

          <div className={styles['modal-actions']}>
            <button type="button" className={styles['btn-cancel']} onClick={onClose} disabled={isSubmitting}>
              Cancelar
            </button>
            <button type="submit" className={styles['btn-primary']} disabled={hours <= 0 || isSubmitting} style={{ display: 'inline-flex', alignItems: 'center', gap: '8px' }}>
              {isSubmitting && <Loader2 size={16} className="animate-spin" />}
              {isSubmitting ? 'Guardando...' : 'Guardar Recuperación'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
