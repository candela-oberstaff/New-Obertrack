import { useEffect, useState, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { taskService, workHourService, userService } from '../services/api'
import type { Task, WorkHour, User } from '../types'
import {
  Clock,
  CheckCircle2,
  AlertTriangle,
  CheckSquare,
  ClipboardList,
  MessageSquare,
  User as UserIcon
} from 'lucide-react'
import styles from './Dashboard.module.css'

const MONTHS_ES = ['Enero', 'Febrero', 'Marzo', 'Abril', 'Mayo', 'Junio', 'Julio', 'Agosto', 'Septiembre', 'Octubre', 'Noviembre', 'Diciembre']
const DAYS_ES = ['Domingo', 'Lunes', 'Martes', 'Miércoles', 'Jueves', 'Viernes', 'Sábado']

const parseLocalDate = (dateStr: string): Date => {
  const [year, month, day] = dateStr.split('T')[0].split('-').map(Number)
  return new Date(year, month - 1, day)
}

export default function Dashboard() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const [tasks, setTasks] = useState<Task[]>([])
  const [workHours, setWorkHours] = useState<WorkHour[]>([])
  const [employees, setEmployees] = useState<User[]>([])
  const [summary, setSummary] = useState({ total_hours: 0, approved_hours: 0, pending_hours: 0 })
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    fetchData()
  }, [user])

  const fetchData = async () => {
    try {
      const [tasksData, workHoursData, summaryData] = await Promise.allSettled([
        taskService.getAll({ limit: 10 }),
        workHourService.getAll({ limit: 10 }),
        workHourService.getSummary(),
      ])

      console.log('Dashboard - workHours:', workHoursData.status === 'fulfilled' ? workHoursData.value : workHoursData.reason)
      console.log('Dashboard - summary:', summaryData.status === 'fulfilled' ? summaryData.value : summaryData.reason)
      console.log('Dashboard - tasks:', tasksData.status === 'fulfilled' ? tasksData.value : tasksData.reason)

      if (tasksData.status === 'fulfilled') {
        setTasks(tasksData.value?.data || [])
      }
      if (workHoursData.status === 'fulfilled') {
        setWorkHours(workHoursData.value?.data || [])
      }
      if (summaryData.status === 'fulfilled') {
        setSummary(summaryData.value || { total_hours: 0, approved_hours: 0, pending_hours: 0 })
      }

      if (user?.is_superadmin || user?.is_manager || user?.user_type === 'empleador') {
        const employeesData = await userService.getEmployees()
        setEmployees(employeesData || [])
      }
    } catch (error) {
      console.error('Error fetching dashboard data:', error)
    } finally {
      setIsLoading(false)
    }
  }

  const today = new Date()
  const greeting = today.getHours() < 12 ? 'Buenos días' : today.getHours() < 18 ? 'Buenas tardes' : 'Buenas noches'

  const weekData = useMemo(() => {
    const days = []
    const startOfWeek = new Date(today)
    startOfWeek.setDate(today.getDate() - today.getDay())
    startOfWeek.setHours(0, 0, 0, 0)

    for (let i = 0; i < 7; i++) {
      const date = new Date(startOfWeek)
      date.setDate(startOfWeek.getDate() + i)
      const dateStr = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
      const dayHours = workHours
        .filter(wh => wh.work_date.split('T')[0] === dateStr)
        .reduce((sum, wh) => sum + wh.hours_worked, 0)
      days.push({
        day: DAYS_ES[i].slice(0, 3),
        hours: dayHours,
        target: 8
      })
    }
    return days
  }, [workHours, today])

  const maxHours = Math.max(...weekData.map(d => Math.max(d.hours, d.target)), 8)

  const pendingTasks = tasks.filter(t => t.status !== 'finalizado')
  const completedTasks = tasks.filter(t => t.status === 'finalizado')

  const getPriorityColor = (priority: string) => {
    const colors: Record<string, string> = {
      urgent: '#ef4444',
      high: '#f97316',
      medium: '#eab308',
      low: '#22c55e',
    }
    return colors[priority] || '#6b7280'
  }

  const getStatusLabel = (status: string) => {
    const labels: Record<string, string> = {
      por_hacer: 'Por hacer',
      en_proceso: 'En proceso',
      finalized: 'Finalizado',
    }
    return labels[status] || status
  }

  if (isLoading) {
    return (
      <div className={styles['dashboard-loading']}>
        <div className={styles['spinner']} />
        <p>Cargando dashboard...</p>
      </div>
    )
  }

  return (
    <div className={styles['dashboard']}>
      <div className={styles['dashboard-header']}>
        <div className={styles['header-content']}>
          <div className={styles['greeting']}>
            <span className={styles['greeting-text']}>{greeting}, {user?.name?.split(' ')[0]}</span>

            <p className={styles['date-text']}>
              {DAYS_ES[today.getDay()]}, {today.getDate()} de {MONTHS_ES[today.getMonth()]}
            </p>
          </div>
        </div>
        <div className={styles['header-actions'] || 'header-actions'}>
          <button className={styles['btn-quick-action']} onClick={() => navigate('/tasks')}>
            + Nueva Tarea
          </button>
        </div>
      </div>

      <div className={styles['stats-row']}>
        <div className={styles['stat-card-large']}>
          <div className={`${styles['stat-icon']} ${styles['blue']}`}>
            <Clock size={24} />
          </div>
          <div className={styles['stat-content']}>
            <span className={styles['stat-label']}>Horas esta semana</span>
            <span className={styles['stat-value']}>{weekData.reduce((s, d) => s + d.hours, 0).toFixed(1)}h</span>
          </div>
        </div>

        <div className={styles['stat-card-large']}>
          <div className={`${styles['stat-icon']} ${styles['green']}`}>
            <CheckCircle2 size={24} />
          </div>
          <div className={styles['stat-content']}>
            <span className={styles['stat-label']}>Horas aprobadas</span>
            <span className={styles['stat-value']}>{summary.approved_hours.toFixed(1)}h</span>
          </div>
        </div>

        <div className={styles['stat-card-large']}>
          <div className={`${styles['stat-icon']} ${styles['orange']}`}>
            <AlertTriangle size={24} />
          </div>
          <div className={styles['stat-content']}>
            <span className={styles['stat-label']}>Pendientes</span>
            <span className={styles['stat-value']}>{summary.pending_hours.toFixed(1)}h</span>
          </div>
        </div>

        <div className={styles['stat-card-large']}>
          <div className={`${styles['stat-icon']} ${styles['purple']}`}>
            <CheckSquare size={24} />
          </div>
          <div className={styles['stat-content']}>
            <span className={styles['stat-label']}>Tareas completadas</span>
            <span className={styles['stat-value']}>{completedTasks.length}</span>
          </div>
        </div>
      </div>

      <div className={styles['dashboard-grid']}>
        <div className={`${styles['dashboard-card']} ${styles['chart-card']}`}>
          <div className={styles['card-header']}>
            <h3>Horas de la semana</h3>
            <span className={styles['card-subtitle']}>Meta: 8h/día</span>
          </div>
          <div className={styles['chart-container']}>
            <div className={styles['bar-chart']}>
              {weekData.map((day, index) => (
                <div key={index} className={styles['bar-column']}>
                  <div className={styles['bar-wrapper']}>
                    <div
                      className={`${styles['bar']} ${styles['hours-bar']}`}
                      style={{ height: `${(day.hours / maxHours) * 100}%` }}
                    >
                      {day.hours > 0 && <span className={styles['bar-value']}>{day.hours}h</span>}
                    </div>
                    <div
                      className={`${styles['bar']} ${styles['target-bar']}`}
                      style={{ height: `${(day.target / maxHours) * 100}%` }}
                    />
                  </div>
                  <span className={styles['bar-label']}>{day.day}</span>
                </div>
              ))}
            </div>
          </div>
        </div>

        <div className={`${styles['dashboard-card']} ${styles['tasks-card'] || 'tasks-card'}`}>
          <div className={styles['card-header']}>
            <h3>Próximas tareas</h3>
            <button className={styles['btn-link']} onClick={() => navigate('/tasks')}>Ver todas</button>
          </div>
          <div className={styles['tasks-list']}>
            {pendingTasks.length === 0 ? (
              <div className={styles['empty-card']}>
                <CheckCircle2 size={40} style={{ color: '#22c55e', marginBottom: '12px' }} />
                <p>No hay tareas pendientes</p>
              </div>
            ) : (
              pendingTasks.slice(0, 5).map(task => (
                <div key={task.id} className={styles['task-row']} onClick={() => navigate('/tasks')}>
                  <div
                    className={styles['task-priority-dot']}
                    style={{ backgroundColor: getPriorityColor(task.priority) }}
                  />
                  <div className={styles['task-details']}>
                    <span className={styles['task-title']}>{task.title}</span>
                    <span className={styles['task-meta']}>
                      {getStatusLabel(task.status)}
                      {task.end_date && ` • ${new Date(task.end_date).toLocaleDateString('es-ES', { day: 'numeric', month: 'short' })}`}
                    </span>
                  </div>
                </div>
              ))
            )}
          </div>
        </div>

        <div className={`${styles['dashboard-card']} ${styles['hours-card'] || 'hours-card'}`}>
          <div className={styles['card-header']}>
            <h3>Registro reciente</h3>
            <button className={styles['btn-link']} onClick={() => navigate('/work-hours')}>Ver todas</button>
          </div>
          <div className={styles['hours-list']}>
            {workHours.length === 0 ? (
              <div className={styles['empty-card']}>
                <ClipboardList size={40} style={{ color: '#94a3b8', marginBottom: '12px' }} />
                <p>Sin registros esta semana</p>
              </div>
            ) : (
              workHours.slice(0, 5).map(wh => (
                <div key={wh.id} className={styles['hour-row']}>
                  <div className={styles['hour-date-badge']}>
                    <span className={styles['day']}>{parseLocalDate(wh.work_date).getDate()}</span>
                    <span className={styles['month']}>{MONTHS_ES[parseLocalDate(wh.work_date).getMonth()].slice(0, 3)}</span>
                  </div>
                  <div className={styles['hour-info']}>
                    <span className={styles['hours-value']}>
                      {wh.hours_worked >= 8 ? 'Jornada completa' : wh.hours_worked > 0 ? `${wh.hours_worked}h` : 'Ausencia'}
                    </span>
                    <span className={`${styles['hours-status']} ${wh.approved ? styles['approved'] : styles['pending']}`}>
                      {wh.approved ? 'Aprobado' : 'Pendiente'}
                    </span>
                  </div>
                </div>
              ))
            )}
          </div>
        </div>

        {(user?.is_superadmin || user?.is_manager || user?.user_type === 'empleador') && employees.length > 0 && (
          <div className={`${styles['dashboard-card']} ${styles['team-card'] || 'team-card'}`}>
            <div className={styles['card-header']}>
              <h3>Equipo de trabajo</h3>
              <span className={styles['card-badge']}>{employees.length}</span>
            </div>
            <div className={styles['team-list']}>
              {employees.slice(0, 6).map(emp => (
                <div key={emp.id} className={styles['team-member']}>
                  <div className={styles['member-avatar']}>
                    {emp.name?.charAt(0).toUpperCase()}
                  </div>
                  <div className={styles['member-info']}>
                    <span className={styles['member-name']}>{emp.name}</span>
                    <span className={styles['member-role']}>{emp.job_title || emp.user_type}</span>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      <div className={styles['quick-actions']}>
        <button className={styles['action-card']} onClick={() => navigate('/tasks')}>
          <span className={styles['action-icon']}><ClipboardList size={24} /></span>
          <span className={styles['action-label']}>Tareas</span>
        </button>
        <button className={styles['action-card']} onClick={() => navigate('/work-hours')}>
          <span className={styles['action-icon']}><Clock size={24} /></span>
          <span className={styles['action-label']}>Horas</span>
        </button>
        <button className={styles['action-card']} onClick={() => navigate('/chat')}>
          <span className={styles['action-icon']}><MessageSquare size={24} /></span>
          <span className={styles['action-label']}>Chat</span>
        </button>
        <button className={styles['action-card']} onClick={() => navigate('/profile')}>
          <span className={styles['action-icon']}><UserIcon size={24} /></span>
          <span className={styles['action-label']}>Perfil</span>
        </button>
      </div>
    </div>
  )
}
