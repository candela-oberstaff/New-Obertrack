import { useState, useEffect, useMemo } from 'react'
import { workHourService } from '../services/api'
import { useAuth } from '../context/AuthContext'
import type { WorkHour } from '../types'
import './WorkHours.css'

const DAYS_ES = ['Dom', 'Lun', 'Mar', 'Mié', 'Jue', 'Vie', 'Sáb']
const MONTHS_ES = ['Enero', 'Febrero', 'Marzo', 'Abril', 'Mayo', 'Junio', 'Julio', 'Agosto', 'Septiembre', 'Octubre', 'Noviembre', 'Diciembre']

const JORNADA_COMPLETA = 8

function CalendarView({ 
  workHours, 
  onDayClick,
  selectedDate,
  currentMonth,
  currentYear 
}: { 
  workHours: WorkHour[]
  onDayClick: (date: string) => void
  selectedDate: string | null
  currentMonth: number
  currentYear: number
}) {
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
    days.push(<div key={`empty-${i}`} className="calendar-day empty" />)
  }
  for (let day = 1; day <= daysInMonth; day++) {
    const dateStr = `${currentYear}-${String(currentMonth + 1).padStart(2, '0')}-${String(day).padStart(2, '0')}`
    const info = dayInfo[dateStr]
    const isSelected = selectedDate === dateStr
    const dayOfWeek = new Date(dateStr).getDay()
    const isWeekend = dayOfWeek === 0 || dayOfWeek === 6
    
    days.push(
      <div 
        key={day} 
        className={`calendar-day ${info?.type || ''} ${isSelected ? 'selected' : ''} ${isWeekend ? 'weekend' : ''}`}
        onClick={() => onDayClick(dateStr)}
      >
        <span className="day-number">{day}</span>
        {info?.type === 'complete' && <span className="day-badge complete">✓</span>}
        {info?.type === 'absence' && <span className="day-badge absence">⚠</span>}
      </div>
    )
  }

  return (
    <div className="calendar-grid">
      {DAYS_ES.map(day => (
        <div key={day} className="calendar-header-day">{day}</div>
      ))}
      {days}
    </div>
  )
}

