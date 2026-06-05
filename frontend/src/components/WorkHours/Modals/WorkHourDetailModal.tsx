import { useState } from 'react'
import { X, Check, Pencil, XCircle } from 'lucide-react'
import type { WorkHour } from '../../../types'
import { parseLocalDate } from '../utils'
import { sanitizeHtml } from '../../../utils/sanitize'
import styles from '../../../pages/WorkHours.module.css'

interface WorkHourDetailModalProps {
  workHour: WorkHour | null
  onClose: () => void
  canApprove: boolean
  onApprove: (id: number) => Promise<void>
  onReject: (id: number, reason: string) => Promise<void>
  onEdit: (wh: WorkHour) => void
  isEmployer?: boolean
}

export function WorkHourDetailModal({
  workHour,
  onClose,
  canApprove,
  onApprove,
  onReject,
  onEdit,
  isEmployer = false
}: WorkHourDetailModalProps) {
  const [showRejectForm, setShowRejectForm] = useState(false)
  const [rejectionReason, setRejectionReason] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)

  if (!workHour) return null

  const statusClass = workHour.approved ? 'approved' : workHour.rejected ? 'rejected' : 'pending'
  const statusLabel = workHour.approved ? 'Aprobado' : workHour.rejected ? 'Rechazado' : 'Pendiente'

  const handleApprove = async () => {
    setIsSubmitting(true)
    try {
      await onApprove(workHour.id)
      onClose()
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleReject = async () => {
    const reason = rejectionReason.trim()
    if (!reason) return
    setIsSubmitting(true)
    try {
      await onReject(workHour.id, reason)
      onClose()
    } finally {
      setIsSubmitting(false)
    }
  }

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
              {workHour.work_type === 'recover' 
                ? `Horas Recuperadas (${workHour.hours_worked}h)` 
                : workHour.work_type === 'complete' || workHour.hours_worked >= 8
                ? 'Jornada Completa' : `Ausencia (${workHour.absence_hours != null ? workHour.absence_hours : 8 - workHour.hours_worked}h)`}
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
                __html: sanitizeHtml(workHour.activities)
              }}
              />
            </div>
          )}
          <div className={styles['detail-row']}>
            <span className={styles['detail-label']}>Estado</span>
            <span className={`${styles['status-pill']} ${styles[statusClass]}`}>
              {statusLabel}
            </span>
          </div>
          {!workHour.approved && workHour.rejected && workHour.rejection_reason && (
            <div className={styles['detail-rejection']}>
              <span className={styles['detail-label']}>Motivo del rechazo</span>
              <p>{workHour.rejection_reason}</p>
            </div>
          )}
          {showRejectForm && (
            <div className={styles['reject-form']}>
              <label htmlFor="rejection-reason">Motivo del rechazo</label>
              <textarea
                id="rejection-reason"
                value={rejectionReason}
                onChange={(e) => setRejectionReason(e.target.value)}
                placeholder="Indica qué debe corregirse en este registro..."
                rows={3}
                autoFocus
              />
            </div>
          )}
        </div>
        <div className={styles['detail-actions']}>
          {!isEmployer && !workHour.approved && (
            <button className={styles['btn-secondary']} onClick={() => onEdit(workHour)}>
              <Pencil size={16} /> Editar
            </button>
          )}
          {canApprove && !workHour.approved && !workHour.rejected && !showRejectForm && (
            <button className={styles['btn-danger-outline']} onClick={() => setShowRejectForm(true)}>
              <XCircle size={16} /> Rechazar
            </button>
          )}
          {canApprove && !workHour.approved && !workHour.rejected && showRejectForm && (
            <button className={styles['btn-danger']} onClick={handleReject} disabled={!rejectionReason.trim() || isSubmitting}>
              <XCircle size={16} /> Confirmar rechazo
            </button>
          )}
          {canApprove && !workHour.approved && (
            <button className={styles['btn-primary']} onClick={handleApprove} disabled={isSubmitting}>
              <Check size={16} /> Aprobar
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
