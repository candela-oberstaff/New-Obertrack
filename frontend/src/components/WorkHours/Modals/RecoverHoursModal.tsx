import { useState } from 'react'
import { X, Clock } from 'lucide-react'
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

  if (!isOpen) return null

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    await onSubmit(date, hours, comments)
    setComments('')
    setHours(1)
    onClose()
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

        <form onSubmit={handleSubmit}>
          <div className={styles['form-group']}>
            <label>Fecha de recuperación</label>
            <input
              type="date"
              value={date}
              max={today}
              onChange={(e) => setDate(e.target.value)}
              required
            />
          </div>

          <div className={styles['form-group']}>
            <label>Horas recuperadas</label>
            <select
              value={hours}
              onChange={(e) => setHours(Number(e.target.value))}
              required
            >
              {[1, 2, 3, 4, 5, 6, 7, 8].map(h => (
                <option key={h} value={h}>{h} {h === 1 ? 'hora' : 'horas'}</option>
              ))}
            </select>
          </div>

          <div className={styles['form-group']}>
            <label>Actividades / Comentarios</label>
            <textarea
              value={comments}
              onChange={(e) => setComments(e.target.value)}
              placeholder="Describe brevemente qué tareas realizaste..."
              rows={3}
              required
              style={{ width: '100%', padding: '10px', borderRadius: '8px', border: '1px solid #e2e8f0', resize: 'vertical' }}
            />
          </div>

          <div className={styles['modal-actions']}>
            <button type="button" className={styles['btn-cancel']} onClick={onClose}>
              Cancelar
            </button>
            <button type="submit" className={styles['btn-primary']} disabled={hours <= 0}>
              Guardar Recuperación
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
