import { useState, useEffect, useMemo } from 'react'
import { workHourService } from '../services/api'
import { useAuth } from '../context/AuthContext'
import { useNavigate } from 'react-router-dom'
import type { WorkHour } from '../types'
import './WorkHours.css'

const DAYS_ES = ['Dom', 'Lun', 'Mar', 'Mié', 'Jue', 'Vie', 'Sáb']
const MONTHS_ES = ['Enero', 'Febrero', 'Marzo', 'Abril', 'Mayo', 'Junio', 'Julio', 'Agosto', 'Septiembre', 'Octubre', 'Noviembre', 'Diciembre']

const JORNADA_COMPLETA = 8

// Format decimal hours to H:MM (e.g. 1.5 -> "1:30", 0 -> "0:00")
const formatHoursMinutes = (hours: number): string => {
  const h = Math.floor(hours)
  const m = Math.round((hours - h) * 60)
  return `${h}:${String(m).padStart(2, '0')}`
}

// Simple rich text editor for activities
function RichTextEditor({ value, onChange, placeholder }: { value: string; onChange: (value: string) => void; placeholder?: string }) {
  const formatText = (tag: string) => {
    const selection = window.getSelection()?.toString() || ''
    if (selection) {
      onChange(value + `<${tag}>${selection}</${tag}>`)
    }
  }

  return (
    <div className="rich-editor">
      <div className="rich-editor-toolbar">
        <button type="button" onClick={() => formatText('strong')} title="Negrita"><strong>B</strong></button>
        <button type="button" onClick={() => formatText('em')} title="Cursiva"><em>I</em></button>
        <button type="button" onClick={() => formatText('u')} title="Subrayado"><u>U</u></button>
        <span className="toolbar-sep">|</span>
        <button type="button" onClick={() => onChange(value + '\n• ')} title="Viñeta">•</button>
        <button type="button" onClick={() => onChange(value + '\n1. ')} title="Número">1.</button>
      </div>
      <textarea
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        rows={5}
      />
    </div>
  )
}