export default function WorkHours() {
  const { user } = useAuth()
  const [workHours, setWorkHours] = useState<WorkHour[]>([])
  const [summary, setSummary] = useState({ total_hours: 0, approved_hours: 0, pending_hours: 0 })
  const [isLoading, setIsLoading] = useState(true)
  const [showModal, setShowModal] = useState(false)
  const [selectedIds, setSelectedIds] = useState<number[]>([])
  const [selectedDate, setSelectedDate] = useState<string | null>(null)
  const [currentMonth, setCurrentMonth] = useState(new Date().getMonth())
  const [currentYear, setCurrentYear] = useState(new Date().getFullYear())

  const [formData, setFormData] = useState({
    work_date: new Date().toISOString().split('T')[0],
    work_type: 'complete' as 'complete' | 'absence',
    activities: '',
  })

  useEffect(() => {
    fetchData()
  }, [])

  const fetchData = async () => {
    try {
      const [hoursRes, summaryRes] = await Promise.all([
        workHourService.getAll({}),
        workHourService.getSummary(),
      ])
      setWorkHours(hoursRes.data)
      setSummary(summaryRes)
    } catch (error) {
      console.error('Error fetching data:', error)
    } finally {
      setIsLoading(false)
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await workHourService.create({
        work_date: formData.work_date,
        work_type: formData.work_type,
        activities: formData.activities || undefined,
      })
      setShowModal(false)
      resetForm()
      fetchData()
    } catch (error) {
      console.error('Error creating work hour:', error)
    }
  }

  const handleApprove = async () => {
    if (selectedIds.length === 0) return
    try {
      await workHourService.approve(selectedIds)
      setSelectedIds([])
      fetchData()
    } catch (error) {
      console.error('Error approving work hours:', error)
    }
  }

  const resetForm = () => {
    setFormData({
      work_date: new Date().toISOString().split('T')[0],
      work_type: 'complete',
      activities: '',
    })
  }

  const toggleSelect = (id: number) => {
    setSelectedIds(prev =>
      prev.includes(id) ? prev.filter(i => i !== id) : [...prev, id]
    )
  }

  const canApprove = user?.is_superadmin || user?.is_manager || user?.user_type === 'empleador'

  const filteredHours = selectedDate 
    ? workHours.filter(wh => wh.work_date.split('T')[0] === selectedDate)
    : workHours

  const prevMonth = () => {
    if (currentMonth === 0) {
      setCurrentMonth(11)
      setCurrentYear(currentYear - 1)
    } else {
      setCurrentMonth(currentMonth - 1)
    }
  }

  const nextMonth = () => {
    if (currentMonth === 11) {
      setCurrentMonth(0)
      setCurrentYear(currentYear + 1)
    } else {
      setCurrentMonth(currentMonth + 1)
    }
  }

  const todayWork = workHours.find(wh => {
    const today = new Date().toISOString().split('T')[0]
    return wh.work_date.split('T')[0] === today
  })

  const weekHours = useMemo(() => {
    const now = new Date()
    const startOfWeek = new Date(now)
    startOfWeek.setDate(now.getDate() - now.getDay())
    startOfWeek.setHours(0, 0, 0, 0)
    
    return workHours
      .filter(wh => new Date(wh.work_date) >= startOfWeek)
      .reduce((sum, wh) => sum + wh.hours_worked, 0)
  }, [workHours])

  if (isLoading) {
    return (
      <div className="work-hours-loading">
        <div className="spinner" />
        <p>Cargando...</p>
      </div>
    )
  }

  return (
    <div className="work-hours-page">
      <div className="page-header">
        <div className="header-left">
          <h1>⏱️ Mi Jornada</h1>
          <p className="header-subtitle">Registra tu día laboral</p>
        </div>
        <button className="btn-primary" onClick={() => setShowModal(true)}>
          + Registrar Día
        </button>
      </div>

      <div className="stats-row">
        <div className="stat-card-mini">
          <span className="stat-icon">📅</span>
          <div className="stat-info">
            <span className="stat-label">Hoy</span>
            <span className="stat-value">
              {todayWork ? (todayWork.work_type === 'complete' ? '✓ Completa' : '⚠ Ausencia') : '-'}
            </span>
          </div>
        </div>
        <div className="stat-card-mini">
          <span className="stat-icon">📆</span>
          <div className="stat-info">
            <span className="stat-label">Esta semana</span>
            <span className="stat-value">{weekHours.toFixed(1)}h</span>
          </div>
        </div>
        <div className="stat-card-mini approved">
          <span className="stat-icon">✓</span>
          <div className="stat-info">
            <span className="stat-label">Aprobadas</span>
            <span className="stat-value">{summary.approved_hours.toFixed(1)}h</span>
          </div>
        </div>
        <div className="stat-card-mini pending">
          <span className="stat-icon">⏳</span>
          <div className="stat-info">
            <span className="stat-label">Pendientes</span>
            <span className="stat-value">{summary.pending_hours.toFixed(1)}h</span>
          </div>
        </div>
      </div>

      <div className="work-hours-content">
        <div className="calendar-section">
          <div className="calendar-header">
            <button className="nav-btn" onClick={prevMonth}>‹</button>
            <h3>{MONTHS_ES[currentMonth]} {currentYear}</h3>
            <button className="nav-btn" onClick={nextMonth}>›</button>
          </div>
          <CalendarView 
            workHours={workHours}
            onDayClick={setSelectedDate}
            selectedDate={selectedDate}
            currentMonth={currentMonth}
            currentYear={currentYear}
          />
          <div className="calendar-legend">
            <span className="legend-item"><span className="legend-dot complete"></span>Completa</span>
            <span className="legend-item"><span className="legend-dot absence"></span>Ausencia</span>
          </div>
          {selectedDate && (
            <button className="clear-filter" onClick={() => setSelectedDate(null)}>
              Ver todos los días
            </button>
          )}
        </div>

        <div className="hours-list-section">
          <div className="list-header">
            <h3>
              {selectedDate 
                ? `Registros del ${new Date(selectedDate).toLocaleDateString('es-ES', { day: 'numeric', month: 'long' })}`
                : 'Mis registros'}
            </h3>
            {canApprove && selectedIds.length > 0 && (
              <button className="btn-approve" onClick={handleApprove}>
                Aprobar ({selectedIds.length})
              </button>
            )}
          </div>

          {filteredHours.length === 0 ? (
            <div className="empty-state">
              <span className="empty-icon">📋</span>
              <p>No hay registros</p>
            </div>
          ) : (
            <div className="hours-list">
              {filteredHours.map((wh) => (
                <div key={wh.id} className={`hour-card ${wh.work_type === 'complete' || wh.hours_worked >= JORNADA_COMPLETA ? 'complete' : 'absence'}`}>
                  <div className="hour-date">
                    <span className="day">{new Date(wh.work_date).getDate()}</span>
                    <span className="month">{MONTHS_ES[new Date(wh.work_date).getMonth()].slice(0, 3)}</span>
                  </div>
                  <div className="hour-info">
                    <span className="hours-value">
                      {wh.work_type === 'complete' || wh.hours_worked >= JORNADA_COMPLETA 
                        ? 'Jornada Completa' 
                        : 'Ausencia'}
                    </span>
                    {wh.activities && <p className="hours-comments">{wh.activities}</p>}
                  </div>
                  <div className="hour-status">
                    {canApprove && wh.hours_worked > 0 && !wh.approved && (
                      <input
                        type="checkbox"
                        checked={selectedIds.includes(wh.id)}
                        onChange={() => toggleSelect(wh.id)}
                      />
                    )}
                    <span className={`status-pill ${wh.approved ? 'approved' : 'pending'}`}>
                      {wh.approved ? 'Aprobado' : 'Pendiente'}
                    </span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {showModal && (
        <div className="modal-overlay" onClick={() => setShowModal(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h2>Registrar Día</h2>
              <button className="close-btn" onClick={() => setShowModal(false)}>✕</button>
            </div>
            <form onSubmit={handleSubmit}>
              <div className="form-group">
                <label>Fecha</label>
                <input
                  type="date"
                  value={formData.work_date}
                  onChange={(e) => setFormData({ ...formData, work_date: e.target.value })}
                  required
                />
              </div>

              <div className="form-group">
                <label>¿Cómo fue tu día?</label>
                <div className="work-type-selector">
                  <button
                    type="button"
                    className={`work-type-btn ${formData.work_type === 'complete' ? 'active' : ''}`}
                    onClick={() => setFormData({ ...formData, work_type: 'complete' })}
                  >
                    <span className="work-type-icon">✓</span>
                    <span className="work-type-label">Jornada Completa</span>
                    <span className="work-type-hours">8h</span>
                  </button>
                  <button
                    type="button"
                    className={`work-type-btn ${formData.work_type === 'absence' ? 'active' : ''}`}
                    onClick={() => setFormData({ ...formData, work_type: 'absence' })}
                  >
                    <span className="work-type-icon">⚠</span>
                    <span className="work-type-label">Ausencia</span>
                    <span className="work-type-hours">0h</span>
                  </button>
                </div>
              </div>

              <div className="form-group">
                <label>¿Qué actividades realizaste hoy?</label>
                <textarea
                  value={formData.activities}
                  onChange={(e) => setFormData({ ...formData, activities: e.target.value })}
                  placeholder="Describe las actividades realizadas durante tu jornada..."
                  rows={4}
                  required={formData.work_type === 'complete'}
                />
                <p className="form-hint">
                  Este registro aparecerá en el reporte de tu empresa.
                </p>
              </div>

              <div className="modal-actions">
                <button type="button" className="btn-cancel" onClick={() => setShowModal(false)}>
                  Cancelar
                </button>
                <button type="submit" className="btn-primary">
                  Registrar
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
