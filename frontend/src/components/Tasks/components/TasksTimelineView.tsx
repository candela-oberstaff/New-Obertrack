import { useEffect, useMemo, useRef } from 'react'
import type { Task, ColumnType } from '../../../types'
import { parseDateOnly, todayMidnight } from '../../../utils/date'
import { compareTasks } from '../taskSort'
import styles from '../../../pages/Tasks.module.css'

/** Ancho en px de un día en el eje horizontal. */
const DAY_WIDTH = 34
const MS_DAY = 86400000

interface TasksTimelineViewProps {
  tasks: Task[]
  columns: ColumnType[]
  onTaskClick: (task: Task) => void
}

/**
 * Timeline estilo Gantt: una fila por tarea, barra desde la fecha de inicio
 * hasta la fecha límite (si falta una de las dos, la barra dura un día), eje de
 * días con meses arriba y línea vertical marcando hoy. Las tareas sin ninguna
 * fecha van en una franja aparte debajo.
 */
export function TasksTimelineView({ tasks, columns, onTaskClick }: TasksTimelineViewProps) {
  const scrollRef = useRef<HTMLDivElement>(null)

  const columnById = useMemo(() => new Map(columns.map((c) => [c.id, c])), [columns])
  const statusRank = useMemo(() => new Map(columns.map((c, i) => [c.id, i])), [columns])

  const { dated, undated } = useMemo(() => {
    const dated: Task[] = []
    const undated: Task[] = []
    for (const t of tasks) {
      if (t.start_date || t.end_date) dated.push(t)
      else undated.push(t)
    }
    // Agrupadas por fase (en el orden del tablero) y dentro por su orden manual.
    dated.sort((a, b) => {
      const rank = (statusRank.get(a.status) ?? 99) - (statusRank.get(b.status) ?? 99)
      if (rank !== 0) return rank
      return compareTasks(a, b)
    })
    return { dated, undated }
  }, [tasks, statusRank])

  const today = todayMidnight().getTime()

  // Rango del eje: de la fecha más temprana a la más tardía (incluyendo hoy),
  // con un colchón a cada lado, empezando en lunes.
  const { rangeStart, days } = useMemo(() => {
    let min = today
    let max = today
    for (const t of dated) {
      const s = parseDateOnly(t.start_date || t.end_date!).getTime()
      const e = parseDateOnly(t.end_date || t.start_date!).getTime()
      if (s < min) min = s
      if (e > max) max = e
    }
    min -= 3 * MS_DAY
    max += 7 * MS_DAY
    const start = new Date(min)
    start.setDate(start.getDate() - ((start.getDay() + 6) % 7)) // alinear a lunes
    start.setHours(0, 0, 0, 0)
    const total = Math.round((max - start.getTime()) / MS_DAY) + 1
    const days = Array.from({ length: total }, (_, i) => {
      const d = new Date(start)
      d.setDate(d.getDate() + i)
      return d
    })
    return { rangeStart: start.getTime(), days }
  }, [dated, today])

  // Meses del encabezado: [{ label, count de días }].
  const months = useMemo(() => {
    const out: { label: string; count: number }[] = []
    for (const d of days) {
      const label = d.toLocaleDateString('es-ES', { month: 'long', year: 'numeric' })
      const last = out[out.length - 1]
      if (last && last.label === label) last.count++
      else out.push({ label, count: 1 })
    }
    return out
  }, [days])

  const dayIndex = (ms: number) => Math.round((ms - rangeStart) / MS_DAY)
  const todayIdx = dayIndex(today)

  // Al montar, centrar el scroll cerca de hoy.
  useEffect(() => {
    const el = scrollRef.current
    if (el) {
      el.scrollLeft = Math.max(0, todayIdx * DAY_WIDTH - el.clientWidth / 3)
    }
  }, [todayIdx])

  const trackWidth = days.length * DAY_WIDTH

  const renderBar = (task: Task) => {
    const column = columnById.get(task.status)
    const startMs = parseDateOnly(task.start_date || task.end_date!).getTime()
    const endMs = parseDateOnly(task.end_date || task.start_date!).getTime()
    const left = dayIndex(Math.min(startMs, endMs)) * DAY_WIDTH
    const width = (Math.abs(dayIndex(endMs) - dayIndex(startMs)) + 1) * DAY_WIDTH
    const overdue = !!task.end_date && !task.completed && endMs < today
    return (
      <button
        type="button"
        className={`${styles['timeline-bar']} ${task.completed ? styles['timeline-bar-done'] || '' : ''} ${overdue ? styles['timeline-bar-overdue'] || '' : ''}`}
        style={{ left, width, ['--chip-color' as any]: column?.color || '#6b7280' }}
        onClick={() => onTaskClick(task)}
        title={`${task.title} · ${column?.title || task.status}`}
      >
        {task.title}
      </button>
    )
  }

  return (
    <div className={styles['tasks-timeline-view']}>
      <div className={styles['timeline-scroll']} ref={scrollRef}>
        <div style={{ minWidth: 'max-content' }}>
          {/* Encabezado: meses y días */}
          <div className={styles['timeline-row']}>
            <div className={`${styles['timeline-label']} ${styles['timeline-label-header']}`} />
            <div className={styles['timeline-track']} style={{ width: trackWidth }}>
              <div className={styles['timeline-months']}>
                {months.map((m, i) => (
                  <div key={i} className={styles['timeline-month']} style={{ width: m.count * DAY_WIDTH }}>
                    {m.label}
                  </div>
                ))}
              </div>
            </div>
          </div>
          <div className={styles['timeline-row']}>
            <div className={`${styles['timeline-label']} ${styles['timeline-label-header']}`} />
            <div className={styles['timeline-track']} style={{ width: trackWidth }}>
              <div className={styles['timeline-days']}>
                {days.map((d, i) => {
                  const weekend = d.getDay() === 0 || d.getDay() === 6
                  return (
                    <div
                      key={i}
                      className={`${styles['timeline-day']} ${weekend ? styles['timeline-day-weekend'] || '' : ''} ${i === todayIdx ? styles['timeline-day-today'] || '' : ''}`}
                      style={{ width: DAY_WIDTH }}
                    >
                      {d.getDate()}
                    </div>
                  )
                })}
              </div>
            </div>
          </div>

          {/* Filas de tareas */}
          <div className={styles['timeline-body']}>
            {todayIdx >= 0 && todayIdx < days.length && (
              <div
                className={styles['timeline-today-line']}
                style={{ left: `calc(var(--timeline-label-w) + ${todayIdx * DAY_WIDTH + DAY_WIDTH / 2}px)` }}
              />
            )}
            {dated.map((task) => {
              const column = columnById.get(task.status)
              return (
                <div key={task.id} className={styles['timeline-row']}>
                  <div
                    className={styles['timeline-label']}
                    onClick={() => onTaskClick(task)}
                    title={task.title}
                  >
                    <span className={styles['column-dot']} style={{ backgroundColor: column?.color || '#6b7280' }} />
                    <span className={task.completed ? styles['timeline-label-done'] || '' : ''}>{task.title}</span>
                  </div>
                  <div className={styles['timeline-track']} style={{ width: trackWidth }}>
                    {renderBar(task)}
                  </div>
                </div>
              )
            })}
            {dated.length === 0 && (
              <div className={styles['timeline-empty']}>No hay tareas con fechas en este tablero</div>
            )}
          </div>
        </div>
      </div>

      {undated.length > 0 && (
        <div className={styles['calendar-no-date']}>
          <span className={styles['calendar-no-date-label']}>Sin fechas ({undated.length})</span>
          <div className={styles['calendar-no-date-chips']}>
            {undated.map((task) => {
              const column = columnById.get(task.status)
              return (
                <button
                  key={task.id}
                  type="button"
                  className={styles['calendar-chip']}
                  style={{ ['--chip-color' as any]: column?.color || '#6b7280' }}
                  onClick={() => onTaskClick(task)}
                  title={task.title}
                >
                  {task.title}
                </button>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}
