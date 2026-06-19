import { useMemo } from 'react'
import type { WorkHour } from '../../types'
import { CheckCircle2, AlertTriangle } from 'lucide-react'
import { DAYS_ES, JORNADA_COMPLETA, parseLocalDate } from './utils'
import styles from '../../pages/WorkHours.module.css'

interface WorkHourCalendarProps {
  workHours: WorkHour[]
  onDayClick: (date: string) => void
  selectedDate: string | null
  currentMonth: number
  currentYear: number
}

export function WorkHourCalendar({
  workHours,
  onDayClick,
  selectedDate,
  currentMonth,
  currentYear
}: WorkHourCalendarProps) {
  const firstDay = new Date(currentYear, currentMonth, 1).getDay()
  const daysInMonth = new Date(currentYear, currentMonth + 1, 0).getDate()

  const dayInfo = useMemo(() => {
    // Se agrega por día (puede haber varios registros el mismo día en vista de
    // equipo). Prioridad determinista: una ausencia prevalece sobre una jornada
    // completa, así un día con ausencias no queda oculto por otro registro;
    // antes "ganaba" el último del arreglo, lo que pintaba el día de forma
    // engañosa.
    const map: Record<string, { type: 'complete' | 'absence' | null }> = {}
    workHours.forEach(wh => {
      const date = wh.work_date.split('T')[0]
      if (map[date]?.type === 'absence') return
      const isAbsence = wh.work_type === 'absence' || wh.hours_worked === 0
      const isComplete = wh.work_type === 'complete' || wh.hours_worked >= JORNADA_COMPLETA
      if (isAbsence) {
        map[date] = { type: 'absence' }
      } else if (isComplete) {
        map[date] = { type: 'complete' }
      } else if (!map[date]) {
        map[date] = { type: null }
      }
    })
    return map
  }, [workHours])

  const days = []
  for (let i = 0; i < firstDay; i++) {
    days.push(<div key={`empty-${i}`} className={`${styles['calendar-day']} ${styles['empty']}`} />)
  }
  
  for (let day = 1; day <= daysInMonth; day++) {
    const dateStr = `${currentYear}-${String(currentMonth + 1).padStart(2, '0')}-${String(day).padStart(2, '0')}`
    const info = dayInfo[dateStr]
    const isSelected = selectedDate === dateStr
    const dayOfWeek = parseLocalDate(dateStr).getDay()
    const isWeekend = dayOfWeek === 0 || dayOfWeek === 6

    days.push(
      <div
        key={day}
        className={`${styles['calendar-day']} ${info?.type ? styles[info.type] : ''} ${isSelected ? styles['selected'] : ''} ${isWeekend ? styles['weekend'] : ''}`}
        onClick={() => onDayClick(dateStr)}
      >
        <span className={styles['day-number']}>{day}</span>
        {info?.type === 'complete' && <span className={`${styles['day-badge']} ${styles['complete']}`}><CheckCircle2 size={12} /></span>}
        {info?.type === 'absence' && <span className={`${styles['day-badge']} ${styles['absence']}`}><AlertTriangle size={12} /></span>}
      </div>
    )
  }

  return (
    <div className={styles['calendar-grid']}>
      {DAYS_ES.map(day => (
        <div key={day} className={styles['calendar-header-day']}>{day}</div>
      ))}
      {days}
    </div>
  )
}
