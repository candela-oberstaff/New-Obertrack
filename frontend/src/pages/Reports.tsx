import {
  BarChart2,
  ChevronLeft,
  ChevronRight,
  Clock,
  CheckSquare,
  CheckCircle2,
  AlertCircle,
  Calendar,
  Check
} from 'lucide-react'
import Tooltip from '../components/Common/Tooltip'
import { useAuth } from '../context/AuthContext'
import { useReports } from '../hooks/useReports'
import { StatCard } from '../components/Common/StatCard'
import { ReportsCharts } from '../components/Reports/ReportsCharts'
import { Select } from '../components/ui/Select'
import styles from './Reports.module.css'

export default function Reports() {
  const { user } = useAuth()
  const {
    employees, selectedEmployee, setSelectedEmployee,
    workHours, tasks, month, setMonth,
    isLoading, reportType, setReportType,
    hoursStats, dailyData, priorityData
  } = useReports(user as any)

  const handlePrevMonth = () => {
    const [year, m] = month.split('-').map(Number)
    const newDate = new Date(year, m - 2)
    setMonth(`${newDate.getFullYear()}-${String(newDate.getMonth() + 1).padStart(2, '0')}`)
  }

  const handleNextMonth = () => {
    const [year, m] = month.split('-').map(Number)
    const newDate = new Date(year, m)
    setMonth(`${newDate.getFullYear()}-${String(newDate.getMonth() + 1).padStart(2, '0')}`)
  }

  return (
    <div className={styles['reports-page']}>
      <div className={styles['page-header']} data-tour="reports-header">
        <div className={styles['header-left']}>
          <h1>
            <BarChart2 size={28} style={{ verticalAlign: 'middle', marginRight: '8px' }} /> Reportes{' '}
            <Tooltip content="Detalles del registro de jornada de los profesionales" size={18} />
          </h1>
          <p className={styles['header-subtitle']}>Análisis de productividad y rendimiento</p>
        </div>
      </div>

      <div className={styles['reports-filters']} data-tour="reports-filters">
        {(user?.is_superadmin || user?.is_manager || user?.user_type === 'empleador') && employees.length > 0 && (
          <Select
            value={selectedEmployee}
            onChange={(v) => setSelectedEmployee(v ? Number(v) : '')}
            clearable
            placeholder="Todos los empleados"
            options={employees.map((emp) => ({ value: emp.id, label: emp.name }))}
          />
        )}

        <div className={styles['month-selector']}>
          <button className={styles['nav-btn']} onClick={handlePrevMonth}><ChevronLeft size={20} /></button>
          <input
            type="month"
            value={month}
            onChange={(e) => setMonth(e.target.value)}
            className={styles['month-input']}
          />
          <button className={styles['nav-btn']} onClick={handleNextMonth}><ChevronRight size={20} /></button>
        </div>

        <div className={styles['report-type-tabs']}>
          <button
            className={`${styles['tab-btn']} ${reportType === 'hours' ? styles['active'] : ''}`}
            onClick={() => setReportType('hours')}
          >
            <Clock size={16} /> Horas
          </button>
          <button
            className={`${styles['tab-btn']} ${reportType === 'tasks' ? styles['active'] : ''}`}
            onClick={() => setReportType('tasks')}
          >
            <CheckSquare size={16} /> Tareas
          </button>
        </div>
      </div>

      {isLoading ? (
        <div className={styles['reports-loading']}>
          <div className={styles['spinner']} />
          <p>Cargando datos...</p>
        </div>
      ) : (
        <>
          {reportType === 'hours' && (
            <div className={styles['reports-content']}>
              <div className={styles['stats-grid']} data-tour="reports-stats">
                {user?.user_type !== 'profesional' && (
                  <>
                    <StatCard 
                      icon={Clock}
                      iconColorClass="blue"
                      label="Horas Totales"
                      value={`${hoursStats.total.toFixed(1)}h`}
                      progressText={`Meta: ${hoursStats.targetHours}h`}
                      tooltip="Horas que los profesionales han registrado a lo largo de la semana"
                    />
                    <StatCard 
                      icon={CheckCircle2}
                      iconColorClass="green"
                      label="Aprobadas"
                      value={`${hoursStats.approved.toFixed(1)}h`}
                      progressText={hoursStats.approved > 0 ? '✓ Verificadas' : 'Sin aprobar'}
                      progressColorClass="success"
                      tooltip="Horas registradas por los profesionales que ya aprobaste"
                    />
                  </>
                )}

                <StatCard 
                  icon={AlertCircle}
                  iconColorClass="orange"
                  label="Pendientes"
                  value={`${hoursStats.pending.toFixed(1)}h`}
                  progressText={hoursStats.pending > 0 ? 'Por aprobar' : 'Sin pendientes'}
                  progressColorClass="warning"
                  tooltip="Horas registradas por los profesionales que tienes pendientes por aprobar"
                />

                <StatCard 
                  icon={Calendar}
                  iconColorClass="purple"
                  label="Días Trabajados"
                  value={hoursStats.daysWorked}
                  progressText={user?.user_type !== 'profesional' ? 'Meta: 20 días' : undefined}
                  tooltip="Cantidad de días que el profesional ha trabajado en el mes"
                />
              </div>

              <div data-tour="reports-charts">
                <ReportsCharts 
                  dailyData={dailyData}
                  workHours={workHours}
                  user={user as any}
                />
              </div>

              <div className={styles['detail-table']} data-tour="reports-detail">
                <h3>
                  Detalle de registros{' '}
                  <Tooltip content="Fecha, hora y estado de los últimos registro hechos por el profesional" size={14} />
                </h3>
                <div className={styles['table-container']}>
                  <table className={styles['data-table']}>
                    <thead>
                      <tr>
                        <th>Fecha</th>
                        <th>{user?.user_type === 'profesional' ? 'Registro' : 'Horas'}</th>
                        <th>Estado</th>
                      </tr>
                    </thead>
                    <tbody>
                      {workHours.length === 0 ? (
                        <tr>
                          <td colSpan={3} className={styles['empty-cell']}>No hay registros</td>
                        </tr>
                      ) : (
                        workHours.slice(0, 10).map((wh) => (
                          <tr key={wh.id}>
                            <td>{new Date(wh.work_date).toLocaleDateString('es-ES', { weekday: 'short', day: 'numeric', month: 'short' })}</td>
                            <td>{user?.user_type === 'profesional' ? '1 Día' : `${wh.hours_worked}h`}</td>
                            <td>
                              <span className={`${styles['status-pill']} ${wh.approved ? styles['approved'] : styles['pending']}`}>
                                {wh.approved ? 'Aprobado' : 'Pendiente'}
                              </span>
                            </td>
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
              </div>
            </div>
          )}

          {reportType === 'tasks' && (
            <div className={styles['reports-content']}>
              <div className={styles['stats-grid']} data-tour="reports-stats">
                <StatCard icon={CheckSquare} iconColorClass="blue" label="Total Tareas" value={tasks.length} />
                <StatCard icon={Check} iconColorClass="green" label="Completadas" value={tasks.filter(t => t.status === 'finalizado').length} />
                <StatCard icon={Clock} iconColorClass="orange" label="En Proceso" value={tasks.filter(t => t.status === 'en_proceso').length} />
                <StatCard icon={AlertCircle} iconColorClass="red" label="Por Hacer" value={tasks.filter(t => t.status === 'por_hacer').length} />
              </div>

              <div className={styles['reports-section']} data-tour="reports-charts">
                <div className={styles['section-header']}>
                  <h3>Distribución de Prioridades</h3>
                </div>
                <div className={styles['priority-summary-bar']}>
                  {['Urgente', 'Alta', 'Media', 'Baja'].map((p) => {
                    const data = priorityData.find(d => d.name === p) || { name: p, value: 0 }
                    const count = data.value
                    const total = tasks.length || 1
                    const width = (count / total) * 100
                    const color = p === 'Urgente' ? '#ef4444' : p === 'Alta' ? '#f97316' : p === 'Media' ? '#f59e0b' : 'var(--primary)'
                    if (count === 0) return null
                    return (
                      <div key={p} className={styles['priority-bar-segment']} style={{ width: `${width}%`, background: color }} title={`${p}: ${count}`}>
                        <span className={styles['segment-label']}>{p} ({count})</span>
                      </div>
                    )
                  })}
                </div>
              </div>

              <div className={styles['reports-section']} data-tour="reports-detail">
                <div className={styles['section-header']}>
                  <h3>Tareas Críticas Pendientes</h3>
                  <span className={styles['badge-count']}>
                    {tasks.filter(t => (t.priority === 'urgent' || t.priority === 'high') && t.status !== 'finalizado').length} Urgentes/Altas
                  </span>
                </div>
                <div className={styles['critical-tasks-container']}>
                  <table className={styles['critical-table']}>
                    <thead>
                      <tr>
                        <th>Tarea</th>
                        <th>Prioridad</th>
                        <th>Estado</th>
                        <th>Creado</th>
                      </tr>
                    </thead>
                    <tbody>
                      {tasks.filter(t => (t.priority === 'urgent' || t.priority === 'high') && t.status !== 'finalizado').length === 0 ? (
                        <tr>
                          <td colSpan={4} className={styles['empty-cell']}>No hay tareas críticas pendientes</td>
                        </tr>
                      ) : (
                        tasks
                          .filter(t => (t.priority === 'urgent' || t.priority === 'high') && t.status !== 'finalizado')
                          .slice(0, 5)
                          .map((t) => (
                            <tr key={t.id}>
                              <td className={styles['task-title-cell']}>{t.title}</td>
                              <td>
                                <span className={`${styles['priority-tag']} ${styles[t.priority] || t.priority}`}>
                                  {t.priority === 'urgent' ? 'Urgente' : 'Alta'}
                                </span>
                              </td>
                              <td>
                                <span className={`${styles['status-pill-small']} ${styles[t.status] || t.status}`}>
                                  {t.status === 'en_proceso' ? 'En proceso' : 'Por hacer'}
                                </span>
                              </td>
                              <td className={styles['task-date-cell']}>{new Date(t.created_at).toLocaleDateString('es-ES')}</td>
                            </tr>
                          ))
                      )}
                    </tbody>
                  </table>
                </div>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  )
}
