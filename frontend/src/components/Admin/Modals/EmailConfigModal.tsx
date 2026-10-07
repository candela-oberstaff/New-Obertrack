import { useState } from 'react'
import { Calendar, Users, X, Plus, Trash2, Clock, Globe, CheckCircle2, AlertCircle } from 'lucide-react'
import type { EmailType, EmailRecipient, UpdateEmailSettingPayload } from '../../../services/settings.service'

interface EmailConfigModalProps {
  emailType: EmailType | null
  onClose: () => void
  onSave: (payload: UpdateEmailSettingPayload) => Promise<void>
}

const TIMEZONES = [
  { value: 'America/Santiago', label: 'America/Santiago (Chile, GMT-3)' },
  { value: 'America/Bogota', label: 'America/Bogota (Colombia, GMT-5)' },
  { value: 'America/Caracas', label: 'America/Caracas (Venezuela, GMT-4)' },
  { value: 'America/Buenos_Aires', label: 'America/Buenos_Aires (Argentina, GMT-3)' },
  { value: 'America/Mexico_City', label: 'America/Mexico_City (México, GMT-6)' },
  { value: 'America/Lima', label: 'America/Lima (Perú, GMT-5)' },
  { value: 'America/New_York', label: 'America/New_York (EE.UU. Este, GMT-4)' },
  { value: 'UTC', label: 'UTC (Tiempo Universal Coordinado)' },
]

const WEEKDAYS = [
  { value: 1, label: 'Lunes' },
  { value: 2, label: 'Martes' },
  { value: 3, label: 'Miércoles' },
  { value: 4, label: 'Jueves' },
  { value: 5, label: 'Viernes' },
  { value: 6, label: 'Sábado' },
  { value: 7, label: 'Domingo' },
]

