import { Check, AlertCircle } from 'lucide-react'
import { RichTextEditor } from '../../Tasks/RichTextEditor'
import { Select } from '../../ui/Select'
import { Modal, Button } from '../../ui'
import { htmlToText } from '../../../utils/sanitize'
import { formatHours } from '../../../utils/formatHours'
import { ABSENCE_REASONS, REASONS_REQUIRING_DETAIL } from '../../../utils/absenceReasons'
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
    comments: string
  }
  setFormData: (data: any) => void
  onSubmit: (e: React.FormEvent) => void
  today: string
  isSubmitting?: boolean
}

export function RegisterDayModal({
  isOpen,
  onClose,
  formData,
  setFormData,
  onSubmit,
  today,
  isSubmitting = false
}: RegisterDayModalProps) {
  const hasActivities = htmlToText(formData.activities).length > 0

  const needsJustification =
    formData.work_type === 'absence' && REASONS_REQUIRING_DETAIL.includes(formData.absence_reason)
  const hasJustification = (formData.comments || '').trim().length > 0
  const isSubmitDisabled = !hasActivities || (needsJustification && !hasJustification)

  const absenceTotal = formData.absence_hours || 0
  const absenceHrs = Math.floor(absenceTotal)
  const absenceMins = Math.round((absenceTotal - absenceHrs) * 60)

  const setAbsence = (hrs: number, mins: number) => {
    let total = hrs + mins / 60
    if (total > 8) total = 8
    setFormData({ ...formData, absence_hours: Math.round(total * 100) / 100 })
  }

  const hoursToRegister = formatHours(Math.max(0, 8 - absenceTotal))

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Registrar Día"
      size="md"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={isSubmitting}>Cancelar</Button>
          <Button type="submit" form="register-day-form" loading={isSubmitting} disabled={isSubmitDisabled}>
            Registrar
          </Button>
        </>
      }
    >
      <form id="register-day-form" onSubmit={onSubmit}>
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
              <span className={styles['work-type-hours']}>{hoursToRegister}</span>
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
                options={ABSENCE_REASONS}
              />
            </div>

            {needsJustification && (
              <div className={styles['form-group']}>
                <label>Justificación</label>
                <textarea
                  required
                  rows={3}
                  value={formData.comments}
                  onChange={(e) => setFormData({ ...formData, comments: e.target.value })}
                  placeholder="Explica brevemente el motivo (ej. corte de energía de CFE de 8am a 12pm, sin luz en toda la zona)..."
                />
                <p className={styles['form-hint']}>
                  Detalla la causa para que tu responsable pueda validar la ausencia.
                </p>
              </div>
            )}

            <div className={styles['form-group']}>
              <label>Duración de la ausencia</label>
              <div style={{ display: 'flex', gap: '12px' }}>
                <Select
                  fullWidth
                  required
                  value={absenceHrs}
                  onChange={(v) => setAbsence(Number(v), absenceMins)}
                  options={[
                    { value: 0, label: '0 horas' },
                    { value: 1, label: '1 hora' },
                    { value: 2, label: '2 horas' },
                    { value: 3, label: '3 horas' },
                    { value: 4, label: '4 horas' },
                    { value: 5, label: '5 horas' },
                    { value: 6, label: '6 horas' },
                    { value: 7, label: '7 horas' },
                    { value: 8, label: '8 horas' },
                  ]}
                />
                <Select
                  fullWidth
                  required
                  value={absenceMins}
                  onChange={(v) => setAbsence(absenceHrs, Number(v))}
                  options={[
                    { value: 0, label: '0 min' },
                    { value: 15, label: '15 min' },
                    { value: 30, label: '30 min' },
                    { value: 45, label: '45 min' },
                  ]}
                />
              </div>
              <p className={styles['form-hint']}>
                Ausencia: {formatHours(absenceTotal)} · Horas a registrar: {hoursToRegister}
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
      </form>
    </Modal>
  )
}
