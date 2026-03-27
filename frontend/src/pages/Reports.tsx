import { useState, useEffect, useMemo } from 'react'
import { 
  BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer
} from 'recharts'
import { userService, workHourService, taskService } from '../services/api'
import { useAuth } from '../context/AuthContext'
import { WorkHour, Task } from '../types'
import { 
  BarChart2, 
  ChevronLeft, 
  ChevronRight, 
  Clock, 
  CheckSquare, 
  CheckCircle2, 
  AlertCircle, 
  Calendar, 
  FileText,
  Check
} from 'lucide-react'
import './Reports.css'



export default function Reports() {
  const { user } = useAuth()
  const [employees, setEmployees] = useState<{ id: number; name: string }[]>([])
  const [selectedEmployee, setSelectedEmployee] = useState<number | ''>('')
  const [workHours, setWorkHours] = useState<WorkHour[]>([])
  const [tasks, setTasks] = useState<Task[]>([])
  const [month, setMonth] = useState(new Date().toISOString().slice(0, 7))
  const [isLoading, setIsLoading] = useState(false)
  const [reportType, setReportType] = useState<'hours' | 'tasks'>('hours')

  useEffect(() => {
    fetchEmployees()
  }, [user])

  useEffect(() => {
    if (selectedEmployee || user?.user_type === 'profesional' || user?.user_type === 'empleador') {
      fetchData()
    }
  }, [selectedEmployee, month, user])

  const fetchEmployees = async () => {
    if (!user?.is_superadmin && !user?.is_manager && user?.user_type !== 'empleador') return
    try {
      const data = await userService.getEmployees()
      setEmployees(data || [])
    } catch (error) {
      console.error('Error fetching employees:', error)
    }
  }

  const fetchData = async () => {
    setIsLoading(true)
    try {
      const [startDate, endDate] = getMonthRange(month)
      
      let userIdFilter: string | undefined
      if (selectedEmployee) {
        userIdFilter = String(selectedEmployee)
      } else if (user?.user_type === 'profesional') {
        userIdFilter = String(user.id)
      }

      const [hoursRes, tasksRes] = await Promise.allSettled([
        workHourService.getAll({
          user_id: userIdFilter,
          start_date: startDate,
          end_date: endDate,
        }),
        taskService.getAll({})
      ])
      
      console.log('Reports - hours:', hoursRes.status === 'fulfilled' ? hoursRes.value : hoursRes.reason)
      console.log('Reports - tasks:', tasksRes.status === 'fulfilled' ? tasksRes.value : tasksRes.reason)
      
      if (hoursRes.status === 'fulfilled') {
        setWorkHours(hoursRes.value?.data || [])
      }
      if (tasksRes.status === 'fulfilled') {
        setTasks(tasksRes.value?.data || [])
      }
    } catch (error) {
      console.error('Error fetching data:', error)
    } finally {
      setIsLoading(false)
    }
  }

  const getMonthRange = (monthStr: string) => {
    const [year, m] = monthStr.split('-').map(Number)
    const startDate = `${year}-${String(m).padStart(2, '0')}-01`
    const lastDay = new Date(year, m, 0).getDate()
    const endDate = `${year}-${String(m).padStart(2, '0')}-${String(lastDay).padStart(2, '0')}`
    return [startDate, endDate]
  }

  const hoursStats = useMemo(() => {
    const total = workHours.reduce((sum, wh) => sum + wh.hours_worked, 0)
    const approved = workHours.filter(wh => wh.approved).reduce((sum, wh) => sum + wh.hours_worked, 0)
    const pending = total - approved
    const daysWorked = new Set(workHours.map(wh => wh.work_date.split('T')[0])).size
    const targetHours = 160
    
    return { total, approved, pending, daysWorked, targetHours, progress: Math.min((total / targetHours) * 100, 100) }
  }, [workHours])

  const dailyData = useMemo(() => {
    const [year, m] = month.split('-').map(Number)
    const daysInMonth = new Date(year, m - 1 + 1, 0).getDate()
    const days = []
    
    for (let day = 1; day <= daysInMonth; day++) {
      const dateStr = `${month}-${String(day).padStart(2, '0')}`
      const dayHours = workHours
        .filter(wh => wh.work_date.split('T')[0] === dateStr)
        .reduce((sum, wh) => sum + wh.hours_worked, 0)
      days.push({ day: day, hours: dayHours, target: 8 })
    }
    return days
  }, [workHours, month])



  const priorityData = useMemo(() => {
    const PRIORITY_LABELS: Record<string, string> = {
      urgent: 'Urgente',
      high: 'Alta',
      medium: 'Media',
      low: 'Baja'
    }
    const counts: Record<string, number> = { urgent: 0, high: 0, medium: 0, low: 0 }
    tasks.forEach(t => {
      if (counts[t.priority] !== undefined) {
        counts[t.priority]++
      }
    })
    return Object.entries(counts)
      .filter(([_, value]) => value > 0)
      .map(([key, value]) => ({ 
        name: PRIORITY_LABELS[key] || key, 
        value 
      }))
  }, [tasks])



  return (
    <div className="reports-page">
      <div className="page-header">
        <div className="header-left">
          <h1><BarChart2 size={28} style={{ verticalAlign: 'middle', marginRight: '8px' }} /> Reportes</h1>
          <p className="header-subtitle">Análisis de productividad y rendimiento</p>
        </div>
      </div>

      <div className="reports-filters">
        {(user?.is_superadmin || user?.is_manager || user?.user_type === 'empleador') && employees.length > 0 && (
          <select
            value={selectedEmployee}
            onChange={(e) => setSelectedEmployee(e.target.value ? Number(e.target.value) : '')}
            className="filter-select"
          >
            <option value="">Todos los empleados</option>
            {employees.map((emp) => (
              <option key={emp.id} value={emp.id}>{emp.name}</option>
            ))}
          </select>
        )}
        
        <div className="month-selector">
          <button className="nav-btn" onClick={() => {
            const [year, m] = month.split('-').map(Number)
            const newDate = new Date(year, m - 2)
            setMonth(`${newDate.getFullYear()}-${String(newDate.getMonth() + 1).padStart(2, '0')}`)
          }}><ChevronLeft size={20} /></button>
          <input
            type="month"
            value={month}
            onChange={(e) => setMonth(e.target.value)}
            className="month-input"
          />
          <button className="nav-btn" onClick={() => {
            const [year, m] = month.split('-').map(Number)
            const newDate = new Date(year, m)
            setMonth(`${newDate.getFullYear()}-${String(newDate.getMonth() + 1).padStart(2, '0')}`)
          }}><ChevronRight size={20} /></button>
        </div>

        <div className="report-type-tabs">
          <button 
            className={`tab-btn ${reportType === 'hours' ? 'active' : ''}`}
            onClick={() => setReportType('hours')}
          >
            <Clock size={16} /> Horas
          </button>
          <button 
            className={`tab-btn ${reportType === 'tasks' ? 'active' : ''}`}
            onClick={() => setReportType('tasks')}
          >
            <CheckSquare size={16} /> Tareas
          </button>
        </div>
      </div>

      {isLoading ? (
        <div className="reports-loading">
          <div className="spinner" />
          <p>Cargando datos...</p>
        </div>
      ) : (
        <>
          {reportType === 'hours' && (
            <div className="reports-content">
              <div className="stats-grid">
                <div className="stat-card-modern">
                  <div className="stat-icon blue">
                    <Clock size={24} />
                  </div>
                  <div className="stat-info">
                    <span className="stat-label">Horas Totales</span>
                    <span className="stat-value">{hoursStats.total.toFixed(1)}h</span>
                    <span className="stat-progress">Meta: {hoursStats.targetHours}h</span>
                  </div>
                </div>

                <div className="stat-card-modern">
                  <div className="stat-icon green">
                    <CheckCircle2 size={24} />
                  </div>
                  <div className="stat-info">
                    <span className="stat-label">Aprobadas</span>
                    <span className="stat-value">{hoursStats.approved.toFixed(1)}h</span>
                    <span className="stat-progress success">{hoursStats.approved > 0 ? '✓ Verificadas' : 'Sin aprobar'}</span>
                  </div>
                </div>

                <div className="stat-card-modern">
                  <div className="stat-icon orange">
                    <AlertCircle size={24} />
                  </div>
                  <div className="stat-info">
                    <span className="stat-label">Pendientes</span>
                    <span className="stat-value">{hoursStats.pending.toFixed(1)}h</span>
                    <span className="stat-progress warning">{hoursStats.pending > 0 ? 'Por aprobar' : 'Sin pendientes'}</span>
                  </div>
                </div>

                <div className="stat-card-modern">
                  <div className="stat-icon purple">
                    <Calendar size={24} />
                  </div>
                  <div className="stat-info">
                    <span className="stat-label">Días Trabajados</span>
                    <span className="stat-value">{hoursStats.daysWorked}</span>
                    <span className="stat-progress">Meta: 20 días</span>
                  </div>
                </div>
              </div>

              <div className="charts-row">
              {workHours.some(wh => wh.activities) && (
                <div className="activities-section">
                  <h3><FileText size={20} style={{ verticalAlign: 'middle', marginRight: '8px' }} /> Actividades Registradas</h3>
                  <div className="activities-list">
                    {workHours.filter(wh => wh.activities).slice(0, 10).map((wh) => (
                      <div key={wh.id} className="activity-card">
                        <div className="activity-date">
                          <span className="activity-day">{new Date(wh.work_date).getDate()}</span>
                          <span className="activity-month">{new Date(wh.work_date).toLocaleDateString('es-ES', { month: 'short' })}</span>
                        </div>
                        <div className="activity-content">
                          <p className="activity-text">{wh.activities}</p>
                          <span className={`activity-type ${wh.work_type}`}>
                            {wh.work_type === 'complete' ? 'Jornada Completa' : 'Ausencia'}
                          </span>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}

                <div className="chart-card large">
                  <div className="chart-header">
                    <h3>Horas Diarias</h3>
                  </div>
                  <div className="chart-body">
                    <ResponsiveContainer width="100%" height={260}>
                      <BarChart data={dailyData}>
                        <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" vertical={false} />
                        <XAxis dataKey="day" stroke="#94a3b8" fontSize={12} tickLine={false} axisLine={false} />
                        <YAxis stroke="#94a3b8" fontSize={12} tickLine={false} axisLine={false} />
                        <Tooltip 
                          cursor={{ fill: '#f8fafc' }}
                          contentStyle={{ borderRadius: 12, border: 'none', boxShadow: '0 10px 15px -3px rgba(0,0,0,0.1)' }}
                          formatter={(value) => [`${value}h`, 'Horas']}
                        />
                        <Bar dataKey="hours" fill="#3b82f6" radius={[4, 4, 0, 0]} name="Horas" />
                        <Bar dataKey="target" fill="#e2e8f0" radius={[4, 4, 0, 0]} name="Meta" />
                      </BarChart>
                    </ResponsiveContainer>
                  </div>
                </div>

                <div className="breakdown-card progress-breakdown">
                  <div className="breakdown-indicator" style={{ background: '#3b82f6' }}></div>
                  <div className="breakdown-info">
                    <span className="breakdown-label">Progreso del Mes</span>
                    <span className="breakdown-value">{hoursStats.progress.toFixed(0)}%</span>
                    <div className="progress-bar-mini">
                      <div className="progress-fill" style={{ width: `${hoursStats.progress}%` }}></div>
                    </div>
                    <p className="breakdown-desc">
                      Has completado {hoursStats.total.toFixed(1)}h de la meta de {hoursStats.targetHours}h.
                    </p>
                  </div>
                </div>
              </div>

              <div className="detail-table">
                <h3>Detalle de registros</h3>
                <div className="table-container">
                  <table className="data-table">
                    <thead>
                      <tr>
                        <th>Fecha</th>
                        <th>Horas</th>
                        <th>Estado</th>
                      </tr>
                    </thead>
                    <tbody>
                      {workHours.length === 0 ? (
                        <tr>
                          <td colSpan={3} className="empty-cell">No hay registros</td>
                        </tr>
                      ) : (
                        workHours.slice(0, 10).map((wh) => (
                          <tr key={wh.id}>
                            <td>{new Date(wh.work_date).toLocaleDateString('es-ES', { weekday: 'short', day: 'numeric', month: 'short' })}</td>
                            <td>{wh.hours_worked}h</td>
                            <td>
                              <span className={`status-pill ${wh.approved ? 'approved' : 'pending'}`}>
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
            <div className="reports-content">
              <div className="stats-grid">
                <div className="stat-card-modern">
                  <div className="stat-icon blue">
                    <CheckSquare size={24} />
                  </div>
                  <div className="stat-info">
                    <span className="stat-label">Total Tareas</span>
                    <span className="stat-value">{tasks.length}</span>
                  </div>
                </div>

                <div className="stat-card-modern">
                  <div className="stat-icon green">
                    <Check size={24} />
                  </div>
                  <div className="stat-info">
                    <span className="stat-label">Completadas</span>
                    <span className="stat-value">{tasks.filter(t => t.status === 'finalizado').length}</span>
                  </div>
                </div>

                <div className="stat-card-modern">
                  <div className="stat-icon orange">
                    <Clock size={24} />
                  </div>
                  <div className="stat-info">
                    <span className="stat-label">En Proceso</span>
                    <span className="stat-value">{tasks.filter(t => t.status === 'en_proceso').length}</span>
                  </div>
                </div>

                <div className="stat-card-modern">
                  <div className="stat-icon red">
                    <AlertCircle size={24} />
                  </div>
                  <div className="stat-info">
                    <span className="stat-label">Por Hacer</span>
                    <span className="stat-value">{tasks.filter(t => t.status === 'por_hacer').length}</span>
                  </div>
                </div>
              </div>

              <div className="reports-section">
                <div className="section-header">
                  <h3>Distribución de Prioridades</h3>
                </div>
                <div className="priority-summary-bar">
                  {['Urgente', 'Alta', 'Media', 'Baja'].map((p) => {
                    const data = priorityData.find(d => d.name === p) || { name: p, value: 0 }
                    const count = data.value
                    const total = tasks.length || 1
                    const width = (count / total) * 100
                    const color = p === 'Urgente' ? '#ef4444' : p === 'Alta' ? '#f97316' : p === 'Media' ? '#f59e0b' : '#3b82f6'
                    
                    if (count === 0) return null
                    
                    return (
                      <div key={p} className="priority-bar-segment" style={{ width: `${width}%`, background: color }} title={`${p}: ${count}`}>
                        <span className="segment-label">{p} ({count})</span>
                      </div>
                    )
                  })}
                </div>
              </div>

              <div className="reports-section">
                <div className="section-header">
                  <h3>Tareas Críticas Pendientes</h3>
                  <span className="badge-count">
                    {tasks.filter(t => (t.priority === 'urgent' || t.priority === 'high') && t.status !== 'finalizado').length} Urgentes/Altas
                  </span>
                </div>
                <div className="critical-tasks-container">
                  <table className="critical-table">
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
                          <td colSpan={4} className="empty-cell">No hay tareas críticas pendientes</td>
                        </tr>
                      ) : (
                        tasks
                          .filter(t => (t.priority === 'urgent' || t.priority === 'high') && t.status !== 'finalizado')
                          .slice(0, 5)
                          .map((t) => (
                            <tr key={t.id}>
                              <td className="task-title-cell">{t.title}</td>
                              <td>
                                <span className={`priority-tag ${t.priority}`}>
                                  {t.priority === 'urgent' ? 'Urgente' : 'Alta'}
                                </span>
                              </td>
                              <td>
                                <span className={`status-pill-small ${t.status}`}>
                                  {t.status === 'en_proceso' ? 'En proceso' : 'Por hacer'}
                                </span>
                              </td>
                              <td className="task-date-cell">{new Date(t.created_at).toLocaleDateString('es-ES')}</td>
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
