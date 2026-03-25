import { useState, useEffect, useMemo } from 'react'
import { 
  BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
  PieChart, Pie, Cell
} from 'recharts'
import { userService, workHourService, taskService } from '../services/api'
import { useAuth } from '../context/AuthContext'
import { WorkHour } from '../types'
import './Reports.css'

const COLORS = ['#3b82f6', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6']

export default function Reports() {
  const { user } = useAuth()
  const [employees, setEmployees] = useState<{ id: number; name: string }[]>([])
  const [selectedEmployee, setSelectedEmployee] = useState<number | ''>('')
  const [workHours, setWorkHours] = useState<WorkHour[]>([])
  const [tasks, setTasks] = useState<{ status: string; priority: string }[]>([])
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

  const statusData = useMemo(() => {
    const counts = {
      por_hacer: 0,
      en_proceso: 0,
      finalizado: 0
    }
    tasks.forEach(t => {
      if (counts[t.status as keyof typeof counts] !== undefined) {
        counts[t.status as keyof typeof counts]++
      }
    })
    return [
      { name: 'Por hacer', value: counts.por_hacer },
      { name: 'En proceso', value: counts.en_proceso },
      { name: 'Finalizado', value: counts.finalizado }
    ].filter(d => d.value > 0)
  }, [tasks])

  const priorityData = useMemo(() => {
    const counts: Record<string, number> = { urgent: 0, high: 0, medium: 0, low: 0 }
    tasks.forEach(t => {
      if (counts[t.priority] !== undefined) {
        counts[t.priority]++
      }
    })
    return Object.entries(counts).map(([name, value]) => ({ name, value }))
  }, [tasks])

  return (
    <div className="reports-page">
      <div className="page-header">
        <div className="header-left">
          <h1>📊 Reportes</h1>
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
          }}>‹</button>
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
          }}>›</button>
        </div>

        <div className="report-type-tabs">
          <button 
            className={`tab-btn ${reportType === 'hours' ? 'active' : ''}`}
            onClick={() => setReportType('hours')}
          >
            ⏱️ Horas
          </button>
          <button 
            className={`tab-btn ${reportType === 'tasks' ? 'active' : ''}`}
            onClick={() => setReportType('tasks')}
          >
            ✓ Tareas
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
                    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                      <circle cx="12" cy="12" r="10" />
                      <polyline points="12 6 12 12 16 14" />
                    </svg>
                  </div>
                  <div className="stat-info">
                    <span className="stat-label">Horas Totales</span>
                    <span className="stat-value">{hoursStats.total.toFixed(1)}h</span>
                    <span className="stat-progress">Meta: {hoursStats.targetHours}h</span>
                  </div>
                </div>

                <div className="stat-card-modern">
                  <div className="stat-icon green">
                    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                      <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
                      <polyline points="22 4 12 14.01 9 11.01" />
                    </svg>
                  </div>
                  <div className="stat-info">
                    <span className="stat-label">Aprobadas</span>
                    <span className="stat-value">{hoursStats.approved.toFixed(1)}h</span>
                    <span className="stat-progress success">{hoursStats.approved > 0 ? '✓ Verificadas' : 'Sin aprobar'}</span>
                  </div>
                </div>

                <div className="stat-card-modern">
                  <div className="stat-icon orange">
                    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                      <circle cx="12" cy="12" r="10" />
                      <line x1="12" y1="8" x2="12" y2="12" />
                      <line x1="12" y1="16" x2="12.01" y2="16" />
                    </svg>
                  </div>
                  <div className="stat-info">
                    <span className="stat-label">Pendientes</span>
                    <span className="stat-value">{hoursStats.pending.toFixed(1)}h</span>
                    <span className="stat-progress warning">{hoursStats.pending > 0 ? 'Por aprobar' : 'Sin pendientes'}</span>
                  </div>
                </div>

                <div className="stat-card-modern">
                  <div className="stat-icon purple">
                    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                      <rect x="3" y="4" width="18" height="18" rx="2" ry="2" />
                      <line x1="16" y1="2" x2="16" y2="6" />
                      <line x1="8" y1="2" x2="8" y2="6" />
                      <line x1="3" y1="10" x2="21" y2="10" />
                    </svg>
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
                  <h3>📝 Actividades Registradas</h3>
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
                    <h3>Horas diarias del mes</h3>
                  </div>
                  <div className="chart-body">
                    <ResponsiveContainer width="100%" height={300}>
                      <BarChart data={dailyData}>
                        <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" />
                        <XAxis dataKey="day" stroke="#94a3b8" fontSize={12} />
                        <YAxis stroke="#94a3b8" fontSize={12} />
                        <Tooltip 
                          contentStyle={{ borderRadius: 8, border: '1px solid #e2e8f0' }}
                          formatter={(value) => [`${value}h`, 'Horas']}
                        />
                        <Bar dataKey="hours" fill="#3b82f6" radius={[4, 4, 0, 0]} name="Horas" />
                        <Bar dataKey="target" fill="#e2e8f0" radius={[4, 4, 0, 0]} name="Meta" />
                      </BarChart>
                    </ResponsiveContainer>
                  </div>
                </div>

                <div className="chart-card">
                  <div className="chart-header">
                    <h3>Progreso mensual</h3>
                  </div>
                  <div className="chart-body donut">
                    <ResponsiveContainer width="100%" height={250}>
                      <PieChart>
                        <Pie
                          data={[
                            { name: 'Completado', value: hoursStats.total },
                            { name: 'Restante', value: Math.max(0, hoursStats.targetHours - hoursStats.total) }
                          ]}
                          cx="50%"
                          cy="50%"
                          innerRadius={60}
                          outerRadius={90}
                          paddingAngle={2}
                          dataKey="value"
                        >
                          <Cell fill="#3b82f6" />
                          <Cell fill="#e2e8f0" />
                        </Pie>
                        <Tooltip />
                      </PieChart>
                    </ResponsiveContainer>
                    <div className="donut-center">
                      <span className="donut-value">{hoursStats.progress.toFixed(0)}%</span>
                      <span className="donut-label">completado</span>
                    </div>
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
                    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                      <path d="M9 11l3 3L22 4" />
                      <path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11" />
                    </svg>
                  </div>
                  <div className="stat-info">
                    <span className="stat-label">Total Tareas</span>
                    <span className="stat-value">{tasks.length}</span>
                  </div>
                </div>

                <div className="stat-card-modern">
                  <div className="stat-icon green">
                    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                      <polyline points="20 6 9 17 4 12" />
                    </svg>
                  </div>
                  <div className="stat-info">
                    <span className="stat-label">Completadas</span>
                    <span className="stat-value">{tasks.filter(t => t.status === 'finalizado').length}</span>
                  </div>
                </div>

                <div className="stat-card-modern">
                  <div className="stat-icon orange">
                    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                      <circle cx="12" cy="12" r="10" />
                      <polyline points="12 6 12 12 16 14" />
                    </svg>
                  </div>
                  <div className="stat-info">
                    <span className="stat-label">En Proceso</span>
                    <span className="stat-value">{tasks.filter(t => t.status === 'en_proceso').length}</span>
                  </div>
                </div>

                <div className="stat-card-modern">
                  <div className="stat-icon red">
                    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                      <circle cx="12" cy="12" r="10" />
                      <line x1="15" y1="9" x2="9" y2="15" />
                      <line x1="9" y1="9" x2="15" y2="15" />
                    </svg>
                  </div>
                  <div className="stat-info">
                    <span className="stat-label">Por Hacer</span>
                    <span className="stat-value">{tasks.filter(t => t.status === 'por_hacer').length}</span>
                  </div>
                </div>
              </div>

              <div className="charts-row">
                <div className="chart-card">
                  <div className="chart-header">
                    <h3>Estado de tareas</h3>
                  </div>
                  <div className="chart-body">
                    <ResponsiveContainer width="100%" height={250}>
                      <PieChart>
                        <Pie
                          data={statusData}
                          cx="50%"
                          cy="50%"
                          outerRadius={80}
                          dataKey="value"
                          label={({ name, percent }) => `${name} ${((percent || 0) * 100).toFixed(0)}%`}
                        >
                          {statusData.map((_, index) => (
                            <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                          ))}
                        </Pie>
                        <Tooltip />
                      </PieChart>
                    </ResponsiveContainer>
                  </div>
                </div>

                <div className="chart-card">
                  <div className="chart-header">
                    <h3>Por prioridad</h3>
                  </div>
                  <div className="chart-body">
                    <ResponsiveContainer width="100%" height={250}>
                      <PieChart>
                        <Pie
                          data={priorityData}
                          cx="50%"
                          cy="50%"
                          innerRadius={50}
                          outerRadius={80}
                          dataKey="value"
                          label={({ name, value }) => `${name}: ${value}`}
                        >
                          {priorityData.map((_, index) => (
                            <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                          ))}
                        </Pie>
                        <Tooltip />
                      </PieChart>
                    </ResponsiveContainer>
                  </div>
                </div>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  )
}
