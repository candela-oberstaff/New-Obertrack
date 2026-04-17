import { X, Check } from 'lucide-react'
import type { WorkHour } from '../../../types'
import { parseLocalDate } from '../utils'
import styles from '../../../pages/WorkHours.module.css'

interface WorkHourDetailModalProps {
  workHour: WorkHour | null
  onClose: () => void
  canApprove: boolean
  onApprove: (id: number) => Promise<void>
}

export function WorkHourDetailModal({
  workHour,
  onClose,
  canApprove,
  onApprove
}: WorkHourDetailModalProps) {
  if (!workHour) return null

  return (
    <div className={styles['modal-overlay']} onClick={onClose}>
      <div className={`${styles['modal']} ${styles['detail-modal']}`} onClick={(e) => e.stopPropagation()}>
        <div className={styles['modal-header']}>
          <h2>Detalle de Registro</h2>
          <button className={styles['close-btn']} onClick={onClose}><X size={20} /></button>
        </div>
        <div className={styles['detail-content']}>
          <div className={styles['detail-row']}>
            <span className={styles['detail-label']}>Profesional</span>
            <span className={styles['detail-value']}>{workHour.user?.name || 'Usuario'}</span>
          </div>
          <div className={styles['detail-row']}>
            <span className={styles['detail-label']}>Fecha</span>
            <span className={styles['detail-value']}>
              {parseLocalDate(workHour.work_date).toLocaleDateString('es-ES', {
                weekday: 'long', year: 'numeric', month: 'long', day: 'numeric'
              })}
            </span>
          </div>
          <div className={styles['detail-row']}>
            <span className={styles['detail-label']}>Tipo</span>
            <span className={`${styles['detail-type']} ${styles[workHour.work_type]}`}>
              {workHour.work_type === 'complete' || workHour.hours_worked >= 8
                ? 'Jornada Completa' : `Ausencia (${workHour.absence_hours}h)`}
            </span>
          </div>
          <div className={styles['detail-row']}>
            <span className={styles['detail-label']}>Horas</span>
            <span className={styles['detail-value']}>{workHour.hours_worked}h</span>
          </div>
          {workHour.absence_reason && (
            <div className={styles['detail-row']}>
              <span className={styles['detail-label']}>Motivo</span>
              <span className={styles['detail-value']}>{workHour.absence_reason}</span>
            </div>
          )}
          {workHour.activities && (
            <div className={styles['detail-activities']}>
              <span className={styles['detail-label']}>Actividades del día</span>
              <div
                className={styles['detail-text']}
                dangerouslySetInnerHTML={{
                  __html: workHour.activities.replace(/\n/g, '<br>').replace(/• /g, '<br>• ').replace(/<br>1\./g, '<br>1.')
                }}
              />
            </div>
          )}
          <div className={styles['detail-row']}>
            <span className={styles['detail-label']}>Estado</span>
            <span className={`${styles['status-pill']} ${workHour.approved ? styles['approved'] : styles['pending']}`}>
              {workHour.approved ? 'Aprobado' : 'Pendiente'}
            </span>
          </div>
        </div>
        <div className={styles['detail-actions']}>
          <button className={styles['btn-cancel']} onClick={onClose}>Cerrar</button>
          {canApprove && !workHour.approved && (
            <button className={styles['btn-primary']} onClick={async () => {
              await onApprove(workHour.id)
              onClose()
            }}><Check size={16} /> Aprobar</button>
          )}
        </div>
      </div>
    </div>
  )
}