export function EmailConfigModal({ emailType, onClose, onSave }: EmailConfigModalProps) {
  if (!emailType) return null

  const [activeTab, setActiveTab] = useState<'fecha' | 'destinatarios'>('fecha')

  // Estado de fecha / horario
  const [frequency, setFrequency] = useState<'diaria' | 'semanal' | 'mensual'>(
    emailType.frequency || 'diaria'
  )
  const [dayOfMonth, setDayOfMonth] = useState<number>(emailType.day_of_month || 1)
  const [weekday, setWeekday] = useState<number>(emailType.weekday || 1)
  const [hour, setHour] = useState<number>(emailType.hour ?? 16)
  const [minute, setMinute] = useState<number>(emailType.minute ?? 0)
  const [timezone, setTimezone] = useState<string>(emailType.timezone || 'America/Santiago')

  // Estado de destinatarios (pre-cargados automáticamente desde el backend con sus nombres y correos reales)
  const [recipients, setRecipients] = useState<EmailRecipient[]>(
    emailType.recipients && emailType.recipients.length > 0
      ? emailType.recipients
      : [{ email: 'lorena@oberstaff.com', name: 'Lorena Moujalli', enabled: true }]
  )

  // Formulario nuevo destinatario
  const [newEmail, setNewEmail] = useState('')
  const [newName, setNewName] = useState('')
  const [errorMsg, setErrorMsg] = useState('')
  const [isSaving, setIsSaving] = useState(false)

  const handleAddRecipient = (e: React.FormEvent) => {
    e.preventDefault()
    setErrorMsg('')
    const trimmedEmail = newEmail.trim().toLowerCase()
    if (!trimmedEmail || !trimmedEmail.includes('@')) {
      setErrorMsg('Ingresa un correo electrónico válido.')
      return
    }
    if (recipients.some((r) => r.email.toLowerCase() === trimmedEmail)) {
      setErrorMsg('Este correo ya está en la lista.')
      return
    }
    setRecipients([
      ...recipients,
      { email: trimmedEmail, name: newName.trim() || undefined, enabled: true },
    ])
    setNewEmail('')
    setNewName('')
  }

  const handleToggleRecipient = (index: number) => {
    setRecipients((prev) =>
      prev.map((r, i) => (i === index ? { ...r, enabled: !r.enabled } : r))
    )
  }

  const handleRemoveRecipient = (index: number) => {
    setRecipients((prev) => prev.filter((_, i) => i !== index))
  }

  const handleSave = async () => {
    setIsSaving(true)
    try {
      await onSave({
        key: emailType.key,
        enabled: emailType.enabled,
        frequency,
        day_of_month: dayOfMonth,
        weekday,
        hour,
        minute,
        timezone,
        recipients,
      })
      onClose()
    } catch (err: any) {
      setErrorMsg(err?.message || 'Error al guardar la configuración.')
    } finally {
      setIsSaving(false)
    }
  }

  // Formatear hora legible 12-horas AM/PM
  const formatTime12h = (h: number, m: number) => {
    const period = h >= 12 ? 'PM' : 'AM'
    const displayHour = h % 12 === 0 ? 12 : h % 12
    const displayMinute = m < 10 ? `0${m}` : `${m}`
    return `${displayHour}:${displayMinute} ${period}`
  }

  // Generar resumen del envío
  const getScheduleSummary = () => {
    const timeStr = formatTime12h(hour, minute)
    const tzLabel = TIMEZONES.find((t) => t.value === timezone)?.value || timezone

    if (frequency === 'mensual') {
      const dayStr = dayOfMonth === 1 ? 'los primeros (01)' : `el día ${dayOfMonth < 10 ? '0' + dayOfMonth : dayOfMonth}`
      return `Este correo se enviará ${dayStr} de cada mes a las ${timeStr} (${tzLabel}).`
    }
    if (frequency === 'semanal') {
      const dayName = WEEKDAYS.find((w) => w.value === weekday)?.label || 'Lunes'
      return `Este correo se enviará todos los ${dayName}s a las ${timeStr} (${tzLabel}).`
    }
    return `Este correo se enviará todos los días a las ${timeStr} (${tzLabel}).`
  }

  const activeRecipientsCount = recipients.filter((r) => r.enabled).length

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        backgroundColor: 'rgba(15, 23, 42, 0.6)',
        backdropFilter: 'blur(4px)',
        zIndex: 9999,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: 16,
      }}
      onClick={onClose}
    >
      <div
        style={{
          background: '#ffffff',
          width: '100%',
          maxWidth: 620,
          borderRadius: 20,
          boxShadow: '0 25px 50px -12px rgba(0, 0, 0, 0.25)',
          overflow: 'hidden',
          display: 'flex',
          flexDirection: 'column',
          maxHeight: '90vh',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Cabecera */}
        <div
          style={{
            padding: '20px 24px',
            borderBottom: '1px solid #e2e8f0',
            display: 'flex',
            alignItems: 'flex-start',
            justifyContent: 'space-between',
            background: 'linear-gradient(to right, #f8fafc, #ffffff)',
          }}
        >
          <div>
            <span
              style={{
                fontSize: 11,
                fontWeight: 700,
                color: '#6366f1',
                textTransform: 'uppercase',
                letterSpacing: '0.05em',
              }}
            >
              Ajustes de correo automático
            </span>
            <h3 style={{ margin: '2px 0 4px', fontSize: 18, fontWeight: 800, color: '#0f172a' }}>
              {emailType.name}
            </h3>
            <p style={{ margin: 0, fontSize: 13, color: '#64748b' }}>{emailType.description}</p>
          </div>
          <button
            onClick={onClose}
            style={{
              background: '#f1f5f9',
              border: 'none',
              borderRadius: 10,
              padding: 8,
              cursor: 'pointer',
              color: '#64748b',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <X size={18} />
          </button>
        </div>

        {/* Pestañas (Tabs) */}
        <div
          style={{
            display: 'flex',
            borderBottom: '1px solid #e2e8f0',
            background: '#f8fafc',
            padding: '0 24px',
            gap: 12,
          }}
        >
          <button
            onClick={() => setActiveTab('fecha')}
            style={{
              padding: '12px 16px',
              fontSize: 13.5,
              fontWeight: 700,
              color: activeTab === 'fecha' ? '#4f46e5' : '#64748b',
              borderBottom: activeTab === 'fecha' ? '2.5px solid #4f46e5' : '2.5px solid transparent',
              background: 'none',
              borderTop: 'none',
              borderLeft: 'none',
              borderRight: 'none',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: 8,
              transition: 'all 0.2s',
            }}
          >
            <Calendar size={16} /> Fecha y Horario
          </button>

          <button
            onClick={() => setActiveTab('destinatarios')}
            style={{
              padding: '12px 16px',
              fontSize: 13.5,
              fontWeight: 700,
              color: activeTab === 'destinatarios' ? '#4f46e5' : '#64748b',
              borderBottom: activeTab === 'destinatarios' ? '2.5px solid #4f46e5' : '2.5px solid transparent',
              background: 'none',
              borderTop: 'none',
              borderLeft: 'none',
              borderRight: 'none',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: 8,
              transition: 'all 0.2s',
            }}
          >
            <Users size={16} /> Destinatarios
            <span
              style={{
                background: activeTab === 'destinatarios' ? '#e0e7ff' : '#e2e8f0',
                color: activeTab === 'destinatarios' ? '#4338ca' : '#475569',
                padding: '1px 7px',
                borderRadius: 999,
                fontSize: 11.5,
                fontWeight: 700,
              }}
            >
              {activeRecipientsCount}/{recipients.length}
            </span>
          </button>
        </div>

        {/* Cuerpo del Modal */}
        <div style={{ padding: 24, overflowY: 'auto', flex: 1 }}>
          {errorMsg && (
            <div
              style={{
                padding: '10px 14px',
                background: '#fef2f2',
                border: '1px solid #fecaca',
                borderRadius: 10,
                color: '#b91c1c',
                fontSize: 13,
                marginBottom: 16,
                display: 'flex',
                alignItems: 'center',
                gap: 8,
              }}
            >
              <AlertCircle size={16} /> {errorMsg}
            </div>
          )}

          {/* TAB 1: FECHA Y HORARIO */}
          {activeTab === 'fecha' && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
              {/* Frecuencia */}
              <div>
                <label style={{ display: 'block', fontSize: 13, fontWeight: 700, color: '#334155', marginBottom: 8 }}>
                  Frecuencia de envío
                </label>
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 10 }}>
                  {(['diaria', 'semanal', 'mensual'] as const).map((freq) => (
                    <button
                      key={freq}
                      type="button"
                      onClick={() => setFrequency(freq)}
                      style={{
                        padding: '10px 12px',
                        borderRadius: 10,
                        border: frequency === freq ? '2px solid #6366f1' : '1px solid #cbd5e1',
                        background: frequency === freq ? '#eeeffe' : '#ffffff',
                        color: frequency === freq ? '#4338ca' : '#334155',
                        fontWeight: frequency === freq ? 700 : 500,
                        fontSize: 13.5,
                        textTransform: 'capitalize',
                        cursor: 'pointer',
                        textAlign: 'center',
                      }}
                    >
                      {freq}
                    </button>
                  ))}
                </div>
              </div>

              {/* Ajuste según Frecuencia */}
              {frequency === 'mensual' && (
                <div>
                  <label style={{ display: 'block', fontSize: 13, fontWeight: 700, color: '#334155', marginBottom: 6 }}>
                    Día del mes
                  </label>
                  <select
                    value={dayOfMonth}
                    onChange={(e) => setDayOfMonth(Number(e.target.value))}
                    style={{
                      width: '100%',
                      padding: '9px 12px',
                      borderRadius: 10,
                      border: '1px solid #cbd5e1',
                      fontSize: 13.5,
                      background: '#fff',
                    }}
                  >
                    {Array.from({ length: 31 }, (_, i) => i + 1).map((d) => (
                      <option key={d} value={d}>
                        Día {d < 10 ? `0${d}` : d} {d === 1 ? '(Primer día del mes)' : ''}
                      </option>
                    ))}
                  </select>
                </div>
              )}

              {frequency === 'semanal' && (
                <div>
                  <label style={{ display: 'block', fontSize: 13, fontWeight: 700, color: '#334155', marginBottom: 6 }}>
                    Día de la semana
                  </label>
                  <select
                    value={weekday}
                    onChange={(e) => setWeekday(Number(e.target.value))}
                    style={{
                      width: '100%',
                      padding: '9px 12px',
                      borderRadius: 10,
                      border: '1px solid #cbd5e1',
                      fontSize: 13.5,
                      background: '#fff',
                    }}
                  >
                    {WEEKDAYS.map((w) => (
                      <option key={w.value} value={w.value}>
                        {w.label}
                      </option>
                    ))}
                  </select>
                </div>
              )}

              {/* Hora y Minutos */}
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
                <div>
                  <label style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 13, fontWeight: 700, color: '#334155', marginBottom: 6 }}>
                    <Clock size={14} /> Hora (00 - 23)
                  </label>
                  <select
                    value={hour}
                    onChange={(e) => setHour(Number(e.target.value))}
                    style={{
                      width: '100%',
                      padding: '9px 12px',
                      borderRadius: 10,
                      border: '1px solid #cbd5e1',
                      fontSize: 13.5,
                      background: '#fff',
                    }}
                  >
                    {Array.from({ length: 24 }, (_, i) => i).map((h) => (
                      <option key={h} value={h}>
                        {formatTime12h(h, 0).split(' ')[0]} {formatTime12h(h, 0).split(' ')[1]} ({h < 10 ? `0${h}` : h}:00)
                      </option>
                    ))}
                  </select>
                </div>

                <div>
                  <label style={{ display: 'block', fontSize: 13, fontWeight: 700, color: '#334155', marginBottom: 6 }}>
                    Minutos
                  </label>
                  <select
                    value={minute}
                    onChange={(e) => setMinute(Number(e.target.value))}
                    style={{
                      width: '100%',
                      padding: '9px 12px',
                      borderRadius: 10,
                      border: '1px solid #cbd5e1',
                      fontSize: 13.5,
                      background: '#fff',
                    }}
                  >
                    {[0, 15, 30, 45].map((m) => (
                      <option key={m} value={m}>
                        :{m < 10 ? `0${m}` : m}
                      </option>
                    ))}
                  </select>
                </div>
              </div>

              {/* Zona horaria */}
              <div>
                <label style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 13, fontWeight: 700, color: '#334155', marginBottom: 6 }}>
                  <Globe size={14} /> Zona Horaria
                </label>
                <select
                  value={timezone}
                  onChange={(e) => setTimezone(e.target.value)}
                  style={{
                    width: '100%',
                    padding: '9px 12px',
                    borderRadius: 10,
                    border: '1px solid #cbd5e1',
                    fontSize: 13.5,
                    background: '#fff',
                  }}
                >
                  {TIMEZONES.map((tz) => (
                    <option key={tz.value} value={tz.value}>
                      {tz.label}
                    </option>
                  ))}
                </select>
              </div>

              {/* Banner de resumen */}
              <div
                style={{
                  padding: '14px 16px',
                  background: '#f0fdf4',
                  border: '1px solid #bbf7d0',
                  borderRadius: 12,
                  display: 'flex',
                  alignItems: 'center',
                  gap: 10,
                  fontSize: 13,
                  color: '#166534',
                  lineHeight: 1.4,
                }}
              >
                <CheckCircle2 size={18} style={{ flexShrink: 0, color: '#16a34a' }} />
                <span>
                  <strong>Programación actual:</strong> {getScheduleSummary()}
                </span>
              </div>
            </div>
          )}

          {/* TAB 2: DESTINATARIOS */}
          {activeTab === 'destinatarios' && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 18 }}>
              <p style={{ margin: 0, fontSize: 13, color: '#64748b' }}>
                Selecciona a quiénes se les enviará este correo automático. Marca o desmarca de la lista o agrega personas adicionales.
              </p>

              {/* Lista de destinatarios */}
              <div
                style={{
                  display: 'flex',
                  flexDirection: 'column',
                  gap: 8,
                  maxHeight: 220,
                  overflowY: 'auto',
                  border: '1px solid #e2e8f0',
                  borderRadius: 12,
                  padding: 8,
                  background: '#f8fafc',
                }}
              >
                {recipients.length === 0 ? (
                  <div style={{ padding: '16px', textAlign: 'center', color: '#94a3b8', fontSize: 13 }}>
                    No hay destinatarios agregados todavía.
                  </div>
                ) : (
                  recipients.map((rec, idx) => (
                    <div
                      key={idx}
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                        padding: '10px 12px',
                        background: rec.enabled ? '#ffffff' : '#f1f5f9',
                        border: `1px solid ${rec.enabled ? '#cbd5e1' : '#e2e8f0'}`,
                        borderRadius: 10,
                        opacity: rec.enabled ? 1 : 0.65,
                      }}
                    >
                      <label style={{ display: 'flex', alignItems: 'center', gap: 10, cursor: 'pointer', flex: 1, minWidth: 0 }}>
                        <input
                          type="checkbox"
                          checked={rec.enabled}
                          onChange={() => handleToggleRecipient(idx)}
                          style={{ width: 16, height: 16, cursor: 'pointer', accentColor: '#4f46e5' }}
                        />
                        <div style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                          <span style={{ fontWeight: 600, fontSize: 13.5, color: '#0f172a' }}>
                            {rec.email}
                          </span>
                          {rec.name && (
                            <span style={{ marginLeft: 8, fontSize: 12, color: '#64748b' }}>
                              ({rec.name})
                            </span>
                          )}
                        </div>
                      </label>

                      <button
                        type="button"
                        onClick={() => handleRemoveRecipient(idx)}
                        title="Quitar destinatario"
                        style={{
                          background: 'none',
                          border: 'none',
                          color: '#94a3b8',
                          cursor: 'pointer',
                          padding: 4,
                          borderRadius: 6,
                          display: 'flex',
                          alignItems: 'center',
                        }}
                        onMouseEnter={(e) => (e.currentTarget.style.color = '#ef4444')}
                        onMouseLeave={(e) => (e.currentTarget.style.color = '#94a3b8')}
                      >
                        <Trash2 size={15} />
                      </button>
                    </div>
                  ))
                )}
              </div>

              {/* Agregar nuevo destinatario */}
              <form onSubmit={handleAddRecipient} style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                <label style={{ fontSize: 12.5, fontWeight: 700, color: '#334155' }}>
                  + Agregar otro destinatario
                </label>
                <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                  <input
                    type="email"
                    placeholder="correo@ejemplo.com"
                    value={newEmail}
                    onChange={(e) => setNewEmail(e.target.value)}
                    style={{
                      flex: 2,
                      minWidth: 180,
                      padding: '8px 12px',
                      borderRadius: 10,
                      border: '1px solid #cbd5e1',
                      fontSize: 13,
                    }}
                  />
                  <input
                    type="text"
                    placeholder="Nombre (opcional)"
                    value={newName}
                    onChange={(e) => setNewName(e.target.value)}
                    style={{
                      flex: 1,
                      minWidth: 140,
                      padding: '8px 12px',
                      borderRadius: 10,
                      border: '1px solid #cbd5e1',
                      fontSize: 13,
                    }}
                  />
                  <button
                    type="submit"
                    style={{
                      display: 'inline-flex',
                      alignItems: 'center',
                      gap: 6,
                      background: '#4f46e5',
                      color: '#fff',
                      border: 'none',
                      borderRadius: 10,
                      padding: '8px 14px',
                      fontSize: 13,
                      fontWeight: 700,
                      cursor: 'pointer',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    <Plus size={15} /> Agregar
                  </button>
                </div>
              </form>

              {/* Nota informativa */}
              <div
                style={{
                  padding: '12px 14px',
                  background: '#f8fafc',
                  border: '1px solid #e2e8f0',
                  borderRadius: 10,
                  fontSize: 12.5,
                  color: '#475569',
                }}
              >
                💡 <strong>Nota:</strong> Los destinatarios con casilla de verificación marcada recibirán los correos de esta alerta. Si desmarcas a alguien, no se le enviará.
              </div>
            </div>
          )}
        </div>

        {/* Pie de página (Footer) */}
        <div
          style={{
            padding: '16px 24px',
            borderTop: '1px solid #e2e8f0',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'flex-end',
            gap: 12,
            background: '#f8fafc',
          }}
        >
          <button
            type="button"
            onClick={onClose}
            disabled={isSaving}
            style={{
              padding: '9px 16px',
              borderRadius: 10,
              border: '1px solid #cbd5e1',
              background: '#ffffff',
              color: '#334155',
              fontWeight: 600,
              fontSize: 13.5,
              cursor: 'pointer',
            }}
          >
            Cancelar
          </button>

          <button
            type="button"
            onClick={handleSave}
            disabled={isSaving}
            style={{
              padding: '9px 20px',
              borderRadius: 10,
              border: 'none',
              background: '#4f46e5',
              color: '#ffffff',
              fontWeight: 700,
              fontSize: 13.5,
              cursor: isSaving ? 'wait' : 'pointer',
              display: 'inline-flex',
              alignItems: 'center',
              gap: 8,
              boxShadow: '0 2px 4px rgba(79, 70, 229, 0.25)',
            }}
          >
            {isSaving ? 'Guardando…' : 'Guardar cambios'}
          </button>
        </div>
      </div>
    </div>
  )
}
