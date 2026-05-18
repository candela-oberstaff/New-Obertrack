import { Calendar, CheckCircle2, Hourglass, Users } from 'lucide-react'
import type { WorkHour } from '../../types'
import styles from '../../pages/WorkHours.module.css'

interface WorkHourStatsProps {
  todayWork?: WorkHour
  weekHours: number
  summary: {
    approved_hours: number
    pending_hours: number
  }
  isEmployer?: boolean
  employerTodayActiveCount?: number
}

export function WorkHourStats({ 
  todayWork, 
  weekHours, 
  summary,
  isEmployer = false,
  employerTodayActiveCount = 0
}: WorkHourStatsProps) {
  return (
    <div className={styles['wh-stats-row']}>
      <div className={styles['stat-card-mini']}>
        <span className={styles['stat-icon']}>
          {isEmployer ? <Users size={20} /> : <Calendar size={20} />}
        </span>
        <div className={styles['stat-info']}>
          <span className={styles['stat-label']}>Hoy</span>
          <span className={styles['stat-value']}>
            {isEmployer ? (
              `${employerTodayActiveCount} ${employerTodayActiveCount === 1 ? 'activo' : 'activos'}`
            ) : todayWork ? (
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
          <span className={styles['stat-label']}>
            {isEmployer ? 'Horas del equipo (semana)' : 'Esta semana'}
          </span>
          <span className={styles['stat-value']}>{weekHours.toFixed(1)}h</span>
        </div>
      </div>
      <div className={`${styles['stat-card-mini']} ${styles['approved']}`}>
        <span className={styles['stat-icon']}><CheckCircle2 size={20} /></span>
        <div className={styles['stat-info']}>
          <span className={styles['stat-label']}>
            {isEmployer ? 'Horas aprobadas (mes)' : 'Aprobadas'}
          </span>
          <span className={styles['stat-value']}>{summary.approved_hours.toFixed(1)}h</span>
        </div>
      </div>
      <div className={`${styles['stat-card-mini']} ${styles['pending']}`}>
        <span className={styles['stat-icon']}><Hourglass size={20} /></span>
        <div className={styles['stat-info']}>
          <span className={styles['stat-label']}>
            {isEmployer ? 'Horas pendientes (mes)' : 'Pendientes'}
          </span>
          <span className={styles['stat-value']}>{summary.pending_hours.toFixed(1)}h</span>
        </div>
      </div>
    </div>
  )
}
