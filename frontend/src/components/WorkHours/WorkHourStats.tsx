import { useState, useEffect, useRef } from 'react'
import { Calendar, CheckCircle2, Hourglass, Users, ChevronLeft, ChevronRight, AlertCircle } from 'lucide-react'
import type { WorkHour } from '../../types'
import Tooltip from '../Common/Tooltip'
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
  absenceHoursToRecover: number
}

export function WorkHourStats({ 
  todayWork, 
  weekHours, 
  summary,
  isEmployer = false,
  employerTodayActiveCount = 0,
  absenceHoursToRecover
}: WorkHourStatsProps) {
  const rowRef = useRef<HTMLDivElement>(null)
  const [showLeftBtn, setShowLeftBtn] = useState(false)
  const [showRightBtn, setShowRightBtn] = useState(true)

  const checkScroll = () => {
    if (rowRef.current) {
      const { scrollLeft, scrollWidth, clientWidth } = rowRef.current
      setShowLeftBtn(scrollLeft > 5)
      setShowRightBtn(scrollLeft < scrollWidth - clientWidth - 5)
    }
  }

  useEffect(() => {
    const el = rowRef.current
    if (el) {
      checkScroll()
      el.addEventListener('scroll', checkScroll)
      window.addEventListener('resize', checkScroll)
      // Check after a tiny delay to ensure clientWidth and scrollWidth are set
      const timer = setTimeout(checkScroll, 100)
      return () => {
        el.removeEventListener('scroll', checkScroll)
        window.removeEventListener('resize', checkScroll)
        clearTimeout(timer)
      }
    }
  }, [employerTodayActiveCount, weekHours, summary])

  const scrollLeft = () => {
    if (rowRef.current) {
      rowRef.current.scrollBy({ left: -180, behavior: 'smooth' })
    }
  }

  const scrollRight = () => {
    if (rowRef.current) {
      rowRef.current.scrollBy({ left: 180, behavior: 'smooth' })
    }
  }

  return (
    <div className={styles['stats-row-container']} data-tour="work-hours-stats">
      {showLeftBtn && (
        <button className={styles['scroll-btn-left']} onClick={scrollLeft} aria-label="Ver tarjetas anteriores">
          <ChevronLeft size={20} />
        </button>
      )}
      <div className={styles['wh-stats-row']} ref={rowRef}>
        <div className={styles['stat-card-mini']}>
          <span className={styles['stat-icon']}>
            {isEmployer ? <Users size={20} /> : <Calendar size={20} />}
          </span>
          <div className={styles['stat-info']}>
            <span className={styles['stat-label']}>
              Hoy
              {isEmployer && (
                <Tooltip content="Miembros de tu equipo activos el día de hoy" size={12} style={{ marginLeft: '4px' }} />
              )}
            </span>
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
              {isEmployer ? 'Horas del equipo (semana)' : 'Esta semana'}{' '}
              <Tooltip content={isEmployer ? "Horas que los profesionales han registrado a lo largo de la semana" : "Horas que registraste esta semana"} size={12} style={{ marginLeft: '4px' }} />
            </span>
            <span className={styles['stat-value']}>{weekHours.toFixed(1)}h</span>
          </div>
        </div>
        <div className={`${styles['stat-card-mini']} ${styles['approved']}`}>
          <span className={styles['stat-icon']}><CheckCircle2 size={20} /></span>
          <div className={styles['stat-info']}>
            <span className={styles['stat-label']}>
              {isEmployer ? 'Horas aprobadas (mes)' : 'Aprobadas'}{' '}
              <Tooltip content={isEmployer ? "Horas registradas por los profesionales que ya aprobaste" : "Horas registradas que ya fueron aprobadas"} size={12} style={{ marginLeft: '4px' }} />
            </span>
            <span className={styles['stat-value']}>{summary.approved_hours.toFixed(1)}h</span>
          </div>
        </div>
        <div className={`${styles['stat-card-mini']} ${styles['pending']}`}>
          <span className={styles['stat-icon']}><Hourglass size={20} /></span>
          <div className={styles['stat-info']}>
            <span className={styles['stat-label']}>
              {isEmployer ? 'Horas pendientes (mes)' : 'Pendientes'}{' '}
              <Tooltip content={isEmployer ? "Horas registradas por los profesionales que tienes pendientes por aprobar" : "Horas registradas que no han sido aprobadas"} size={12} style={{ marginLeft: '4px' }} />
            </span>
            <span className={styles['stat-value']}>{summary.pending_hours.toFixed(1)}h</span>
          </div>
        </div>
        <div className={`${styles['stat-card-mini']} ${styles['absence'] || 'absence'}`}>
          <span className={styles['stat-icon']}><AlertCircle size={20} /></span>
          <div className={styles['stat-info']}>
            <span className={styles['stat-label']}>
              {isEmployer ? 'Hrs a recuperar (equipo)' : 'Hrs a recuperar'}{' '}
              <Tooltip content={isEmployer ? "Total de horas de ausencia registradas por el equipo este mes que se pueden recuperar" : "Total de horas de ausencia que tienes pendientes de recuperar este mes"} size={12} style={{ marginLeft: '4px' }} />
            </span>
            <span className={styles['stat-value']}>{absenceHoursToRecover.toFixed(1)}h</span>
          </div>
        </div>
      </div>
      {showRightBtn && (
        <button className={styles['scroll-btn-right']} onClick={scrollRight} aria-label="Ver siguientes tarjetas">
          <ChevronRight size={20} />
        </button>
      )}
    </div>
  )
}
