import { useMemo, useState } from 'react'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import type { Task, ColumnType } from '../../../types'
import { parseDateOnly } from '../../../utils/date'
import styles from '../../../pages/Tasks.module.css'

const WEEKDAYS = ['Lun', 'Mar', 'Mié', 'Jue', 'Vie', 'Sáb', 'Dom']

function dayKey(d: Date): string {
  return `${d.getFullYear()}-${d.getMonth() + 1}-${d.getDate()}`
}

interface TasksCalendarViewProps {
  tasks: Task[]
  columns: ColumnType[]
  onTaskClick: (task: Task) => void
}

export function TasksCalendarView({ tasks, columns, onTaskClick }: TasksCalendarViewProps) {
  const [monthStart, setMonthStart] = useState(() => {
    const now = new Date()
    return new Date(now.getFullYear(), now.getMonth(), 1)
  })

  const columnById = useMemo(() => new Map(columns.map((c) => [c.id, c])), [columns])

  // Cuadrícula de 6 semanas empezando en el lunes de la semana del día 1.
  const days = useMemo(() => {
    const first = new Date(monthStart)
    const offset = (first.getDay() + 6) % 7 // lunes = 0
    first.setDate(first.getDate() - offset)
    return Array.from({ length: 42 }, (_, i) => {
      const d = new Date(first)
      d.setDate(d.getDate() + i)
      return d
    })
  }, [monthStart])

  // Tareas agrupadas por día de su fecha límite.
  const tasksByDay = useMemo(() => {
    const map = new Map<string, Task[]>()
    for (const t of tasks) {
      if (!t.end_date) continue
      const key = dayKey(parseDateOnly(t.end_date))
      const list = map.get(key)
      if (list) list.push(t)
      else map.set(key, [t])
    }
    return map
  }, [tasks])

  const noDateTasks = useMemo(() => tasks.filter((t) => !t.end_date), [tasks])

  const todayKey = dayKey(new Date())
  const monthLabel = monthStart.toLocaleDateString('es-ES', { month: 'long', year: 'numeric' })

  const shiftMonth = (delta: number) => {
    setMonthStart((prev) => new Date(prev.getFullYear(), prev.getMonth() + delta, 1))
  }

  const goToday = () => {
    const now = new Date()
    setMonthStart(new Date(now.getFullYear(), now.getMonth(), 1))
  }

  const renderChip = (task: Task) => {
    const column = columnById.get(task.status)
    return (
      <button
        key={task.id}
        type="button"
        className={`${styles['calendar-chip']} ${task.completed ? styles['calendar-chip-done'] || '' : ''}`}
        style={{ ['--chip-color' as any]: column?.color || '#6b7280' }}
        onClick={() => onTaskClick(task)}
        title={task.title}
      >
        {task.title}
      </button>
    )
  }

  return (
    <div className={styles['tasks-calendar-view']}>
      <div className={styles['calendar-header']}>
        <div className={styles['calendar-nav']}>
          <button type="button" className={styles['btn-icon']} onClick={() => shiftMonth(-1)} title="Mes anterior">
            <ChevronLeft size={18} />
          </button>
          <h2 className={styles['calendar-month']}>{monthLabel}</h2>
          <button type="button" className={styles['btn-icon']} onClick={() => shiftMonth(1)} title="Mes siguiente">
            <ChevronRight size={18} />
          </button>
        </div>
        <button type="button" className={styles['btn-secondary'] || 'btn-secondary'} onClick={goToday}>
          Hoy
        </button>
      </div>

      <div className={styles['calendar-grid']}>
        {WEEKDAYS.map((d) => (
          <div key={d} className={styles['calendar-weekday']}>{d}</div>
        ))}
        {days.map((d) => {
          const key = dayKey(d)
          const inMonth = d.getMonth() === monthStart.getMonth()
          const dayTasks = tasksByDay.get(key) || []
          return (
            <div
              key={key}
              className={`${styles['calendar-day']} ${inMonth ? '' : styles['calendar-day-out'] || ''} ${key === todayKey ? styles['calendar-day-today'] || '' : ''}`}
            >
              <span className={styles['calendar-day-number']}>{d.getDate()}</span>
              <div className={styles['calendar-day-tasks']}>
                {dayTasks.slice(0, 4).map(renderChip)}
                {dayTasks.length > 4 && (
                  <span className={styles['calendar-more']}>+{dayTasks.length - 4} más</span>
                )}
              </div>
            </div>
          )
        })}
      </div>

      {noDateTasks.length > 0 && (
        <div className={styles['calendar-no-date']}>
          <span className={styles['calendar-no-date-label']}>Sin fecha límite ({noDateTasks.length})</span>
          <div className={styles['calendar-no-date-chips']}>
            {noDateTasks.map(renderChip)}
          </div>
        </div>
      )}
    </div>
  )
}
