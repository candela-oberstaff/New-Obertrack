import { Calendar, CheckCircle2, Hourglass } from 'lucide-react'
import type { WorkHour } from '../../types'
import styles from '../../pages/WorkHours.module.css'

interface WorkHourStatsProps {
  todayWork?: WorkHour
  weekHours: number
  summary: {
    approved_hours: number
    pending_hours: number
  }
}

export function WorkHourStats({ todayWork, weekHours, summary }: WorkHourStatsProps) {
  return (
    <div className={styles['wh-stats-row']}>
      <div className={styles['stat-card-mini']}>
        <span className={styles['stat-icon']}><Calendar size={20} /></span>
        <div className={styles['stat-info']}>
          <span className={styles['stat-label']}>Hoy</span>
          <span className={styles['stat-value']}>
            {todayWork ? (
              todayWork.work_type === 'complete' || todayWork.hours_worked >= 8 ? (
                <><CheckCircle2 size={14} /> Completa</>
              ) : (
                <><CheckCircle2 size={14} /> {todayWork.hours_worked}h</>
              )
            ) : '-'}
          </span>
        </div>
      </div>
      <div className={styles['stat-card-mini']}>
        <span className={styles['stat-icon']}><Calendar size={20} /></span>
        <div className={styles['stat-info']}>
          <span className={styles['stat-label']}>Esta semana</span>
          <span className={styles['stat-value']}>{weekHours.toFixed(1)}h</span>
        </div>
      </div>
      <div className={`${styles['stat-card-mini']} ${styles['approved']}`}>
        <span className={styles['stat-icon']}><CheckCircle2 size={20} /></span>
        <div className={styles['stat-info']}>
          <span className={styles['stat-label']}>Aprobadas</span>
          <span className={styles['stat-value']}>{summary.approved_hours.toFixed(1)}h</span>
        </div>
      </div>
      <div className={`${styles['stat-card-mini']} ${styles['pending']}`}>
        <span className={styles['stat-icon']}><Hourglass size={20} /></span>
        <div className={styles['stat-info']}>
          <span className={styles['stat-label']}>Pendientes</span>
          <span className={styles['stat-value']}>{summary.pending_hours.toFixed(1)}h</span>
        </div>
      </div>
    </div>
  )
}
