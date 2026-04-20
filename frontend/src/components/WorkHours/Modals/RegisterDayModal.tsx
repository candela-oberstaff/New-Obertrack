import { X, Check, AlertCircle } from 'lucide-react'
import { RichTextEditor } from '../../Tasks/RichTextEditor'
import styles from '../../../pages/WorkHours.module.css'

interface RegisterDayModalProps {
  isOpen: boolean
  onClose: () => void
  formData: {
    work_date: string
    work_type: 'complete' | 'absence'
    activities: string
    absence_reason: string
    absence_hours: number
  }
  setFormData: (data: any) => void
  onSubmit: (e: React.FormEvent) => void
  today: string
}

export function RegisterDayModal({
  isOpen,
  onClose,
  formData,
  setFormData,
  onSubmit,
  today
}: RegisterDayModalProps) {
  if (!isOpen) return null

  return (
    <div className={styles['modal-overlay']} onClick={onClose}>
      <div className={styles['modal']} onClick={(e) => e.stopPropagation()}>
        <div className={styles['modal-header']}>
          <h2>Registrar Día</h2>
          <button className={styles['close-btn']} onClick={onClose}><X size={20} /></button>
        </div>
        <form onSubmit={onSubmit}>
          <div className={styles['form-group']}>
            <label>Fecha</label>
            <input
              type="date"
              value={formData.work_date}
              max={today}
              onChange={(e) => setFormData({ ...formData, work_date: e.target.value })}
              required
            />
          </div>

          <div className={styles['form-group']}>
            <label>¿Cómo fue tu día?</label>
            <div className={styles['work-type-selector']}>
              <button
                type="button"
                className={`${styles['work-type-btn']} ${formData.work_type === 'complete' ? styles['active'] : ''}`}
                onClick={() => setFormData({ ...formData, work_type: 'complete' })}
              >
                <span className={styles['work-type-icon']}><Check size={20} /></span>
                <span className={styles['work-type-label']}>Jornada Completa</span>
                <span className={styles['work-type-hours']}>8h</span>
              </button>
              <button
                type="button"
                className={`${styles['work-type-btn']} ${formData.work_type === 'absence' ? styles['active'] : ''}`}
                onClick={() => setFormData({ ...formData, work_type: 'absence' })}
              >
                <span className={styles['work-type-icon']}><AlertCircle size={20} /></span>
                <span className={styles['work-type-label']}>Ausencia</span>
                <span className={styles['work-type-hours']}>{Math.max(0, 8 - (formData.absence_hours || 0))}h</span>
              </button>
            </div>
          </div>

          {formData.work_type === 'absence' && (
            <>
              <div className={styles['form-group']}>
                <label>Motivo de ausencia</label>
                <select
                  value={formData.absence_reason}
                  onChange={(e) => setFormData({ ...formData, absence_reason: e.target.value })}
                  required
                >
                  <option value="">Selecciona un motivo</option>
                  <option value="enfermedad">Enfermedad</option>
                  <option value="cita_medica">Cita Médica</option>
                  <option value="emergencia_familiar">Emergencia Familiar</option>
                  <option value="vacaciones">Vacaciones</option>
                  <option value="permiso_personal">Permiso Personal</option>
                  <option value="otro">Otro</option>
                </select>
              </div>

              <div className={styles['form-group']}>
                <label>Horas de ausencia</label>
                <select
                  value={formData.absence_hours}
                  onChange={(e) => setFormData({ ...formData, absence_hours: Number(e.target.value) })}
                  required
                >
                  <option value={0}>0 horas</option>
                  <option value={1}>1 hora</option>
                  <option value={2}>2 horas</option>
                  <option value={3}>3 horas</option>
                  <option value={4}>4 horas</option>
                  <option value={5}>5 horas</option>
                  <option value={6}>6 horas</option>
                  <option value={7}>7 horas</option>
                  <option value={8}>8 horas (día completo)</option>
                </select>
                <p className={styles['form-hint']}>
                  Horas a registrar: {Math.max(0, 8 - (formData.absence_hours || 0))}h
                </p>
              </div>
            </>
          )}

          <div className={styles['form-group']}>
            <label>¿Qué actividades realizaste hoy?</label>
            <RichTextEditor
              value={formData.activities}
              onChange={(value) => setFormData({ ...formData, activities: value })}
              placeholder="Describe las actividades realizadas durante tu jornada...&#10;• Tarea 1&#10;• Tarea 2"
            />
          </div>

          <div className={styles['modal-actions']}>
            <button type="button" className={styles['btn-cancel']} onClick={onClose}>
              Cancelar
            </button>
            <button type="submit" className={styles['btn-primary']}>
              Registrar
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
