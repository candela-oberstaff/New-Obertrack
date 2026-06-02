import { X, Check, AlertCircle } from 'lucide-react'
import { RichTextEditor } from '../../Tasks/RichTextEditor'
import { Select } from '../../ui/Select'
import styles from '../../../pages/WorkHours.module.css'

interface RegisterDayModalProps {
  isOpen: boolean
  onClose: () => void
  formData: {
    work_date: string
    work_type: 'complete' | 'absence' | 'recover'
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
              min={new Date(new Date().getFullYear(), new Date().getMonth(), 1).toISOString().split('T')[0]}
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
                onClick={() => setFormData({ ...formData, work_type: 'complete', absence_hours: 0 })}
              >
                <span className={styles['work-type-icon']}><Check size={20} /></span>
                <span className={styles['work-type-label']}>Jornada Completa</span>
                <span className={styles['work-type-hours']}>8h</span>
              </button>
              <button
                type="button"
                className={`${styles['work-type-btn']} ${formData.work_type === 'absence' ? styles['active'] : ''}`}
                onClick={() => setFormData({ ...formData, work_type: 'absence', absence_hours: 8 })}
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
                <Select
                  fullWidth
                  required
                  value={formData.absence_reason}
                  onChange={(v) => setFormData({ ...formData, absence_reason: String(v) })}
                  placeholder="Selecciona un motivo"
                  options={[
                    { value: 'enfermedad', label: 'Enfermedad' },
                    { value: 'cita_medica', label: 'Cita Médica' },
                    { value: 'emergencia_familiar', label: 'Emergencia Familiar' },
                    { value: 'vacaciones', label: 'Vacaciones' },
                    { value: 'permiso_personal', label: 'Permiso Personal' },
                    { value: 'otro', label: 'Otro' },
                  ]}
                />
              </div>

              <div className={styles['form-group']}>
                <label>Horas de ausencia</label>
                <Select
                  fullWidth
                  required
                  value={formData.absence_hours}
                  onChange={(v) => setFormData({ ...formData, absence_hours: Number(v) })}
                  options={[
                    { value: 1, label: '1 hora' },
                    { value: 2, label: '2 horas' },
                    { value: 3, label: '3 horas' },
                    { value: 4, label: '4 horas' },
                    { value: 5, label: '5 horas' },
                    { value: 6, label: '6 horas' },
                    { value: 7, label: '7 horas' },
                    { value: 8, label: '8 horas (día completo)' },
                  ]}
                />
                <p className={styles['form-hint']}>
                  Horas a registrar: {Math.max(0, 8 - (formData.absence_hours || 0))}h
                </p>
              </div>

              <div className={styles['absence-alert'] || 'absence-alert'}>
                <AlertCircle size={18} />
                <div>
                  <strong>Nota sobre ausencias</strong>
                  <p>Al registrar una ausencia, podrás recuperar estas horas en otro momento si así lo deseas coordinar.</p>
                </div>
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
