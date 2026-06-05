import { Check, ClipboardList } from 'lucide-react'
import Tooltip from '../Common/Tooltip'
import type { WorkHour } from '../../types'
import { MONTHS_ES, parseLocalDate, JORNADA_COMPLETA } from './utils'
import styles from '../../pages/WorkHours.module.css'

interface WorkHourListProps {
  filteredHours: WorkHour[]
  selectedDate: string | null
  canApprove: boolean
  pendingForSelectedDate: WorkHour[]
  onBulkApprove: () => void
  onItemClick: (wh: WorkHour) => void
  isEmployer?: boolean
}

export function WorkHourList({
  filteredHours,
  selectedDate,
  canApprove,
  pendingForSelectedDate,
  onBulkApprove,
  onItemClick,
  isEmployer
}: WorkHourListProps) {
  const getStatus = (wh: WorkHour) => {
    if (wh.approved) return { className: 'approved', label: 'Aprobado' }
    if (wh.rejected) return { className: 'rejected', label: 'Rechazado' }
    return { className: 'pending', label: 'Pendiente' }
  }

  return (
    <div className={styles['hours-list-section']} data-tour="work-hours-list">
      <div className={styles['list-header']}>
        <h3>
          {selectedDate
            ? `Registros del ${parseLocalDate(selectedDate).toLocaleDateString('es-ES', { day: 'numeric', month: 'long' })}`
            : isEmployer
            ? 'Registros'
            : 'Mis registros'}{' '}
          {!selectedDate && !isEmployer && (
            <Tooltip content="Últimos registros que haz realizado" size={14} />
          )}
        </h3>
        {canApprove && pendingForSelectedDate.length > 0 && (
          <button className={styles['btn-bulk-approve']} onClick={onBulkApprove}>
            <Check size={16} /> Aprobar todos ({pendingForSelectedDate.length})
          </button>
        )}
      </div>

      {filteredHours.length === 0 ? (
        <div className={styles['empty-state']}>
          <span className={styles['empty-icon']}><ClipboardList size={40} /></span>
          <p>No hay registros</p>
        </div>
      ) : (
        <div className={styles['hours-list']}>
          {filteredHours.map((wh) => (
            <div 
              key={wh.id} 
              className={`${styles['hour-card']} ${wh.work_type === 'complete' || wh.hours_worked >= JORNADA_COMPLETA ? styles['complete'] : wh.work_type === 'recover' ? styles['recover'] : styles['absence']} ${styles['clickable']}`} 
              onClick={() => onItemClick(wh)}
            >
              <div className={styles['hour-date']}>
                <span className={styles['day']}>{parseLocalDate(wh.work_date).getDate()}</span>
                <span className={styles['month']}>{MONTHS_ES[parseLocalDate(wh.work_date).getMonth()].slice(0, 3)}</span>
              </div>
              <div className={styles['hour-info']}>
                {canApprove && wh.user && <span className={styles['hours-user']}>{wh.user.name}</span>}
                <span className={styles['hours-value']}>
                  {wh.work_type === 'complete' || wh.hours_worked >= JORNADA_COMPLETA
                    ? 'Jornada Completa'
                    : wh.work_type === 'recover'
                    ? `Recuperación (${wh.hours_worked}h)`
                    : `Ausencia (${wh.absence_hours != null ? wh.absence_hours : 8 - wh.hours_worked}h)`}
                </span>
                {wh.activities && <p className={styles['hours-comments']}>{wh.activities.replace(/<[^>]*>/g, '')}</p>}
              </div>
              <div className={styles['hour-status']}>
                <span className={`${styles['status-pill']} ${styles[getStatus(wh).className]}`}>
                  {getStatus(wh).label}
                </span>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
