import { useState, useEffect } from 'react'
import { Modal, Button } from '../../components/ui'
import { Select } from '../../components/ui/Select'
import type { EmployeeSummary } from '../../types'
import { adminService } from '../../services/api'
import { useNotification } from '../../context/NotificationContext'
import styles from './Tenants.module.css'

interface EmployeeScheduleModalProps {
  tenantId: number
  employee: EmployeeSummary | null
  onClose: () => void
  onSaved: () => void
}

const DAYS = ['Lunes', 'Martes', 'Miércoles', 'Jueves', 'Viernes', 'Sábado', 'Domingo']
const DAY_SHORT = ['L', 'M', 'M', 'J', 'V', 'S', 'D']

const SCHEDULE_TYPE_OPTIONS = [
  { value: 'Full-time', label: 'Tiempo Completo (Full-time)' },
  { value: 'Part-time', label: 'Medio Tiempo (Part-time)' }
]

export default function EmployeeScheduleModal({
  tenantId,
  employee,
  onClose,
  onSaved,
}: EmployeeScheduleModalProps) {
  const notify = useNotification()
  const [scheduleType, setScheduleType] = useState('')
  const [selectedDays, setSelectedDays] = useState<string[]>([])
  const [startTime, setStartTime] = useState('')
  const [endTime, setEndTime] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (employee) {
      setScheduleType(employee.schedule_type || '')
      const daysStr = employee.schedule_days || ''
      setSelectedDays(daysStr ? daysStr.split(', ').map(d => d.trim()).filter(Boolean) : [])
      setStartTime(employee.schedule_start_time || '')
      setEndTime(employee.schedule_end_time || '')
      setError(null)
    }
  }, [employee])

  const handleToggleDay = (day: string) => {
    setSelectedDays(prev =>
      prev.includes(day) ? prev.filter(d => d !== day) : [...prev, day]
    )
  }

  const handleClearSchedule = async () => {
    if (!employee) return
    setSaving(true)
    setError(null)
    try {
      await adminService.deleteEmployeeSchedule(tenantId, employee.id)
      notify.success('Horario eliminado correctamente')
      onSaved()
      onClose()
    } catch (err: any) {
      setError(err?.response?.data?.error || 'No se pudo eliminar el horario')
    } finally {
      setSaving(false)
    }
  }

  const handleSave = async () => {
    if (!employee) return
    setSaving(true)
    setError(null)
    try {
      await adminService.updateEmployeeSchedule(tenantId, employee.id, {
        schedule_type: scheduleType,
        schedule_days: selectedDays.join(', '),
        schedule_start_time: startTime,
        schedule_end_time: endTime,
      })
      notify.success('Horario guardado correctamente')
      onSaved()
      onClose()
    } catch (err: any) {
      setError(err?.response?.data?.error || 'No se pudo guardar el horario')
    } finally {
      setSaving(false)
    }
  }

  const isDirty = employee
    ? scheduleType !== (employee.schedule_type || '') ||
      selectedDays.join(', ') !== (employee.schedule_days || '') ||
      startTime !== (employee.schedule_start_time || '') ||
      endTime !== (employee.schedule_end_time || '')
    : false

  return (
    <Modal
      isOpen={employee !== null}
      isDirty={isDirty}
      onClose={onClose}
      title={employee ? `Configurar Horario — ${employee.name}` : 'Configurar Horario'}
      size="md"
      footer={
        <div style={{ display: 'flex', width: '100%', justifyContent: 'space-between', alignItems: 'center' }}>
          <div>
            {(employee?.schedule_type || employee?.schedule_days) && (
              <Button variant="danger" onClick={handleClearSchedule} disabled={saving}>
                Limpiar Horario
              </Button>
            )}
          </div>
          <div style={{ display: 'flex', gap: '8px' }}>
            <Button variant="secondary" onClick={onClose} disabled={saving}>
              Cancelar
            </Button>
            <Button onClick={handleSave} loading={saving}>
              Guardar Horario
            </Button>
          </div>
        </div>
      }
    >
      <p className={styles.modalHint}>
        Configura la jornada y el horario de trabajo semanal para este profesional en la empresa.
      </p>

      <div className={styles.field}>
        <label>Tipo de Jornada</label>
        <Select
          fullWidth
          value={scheduleType}
          onChange={v => setScheduleType(String(v))}
          options={SCHEDULE_TYPE_OPTIONS}
          placeholder="Selecciona tipo de jornada..."
        />
      </div>

      <div className={styles.field} style={{ marginBottom: '20px' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <label style={{ marginBottom: 0 }}>Días de la Semana</label>
          <label style={{ display: 'inline-flex', alignItems: 'center', gap: '8px', fontSize: '13px', fontWeight: 600, color: 'var(--primary)', cursor: 'pointer', userSelect: 'none' }}>
            <div style={{ position: 'relative', display: 'flex', alignItems: 'center' }}>
              <input
                type="checkbox"
                checked={
                  ['Lunes', 'Martes', 'Miércoles', 'Jueves', 'Viernes'].every(d => selectedDays.includes(d))
                }
                onChange={(e) => {
                  if (e.target.checked) {
                    setSelectedDays(prev => {
                      const weekdays = ['Lunes', 'Martes', 'Miércoles', 'Jueves', 'Viernes']
                      const others = prev.filter(d => !weekdays.includes(d))
                      return [...others, ...weekdays]
                    })
                  } else {
                    setSelectedDays(prev => prev.filter(d => !['Lunes', 'Martes', 'Miércoles', 'Jueves', 'Viernes'].includes(d)))
                  }
                }}
                style={{
                  position: 'absolute',
                  opacity: 0,
                  cursor: 'pointer',
                  height: 0,
                  width: 0,
                }}
              />
              <span
                style={{
                  display: 'inline-flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  width: '18px',
                  height: '18px',
                  borderRadius: '5px',
                  border: ['Lunes', 'Martes', 'Miércoles', 'Jueves', 'Viernes'].every(d => selectedDays.includes(d))
                    ? '1.5px solid var(--primary)'
                    : '1.5px solid #cbd5e1',
                  background: ['Lunes', 'Martes', 'Miércoles', 'Jueves', 'Viernes'].every(d => selectedDays.includes(d))
                    ? 'var(--primary)'
                    : 'transparent',
                  transition: 'all 0.15s ease',
                }}
              >
                {['Lunes', 'Martes', 'Miércoles', 'Jueves', 'Viernes'].every(d => selectedDays.includes(d)) && (
                  <svg viewBox="0 0 24 24" fill="none" stroke="white" strokeWidth="4" strokeLinecap="round" strokeLinejoin="round" style={{ width: '12px', height: '12px' }}>
                    <polyline points="20 6 9 17 4 12" />
                  </svg>
                )}
              </span>
            </div>
            Lunes a viernes
          </label>
        </div>
        <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', marginTop: '10px' }}>
          {DAYS.map((day, idx) => {
            const isSelected = selectedDays.includes(day)
            return (
              <button
                key={day}
                type="button"
                onClick={() => handleToggleDay(day)}
                style={{
                  width: '42px',
                  height: '42px',
                  borderRadius: '50%',
                  border: isSelected ? '1px solid var(--primary)' : '1px solid #cbd5e1',
                  background: isSelected ? 'rgba(204, 51, 204, 0.1)' : 'transparent',
                  color: isSelected ? 'var(--primary)' : '#475569',
                  fontWeight: 600,
                  fontSize: '14px',
                  cursor: 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  transition: 'all 0.15s ease',
                }}
                title={day}
              >
                {DAY_SHORT[idx]}
              </button>
            )
          })}
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }}>
        <div className={styles.field}>
          <label>Hora de Inicio</label>
          <input
            type="time"
            value={startTime}
            onChange={e => setStartTime(e.target.value)}
            style={{
              width: '100%',
              padding: '10px 14px',
              border: '1px solid #cbd5e1',
              borderRadius: 'var(--radius)',
              fontSize: '14px',
              outline: 'none',
            }}
          />
        </div>
        <div className={styles.field}>
          <label>Hora de Fin</label>
          <input
            type="time"
            value={endTime}
            onChange={e => setEndTime(e.target.value)}
            style={{
              width: '100%',
              padding: '10px 14px',
              border: '1px solid #cbd5e1',
              borderRadius: 'var(--radius)',
              fontSize: '14px',
              outline: 'none',
            }}
          />
        </div>
      </div>

      {error && <p className={styles.errorMsg} style={{ marginTop: '16px' }}>{error}</p>}
    </Modal>
  )
}
