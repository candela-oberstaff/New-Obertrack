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
    const map: Record<string, { hours: number; type: 'complete' | 'absence' | null }> = {}
    workHours.forEach(wh => {
      const date = wh.work_date.split('T')[0]
      if (wh.work_type === 'complete' || wh.hours_worked >= JORNADA_COMPLETA) {
        map[date] = { hours: wh.hours_worked, type: 'complete' }
      } else if (wh.work_type === 'absence' || wh.hours_worked === 0) {
        map[date] = { hours: 0, type: 'absence' }
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
