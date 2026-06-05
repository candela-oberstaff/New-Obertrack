import { useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { useDashboard } from '../hooks'
import {
  Clock,
  CheckCircle2,
  AlertTriangle,
  CheckSquare,
  ClipboardList,
  MessageSquare,
  User as UserIcon
} from 'lucide-react'
import Tooltip from '../components/Common/Tooltip'
import { TeamPanel } from '../components/Profile/TeamPanel'
import styles from './Dashboard.module.css'

const MONTHS_ES = ['Enero', 'Febrero', 'Marzo', 'Abril', 'Mayo', 'Junio', 'Julio', 'Agosto', 'Septiembre', 'Octubre', 'Noviembre', 'Diciembre']
const DAYS_ES = ['Domingo', 'Lunes', 'Martes', 'Miércoles', 'Jueves', 'Viernes', 'Sábado']

export default function Dashboard() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const {
    workHours,
    summary,
    isLoading,
    weekData,
    maxHours,
    pendingTasks,
    completedTasks,
    getPriorityColor,
    getStatusLabel,
  } = useDashboard(user)

  const today = useMemo(() => new Date(), [])
  const greeting = today.getHours() < 12 ? 'Buenos días' : today.getHours() < 18 ? 'Buenas tardes' : 'Buenas noches'

  const isEmployer = user?.user_type === 'empleador' || user?.is_superadmin || user?.is_manager

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
      <div className={styles['dashboard-header']} data-tour="dashboard-header">
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

      <div className={styles['stats-row']} data-tour="dashboard-stats">
        <div className={styles['stat-card-large']}>
          <div className={`${styles['stat-icon']} ${styles['blue']}`}>
            <Clock size={24} />
          </div>
          <div className={styles['stat-content']}>
            <span className={styles['stat-label']}>
              Horas esta semana
              <Tooltip content={isEmployer ? "Horas que los profesionales han registrado esta semana" : "Horas que registraste esta semana"} size={14} />
            </span>
            <span className={styles['stat-value']}>{weekData.reduce((s, d) => s + d.hours, 0).toFixed(1)}h</span>
          </div>
        </div>

        <div className={styles['stat-card-large']}>
          <div className={`${styles['stat-icon']} ${styles['green']}`}>
            <CheckCircle2 size={24} />
          </div>
          <div className={styles['stat-content']}>
            <span className={styles['stat-label']}>
              Horas aprobadas
              <Tooltip content={isEmployer ? "Horas registradas por los profesionales que ya fueron aprobadas" : "Horas registradas que ya fueron aprobadas"} size={14} />
            </span>
            <span className={styles['stat-value']}>{summary.approved_hours.toFixed(1)}h</span>
          </div>
        </div>

        <div className={styles['stat-card-large']}>
          <div className={`${styles['stat-icon']} ${styles['orange']}`}>
            <AlertTriangle size={24} />
          </div>
          <div className={styles['stat-content']}>
            <span className={styles['stat-label']}>
              Pendientes
              <Tooltip content={isEmployer ? "Horas registradas por los profesionales que no han sido aprobadas" : "Horas registradas que no han sido aprobadas"} size={14} />
            </span>
            <span className={styles['stat-value']}>{summary.pending_hours.toFixed(1)}h</span>
          </div>
        </div>

        <div className={styles['stat-card-large']}>
          <div className={`${styles['stat-icon']} ${styles['purple']}`}>
            <CheckSquare size={24} />
          </div>
          <div className={styles['stat-content']}>
            <span className={styles['stat-label']}>
              Tareas completadas
              <Tooltip content={isEmployer ? "Tareas completadas por el equipo este mes" : "Tareas que completaste este mes"} size={14} />
            </span>
            <span className={styles['stat-value']}>{completedTasks.length}</span>
          </div>
        </div>
      </div>

      <div className={styles['dashboard-grid']}>
        <div className={`${styles['dashboard-card']} ${styles['chart-card']}`} data-tour="dashboard-hours-chart">
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

        <div className={`${styles['dashboard-card']} ${styles['tasks-card'] || 'tasks-card'}`} data-tour="dashboard-tasks-card">
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

        <div className={`${styles['dashboard-card']} ${styles['hours-card'] || 'hours-card'}`} data-tour="dashboard-hours-card">
          <div className={styles['card-header']}>
            <h3>
              Registro reciente
              <Tooltip content={isEmployer ? "Horas registradas recientemente por los profesionales" : "Horas registradas recientemente"} size={14} />
            </h3>
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
                    <span className={styles['day']}>{new Date(wh.work_date).getDate()}</span>
                    <span className={styles['month']}>{MONTHS_ES[new Date(wh.work_date).getMonth()].slice(0, 3)}</span>
                  </div>
                  <div className={styles['hour-info']}>
                    <span className={styles['hours-value']}>
                      {wh.hours_worked >= 8 ? 'Jornada completa' : wh.hours_worked > 0 ? `${wh.hours_worked}h` : 'Ausencia'}
                      {isEmployer && wh.user?.name && (
                        <span className={styles['hour-author']}> — {wh.user.name}</span>
                      )}
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

        {(user?.is_superadmin || user?.is_manager || user?.user_type === 'empleador') && (
          <div data-tour="dashboard-team-card">
            {user?.is_manager && !user?.is_superadmin && !user?.user_type?.includes('empleador') && (
              <TeamPanel type="manager" />
            )}
            {(user?.user_type === 'empleador' || user?.is_superadmin) && (
              <TeamPanel type="employer" />
            )}
          </div>
        )}
      </div>

      <div className={styles['quick-actions']} data-tour="dashboard-quick-actions">
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
