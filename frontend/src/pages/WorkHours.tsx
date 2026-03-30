import { useState, useEffect, useMemo } from 'react'
import { workHourService } from '../services/api'
import { useAuth } from '../context/AuthContext'
import type { WorkHour } from '../types'
import {
  Calendar,
  CheckCircle2,
  AlertTriangle,
  Clock,
  Hourglass,
  ChevronLeft,
  ChevronRight,
  ClipboardList,
  X,
  Check,
  AlertCircle
} from 'lucide-react'
import { RichTextEditor } from '../components/Tasks/RichTextEditor'
import styles from './WorkHours.module.css'

const DAYS_ES = ['Dom', 'Lun', 'Mar', 'Mié', 'Jue', 'Vie', 'Sáb']
const MONTHS_ES = ['Enero', 'Febrero', 'Marzo', 'Abril', 'Mayo', 'Junio', 'Julio', 'Agosto', 'Septiembre', 'Octubre', 'Noviembre', 'Diciembre']

const JORNADA_COMPLETA = 8

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

export default function WorkHours() {
  const { user } = useAuth()
  const [workHours, setWorkHours] = useState<WorkHour[]>([])
  const [, setPendingHours] = useState<WorkHour[]>([])
  const [summary, setSummary] = useState({ total_hours: 0, approved_hours: 0, pending_hours: 0 })
  const [isLoading, setIsLoading] = useState(true)
  const [showModal, setShowModal] = useState(false)
  const [selectedDate, setSelectedDate] = useState<string | null>(null)
  const [currentMonth, setCurrentMonth] = useState(new Date().getMonth())
  const [currentYear, setCurrentYear] = useState(new Date().getFullYear())
  const [selectedWorkHour, setSelectedWorkHour] = useState<WorkHour | null>(null)

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
      <div className={styles['work-hours-loading']}>
        <div className={styles['spinner']} />
        <p>Cargando...</p>
      </div>
    )
  }

  return (
    <div className={styles['work-hours-page']}>
      <div className={styles['wh-page-header']}>
        <div className={styles['header-left']}>
          <h1><Clock size={28} style={{ verticalAlign: 'middle', marginRight: '8px' }} /> Mi Jornada</h1>
          <p className={styles['header-subtitle']}>Registra tu día laboral</p>
        </div>
        <button className={styles['btn-primary']} onClick={() => setShowModal(true)}>
          + Registrar Día
        </button>
      </div>

      <div className={styles['wh-stats-row']}>
        <div className={styles['stat-card-mini']}>
          <span className={styles['stat-icon']}><Calendar size={20} /></span>
          <div className={styles['stat-info']}>
            <span className={styles['stat-label']}>Hoy</span>
            <span className={styles['stat-value']}>
              {todayWork ? (todayWork.work_type === 'complete' ? <><CheckCircle2 size={14} /> Completa</> : <><AlertTriangle size={14} /> Ausencia</>) : '-'}
            </span>
          </div>
        </div>
        <div className={styles['stat-card-mini']}>
          <span className={styles['stat-icon']}><Calendar size={20} /></span>
          <div className={styles['stat-info']}>
            <span className={styles['stat-label']}>Esta semana</span>
            <span className={styles['stat-value']}>{weekHours.toFixed(1)}h</span>
          </div>
        </div>
        <div className={`${styles['stat-card-mini']} ${styles['approved']}`}>
          <span className={styles['stat-icon']}><CheckCircle2 size={20} /></span>
          <div className={styles['stat-info']}>
            <span className={styles['stat-label']}>Aprobadas</span>
            <span className={styles['stat-value']}>{summary.approved_hours.toFixed(1)}h</span>
          </div>
        </div>
        <div className={`${styles['stat-card-mini']} ${styles['pending']}`}>
          <span className={styles['stat-icon']}><Hourglass size={20} /></span>
          <div className={styles['stat-info']}>
            <span className={styles['stat-label']}>Pendientes</span>
            <span className={styles['stat-value']}>{summary.pending_hours.toFixed(1)}h</span>
          </div>
        </div>
      </div>

      <div className={styles['work-hours-content']}>
        <div className={styles['calendar-section']}>
          <div className={styles['calendar-header']}>
            <button className={styles['nav-btn']} onClick={prevMonth}><ChevronLeft size={20} /></button>
            <h3>{MONTHS_ES[currentMonth]} {currentYear}</h3>
            <button className={styles['nav-btn']} onClick={nextMonth}><ChevronRight size={20} /></button>
          </div>
          <CalendarView
            workHours={workHours}
            onDayClick={setSelectedDate}
            selectedDate={selectedDate}
            currentMonth={currentMonth}
            currentYear={currentYear}
          />
          <div className={styles['calendar-legend']}>
            <span className={styles['legend-item']}><span className={`${styles['legend-dot']} ${styles['complete']}`}></span>Completa</span>
            <span className={styles['legend-item']}><span className={`${styles['legend-dot']} ${styles['absence']}`}></span>Ausencia</span>
          </div>
          {selectedDate && (
            <button className={styles['clear-filter']} onClick={() => setSelectedDate(null)}>
              Ver todos los días
            </button>
          )}
        </div>

        <div className={styles['hours-list-section']}>
          <div className={styles['list-header']}>
            <h3>
              {selectedDate
                ? `Registros del ${parseLocalDate(selectedDate).toLocaleDateString('es-ES', { day: 'numeric', month: 'long' })}`
                : 'Mis registros'}
            </h3>
            {canApprove && pendingForSelectedDate.length > 0 && (
              <button className={styles['btn-bulk-approve']} onClick={handleBulkApprove}>
                <Check size={16} /> Aprobar todos ({pendingForSelectedDate.length})
              </button>
            )}
          </div>

          {filteredHours.length === 0 ? (
            <div className={styles['empty-state']}>
              <span className={styles['empty-icon']}><ClipboardList size={40} /></span>
              <p>No hay registros</p>
            </div>
          ) : (
            <div className={styles['hours-list']}>
              {filteredHours.map((wh) => (
                <div key={wh.id} className={`${styles['hour-card']} ${wh.work_type === 'complete' || wh.hours_worked >= JORNADA_COMPLETA ? styles['complete'] : styles['absence']} ${styles['clickable']}`} onClick={() => setSelectedWorkHour(wh)}>
                  <div className={styles['hour-date']}>
                    <span className={styles['day']}>{parseLocalDate(wh.work_date).getDate()}</span>
                    <span className={styles['month']}>{MONTHS_ES[parseLocalDate(wh.work_date).getMonth()].slice(0, 3)}</span>
                  </div>
                  <div className={styles['hour-info']}>
                    {canApprove && wh.user && <span className={styles['hours-user']}>{wh.user.name}</span>}
                    <span className={styles['hours-value']}>
                      {wh.work_type === 'complete' || wh.hours_worked >= JORNADA_COMPLETA
                        ? 'Jornada Completa'
                        : `Ausencia (${wh.hours_worked}h)`}
                    </span>
                    {wh.activities && <p className={styles['hours-comments']}>{wh.activities}</p>}
                  </div>
                  <div className={styles['hour-status']}>
                    <span className={`${styles['status-pill']} ${wh.approved ? styles['approved'] : styles['pending']}`}>
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
        <div className={styles['modal-overlay']} onClick={() => setShowModal(false)}>
          <div className={styles['modal']} onClick={(e) => e.stopPropagation()}>
            <div className={styles['modal-header']}>
              <h2>Registrar Día</h2>
              <button className={styles['close-btn']} onClick={() => setShowModal(false)}><X size={20} /></button>
            </div>
            <form onSubmit={handleSubmit}>
              <div className={styles['form-group']}>
                <label>Fecha</label>
                <input
                  type="date"
                  value={formData.work_date}
                  max={today}
                  onChange={(e) => setFormData({ ...formData, work_date: e.target.value })}
                  required
                />
              </div>

              <div className={styles['form-group']}>
                <label>¿Cómo fue tu día?</label>
                <div className={styles['work-type-selector']}>
                  <button
                    type="button"
                    className={`${styles['work-type-btn']} ${formData.work_type === 'complete' ? styles['active'] : ''}`}
                    onClick={() => setFormData({ ...formData, work_type: 'complete' })}
                  >
                    <span className={styles['work-type-icon']}><Check size={20} /></span>
                    <span className={styles['work-type-label']}>Jornada Completa</span>
                    <span className={styles['work-type-hours']}>8h</span>
                  </button>
                  <button
                    type="button"
                    className={`${styles['work-type-btn']} ${formData.work_type === 'absence' ? styles['active'] : ''}`}
                    onClick={() => setFormData({ ...formData, work_type: 'absence' })}
                  >
                    <span className={styles['work-type-icon']}><AlertCircle size={20} /></span>
                    <span className={styles['work-type-label']}>Ausencia</span>
                    <span className={styles['work-type-hours']}>{Math.max(0, 8 - (formData.absence_hours || 0))}h</span>
                  </button>
                </div>
              </div>

              {formData.work_type === 'absence' && (
                <>
                  <div className={styles['form-group']}>
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

                  <div className={styles['form-group']}>
                    <label>Horas de ausencia</label>
                    <select
                      value={formData.absence_hours}
                      onChange={(e) => setFormData({ ...formData, absence_hours: Number(e.target.value) })}
                      required
                    >
                      <option value={0}>0 horas</option>
                      <option value={1}>1 hora</option>
                      <option value={2}>2 horas</option>
                      <option value={3}>3 horas</option>
                      <option value={4}>4 horas</option>
                      <option value={5}>5 horas</option>
                      <option value={6}>6 horas</option>
                      <option value={7}>7 horas</option>
                      <option value={8}>8 horas (día completo)</option>
                    </select>
                    <p className={styles['form-hint']}>
                      Horas a registrar: {Math.max(0, 8 - (formData.absence_hours || 0))}h
                    </p>
                  </div>
                </>
              )}

              <div className={styles['form-group']}>
                <label>¿Qué actividades realizaste hoy?</label>
                <RichTextEditor
                  value={formData.activities}
                  onChange={(value) => setFormData({ ...formData, activities: value })}
                  placeholder="Describe las actividades realizadas durante tu jornada...&#10;• Tarea 1&#10;• Tarea 2"
                />
                <p className={styles['form-hint']}>
                  Este registro aparecerá en el reporte de tu empresa.
                </p>
              </div>

              <div className={styles['modal-actions']}>
                <button type="button" className={styles['btn-cancel']} onClick={() => setShowModal(false)}>
                  Cancelar
                </button>
                <button type="submit" className={styles['btn-primary']}>
                  Registrar
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {selectedWorkHour && (
        <div className={styles['modal-overlay']} onClick={() => setSelectedWorkHour(null)}>
          <div className={`${styles['modal']} ${styles['detail-modal']}`} onClick={(e) => e.stopPropagation()}>
            <div className={styles['modal-header']}>
              <h2>Detalle de Registro</h2>
              <button className={styles['close-btn']} onClick={() => setSelectedWorkHour(null)}><X size={20} /></button>
            </div>
            <div className={styles['detail-content']}>
              <div className={styles['detail-row']}>
                <span className={styles['detail-label']}>Profesional</span>
                <span className={styles['detail-value']}>{selectedWorkHour.user?.name || 'Usuario'}</span>
              </div>
              <div className={styles['detail-row']}>
                <span className={styles['detail-label']}>Fecha</span>
                <span className={styles['detail-value']}>
                  {parseLocalDate(selectedWorkHour.work_date).toLocaleDateString('es-ES', {
                    weekday: 'long', year: 'numeric', month: 'long', day: 'numeric'
                  })}
                </span>
              </div>
              <div className={styles['detail-row']}>
                <span className={styles['detail-label']}>Tipo</span>
                <span className={`${styles['detail-type']} ${styles[selectedWorkHour.work_type]}`}>
                  {selectedWorkHour.work_type === 'complete' || selectedWorkHour.hours_worked >= 8
                    ? 'Jornada Completa' : `Ausencia (${selectedWorkHour.hours_worked}h)`}
                </span>
              </div>
              <div className={styles['detail-row']}>
                <span className={styles['detail-label']}>Horas</span>
                <span className={styles['detail-value']}>{selectedWorkHour.hours_worked}h</span>
              </div>
              {selectedWorkHour.absence_reason && (
                <div className={styles['detail-row']}>
                  <span className={styles['detail-label']}>Motivo</span>
                  <span className={styles['detail-value']}>{selectedWorkHour.absence_reason}</span>
                </div>
              )}
              {selectedWorkHour.activities && (
                <div className={styles['detail-activities']}>
                  <span className={styles['detail-label']}>Actividades del día</span>
                  <div
                    className={styles['detail-text']}
                    dangerouslySetInnerHTML={{
                      __html: selectedWorkHour.activities.replace(/\n/g, '<br>').replace(/• /g, '<br>• ').replace(/<br>1\./g, '<br>1.')
                    }}
                  />
                </div>
              )}
              <div className={styles['detail-row']}>
                <span className={styles['detail-label']}>Estado</span>
                <span className={`${styles['status-pill']} ${selectedWorkHour.approved ? styles['approved'] : styles['pending']}`}>
                  {selectedWorkHour.approved ? 'Aprobado' : 'Pendiente'}
                </span>
              </div>
            </div>
            <div className={styles['detail-actions']}>
              <button className={styles['btn-cancel']} onClick={() => setSelectedWorkHour(null)}>Cerrar</button>
              {canApprove && !selectedWorkHour.approved && (
                <button className={styles['btn-primary']} onClick={async () => {
                  await workHourService.approve([selectedWorkHour.id])
                  setSelectedWorkHour(null)
                  fetchData()
                }}><Check size={16} /> Aprobar</button>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