// Función para parsear fechas sin problema de timezone
const parseLocalDate = (dateStr: string): Date => {
  const [year, month, day] = dateStr.split('T')[0].split('-').map(Number)
  return new Date(year, month - 1, day)
}

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
    const dayOfWeek = parseLocalDate(dateStr).getDay()
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
  const navigate = useNavigate()
  const [workHours, setWorkHours] = useState<WorkHour[]>([])
  const [, setPendingHours] = useState<WorkHour[]>([])
  const [summary, setSummary] = useState({ total_hours: 0, approved_hours: 0, pending_hours: 0 })
  const [isLoading, setIsLoading] = useState(true)
  const [showModal, setShowModal] = useState(false)
  const [selectedDate, setSelectedDate] = useState<string | null>(null)
  const [currentMonth, setCurrentMonth] = useState(new Date().getMonth())
  const [currentYear, setCurrentYear] = useState(new Date().getFullYear())
  const [selectedWorkHour, setSelectedWorkHour] = useState<WorkHour | null>(null)

  // Check if profile is complete (employers/superadmins are exempt)
  const isProfileComplete = useMemo(() => {
    if (!user) return false
    if (user.is_superadmin || user.user_type === 'empleador') return true
    return !!(user.phone_number && user.country && user.city && user.job_title && user.location)
  }, [user])

  const [formData, setFormData] = useState({
    work_date: new Date().toISOString().split('T')[0],
    work_type: 'complete' as 'complete' | 'absence',
    activities: '',
    absence_reason: '',
    absence_hours: 0,
  })

  useEffect(() => {
    fetchData()
  }, [])

  const fetchData = async () => {
    try {
      const isEmployer = user?.user_type === 'empleador'
      const isSuperadmin = user?.is_superadmin
      
      const [hoursRes, summaryRes] = await Promise.all([
        workHourService.getAll({}),
        workHourService.getSummary(),
      ])
      setWorkHours(hoursRes.data)
      setSummary(summaryRes)
      
      if (isEmployer || isSuperadmin) {
        const pendingRes = await workHourService.getPending()
        setPendingHours(pendingRes)
      }
    } catch (error) {
      console.error('Error fetching data:', error)
    } finally {
      setIsLoading(false)
    }
  }

  const today = new Date().toISOString().split('T')[0]

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      const hoursWorked = formData.work_type === 'absence' 
        ? Math.max(0, 8 - (formData.absence_hours || 0))
        : 8
      
      await workHourService.create({
        work_date: formData.work_date,
        work_type: formData.work_type,
        activities: formData.activities || undefined,
        hours_worked: hoursWorked,
        absence_reason: formData.work_type === 'absence' ? formData.absence_reason : undefined,
        absence_hours: formData.work_type === 'absence' ? formData.absence_hours : undefined,
      })
      setShowModal(false)
      resetForm()
      fetchData()
    } catch (error) {
      console.error('Error creating work hour:', error)
    }
  }

  const resetForm = () => {
    setFormData({
      work_date: new Date().toISOString().split('T')[0],
      work_type: 'complete',
      activities: '',
      absence_reason: '',
      absence_hours: 0,
    })
  }

  const canApprove = user?.is_superadmin || user?.is_manager || user?.user_type === 'empleador'

  const filteredHours = selectedDate 
    ? workHours.filter(wh => wh.work_date.split('T')[0] === selectedDate)
    : workHours

  const pendingForSelectedDate = selectedDate
    ? filteredHours.filter(wh => !wh.approved && wh.hours_worked > 0)
    : workHours.filter(wh => !wh.approved && wh.hours_worked > 0)

  const handleBulkApprove = async () => {
    if (pendingForSelectedDate.length === 0) return
    try {
      await workHourService.approve(pendingForSelectedDate.map(wh => wh.id))
      fetchData()
    } catch (error) {
      console.error('Error approving work hours:', error)
    }
  }

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
    const startStr = startOfWeek.toISOString().split('T')[0]
    
    return workHours
      .filter(wh => wh.work_date.split('T')[0] >= startStr)
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

      {/* 
      <div className="page-header">
        <div className="header-left"> 
          <h1>⏱️ Mi Jornada</h1>
          <p className="header-subtitle">Registra tu día laboral</p>
        </div>
        {isProfileComplete && (
          <button className="btn-primary" onClick={() => setShowModal(true)}>
            + Registrar Día
          </button>
        )}
      </div>
      */}

      {!isProfileComplete && ( 
        <div className="profile-incomplete-banner">
          <div className="banner-icon">⚠️</div>
          <div className="banner-content">
            <strong>Completa tu perfil para registrar horas</strong>
            <p>Debes completar los campos de Teléfono, País, Ciudad, Puesto y Dirección en tu perfil antes de poder registrar tu jornada laboral.</p>
          </div>
          <button className="btn-primary" onClick={() => navigate('/profile')}>
            Completar Perfil
          </button>
        </div>
      )}

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
                ? `Registros del ${parseLocalDate(selectedDate).toLocaleDateString('es-ES', { day: 'numeric', month: 'long' })}`
                : 'Mis registros'}
            </h3>
            {canApprove && pendingForSelectedDate.length > 0 && (
              <button className="btn-bulk-approve" onClick={handleBulkApprove}>
                ✓ Aprobar todos ({pendingForSelectedDate.length})
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
                <div key={wh.id} className={`hour-card ${wh.work_type === 'complete' || wh.hours_worked >= JORNADA_COMPLETA ? 'complete' : 'absence'} clickable`} onClick={() => setSelectedWorkHour(wh)}>
                  <div className="hour-date">
                    <span className="day">{parseLocalDate(wh.work_date).getDate()}</span>
                    <span className="month">{MONTHS_ES[parseLocalDate(wh.work_date).getMonth()].slice(0, 3)}</span>
                  </div>
                  <div className="hour-info">
                    {canApprove && wh.user && <span className="hours-user">{wh.user.name}</span>}
                    <span className="hours-value">
                      {wh.work_type === 'complete' || wh.hours_worked >= JORNADA_COMPLETA 
                        ? 'Jornada Completa' 
                        : `Ausencia (${wh.hours_worked}h)`}
                    </span>
                    {wh.activities && <p className="hours-comments">{wh.activities}</p>}
                  </div>
                  <div className="hour-status">
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
                  max={today}
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
                    <span className="work-type-hours">{formatHoursMinutes(Math.max(0, 8 - (formData.absence_hours || 0)))}</span>
                  </button>
                </div>
              </div>

              {formData.work_type === 'absence' && (
                <>
                  <div className="form-group">
                    <label>Motivo de ausencia</label>
                    <select
                      value={formData.absence_reason}
                      onChange={(e) => setFormData({ ...formData, absence_reason: e.target.value })}
                      required
                    >
                      <option value="">Selecciona un motivo</option>
                      <option value="enfermedad">Enfermedad</option>
                      <option value="cita_medica">Cita Médica</option>
                      <option value="emergencia_familiar">Emergencia Familiar</option>
                      <option value="vacaciones">Vacaciones</option>
                      <option value="permiso_personal">Permiso Personal</option>
                      <option value="otro">Otro</option>
                    </select>
                  </div>

                  <div className="form-group">
                    <label>Horas Trabajadas</label>
                    <div className="hours-counter">
                      <button
                        type="button"
                        className="counter-btn"
                        onClick={() => setFormData(prev => ({ ...prev, absence_hours: Math.max(0, prev.absence_hours - 0.5) }))}
                        disabled={formData.absence_hours <= 0}
                      >
                        −
                      </button>
                      <span className="counter-value">
                        {formatHoursMinutes(formData.absence_hours)}
                      </span>
                      <button
                        type="button"
                        className="counter-btn"
                        onClick={() => setFormData(prev => ({ ...prev, absence_hours: Math.min(7.5, prev.absence_hours + 0.5) }))}
                        disabled={formData.absence_hours >= 7.5}
                      >
                        +
                      </button>
                    </div>
                 {/* 
                 
                   <p className="form-hint">
                      Horas a registrar: {Math.max(0, 8 - (formData.absence_hours || 0))}h
                    </p>
                 
                 */}
                  

                  </div>
                </>
              )}

              <div className="form-group">
                <label>¿Qué actividades realizaste hoy?</label>
                <RichTextEditor
                  value={formData.activities}
                  onChange={(value) => setFormData({ ...formData, activities: value })}
                  placeholder="Describe las actividades realizadas durante tu jornada...&#10;• Tarea 1&#10;• Tarea 2"
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

      {selectedWorkHour && (
        <div className="modal-overlay" onClick={() => setSelectedWorkHour(null)}>
          <div className="modal detail-modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h2>Detalle de Registro</h2>
              <button className="close-btn" onClick={() => setSelectedWorkHour(null)}>✕</button>
            </div>
            <div className="detail-content">
              <div className="detail-row">
                <span className="detail-label">Profesional</span>
                <span className="detail-value">{selectedWorkHour.user?.name || 'Usuario'}</span>
              </div>
              <div className="detail-row">
                <span className="detail-label">Fecha</span>
                <span className="detail-value">
                  {parseLocalDate(selectedWorkHour.work_date).toLocaleDateString('es-ES', { 
                    weekday: 'long', year: 'numeric', month: 'long', day: 'numeric' 
                  })}
                </span>
              </div>
              <div className="detail-row">
                <span className="detail-label">Tipo</span>
                <span className={`detail-type ${selectedWorkHour.work_type}`}>
                  {selectedWorkHour.work_type === 'complete' || selectedWorkHour.hours_worked >= 8 
                    ? 'Jornada Completa' : `Ausencia (${selectedWorkHour.hours_worked}h)`}
                </span>
              </div>
              <div className="detail-row">
                <span className="detail-label">Horas</span>
                <span className="detail-value">{selectedWorkHour.hours_worked}h</span>
              </div>
              {selectedWorkHour.absence_reason && (
                <div className="detail-row">
                  <span className="detail-label">Motivo</span>
                  <span className="detail-value">{selectedWorkHour.absence_reason}</span>
                </div>
              )}
              {selectedWorkHour.activities && (
                <div className="detail-activities">
                  <span className="detail-label">Actividades del día</span>
                  <div 
                    className="detail-text" 
                    dangerouslySetInnerHTML={{ 
                      __html: selectedWorkHour.activities.replace(/\n/g, '<br>').replace(/• /g, '<br>• ').replace(/<br>1\./g, '<br>1.') 
                    }}
                  />
                </div>
              )}
              <div className="detail-row">
                <span className="detail-label">Estado</span>
                <span className={`status-pill ${selectedWorkHour.approved ? 'approved' : 'pending'}`}>
                  {selectedWorkHour.approved ? 'Aprobado' : 'Pendiente'}
                </span>
              </div>
            </div>
            <div className="detail-actions">
              <button className="btn-cancel" onClick={() => setSelectedWorkHour(null)}>Cerrar</button>
              {canApprove && !selectedWorkHour.approved && (
                <button className="btn-primary" onClick={async () => {
                  await workHourService.approve([selectedWorkHour.id])
                  setSelectedWorkHour(null)
                  fetchData()
                }}>✓ Aprobar</button>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
