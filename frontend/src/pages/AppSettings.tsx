import { useEffect, useState } from 'react'
import { CheckCircle2, Clock, Play, Save, XCircle } from 'lucide-react'
import { useReportSchedule } from '../hooks/useReportSchedule'
import { useNotification } from '../context/NotificationContext'
import { Button, Select, Skeleton } from '../components/ui'
import type { ReportFrequency, ReportSchedule } from '../services/api'
import styles from '../components/Admin/Admin.module.css'

const FREQUENCIES: { value: ReportFrequency; label: string }[] = [
  { value: 'daily', label: 'Diaria' },
  { value: 'weekly', label: 'Semanal' },
  { value: 'monthly', label: 'Mensual' },
]

const WEEKDAYS = [
  { value: 1, label: 'Lunes' },
  { value: 2, label: 'Martes' },
  { value: 3, label: 'Miércoles' },
  { value: 4, label: 'Jueves' },
  { value: 5, label: 'Viernes' },
  { value: 6, label: 'Sábado' },
  { value: 0, label: 'Domingo' },
]

// Lista curada: la IANA completa son cientos de zonas y no aporta nada acá.
const TIMEZONES = [
  'UTC',
  'America/Caracas',
  'America/Bogota',
  'America/Lima',
  'America/Mexico_City',
  'America/Santiago',
  'America/Argentina/Buenos_Aires',
  'America/Sao_Paulo',
  'America/New_York',
  'Europe/Madrid',
]

const HOURS = Array.from({ length: 24 }, (_, i) => ({ value: i, label: String(i).padStart(2, '0') }))
const MINUTES = [0, 15, 30, 45].map((m) => ({ value: m, label: String(m).padStart(2, '0') }))
const DAYS_OF_MONTH = Array.from({ length: 28 }, (_, i) => ({ value: i + 1, label: String(i + 1) }))

// Explica en una frase qué va a pasar, para que nadie tenga que adivinar.
function describe(cfg: ReportSchedule): string {
  const at = `${String(cfg.hour).padStart(2, '0')}:${String(cfg.minute).padStart(2, '0')} (${cfg.timezone})`
  if (cfg.frequency === 'daily') return `Cada día a las ${at} se envía el reporte del día anterior.`
  if (cfg.frequency === 'weekly') {
    const d = WEEKDAYS.find((w) => w.value === cfg.weekday)?.label ?? '—'
    return `Cada ${d.toLowerCase()} a las ${at} se envía el reporte de la semana anterior (lunes a domingo).`
  }
  return `El día ${cfg.day_of_month} de cada mes a las ${at} se envía el reporte del mes anterior.`
}

export default function AppSettings() {
  const { schedule, runs, isLoading, save, isSaving, runNow, isRunning } = useReportSchedule()
  const { success, error: showError } = useNotification()
  const [form, setForm] = useState<ReportSchedule | null>(null)

  useEffect(() => {
    if (schedule) setForm(schedule)
  }, [schedule])

  const handleSave = async () => {
    if (!form) return
    try {
      await save({
        enabled: form.enabled,
        frequency: form.frequency,
        hour: form.hour,
        minute: form.minute,
        timezone: form.timezone,
        weekday: form.weekday,
        day_of_month: form.day_of_month,
      })
      success('Configuración guardada.')
    } catch (err: any) {
      showError(err?.response?.data?.error ?? 'No se pudo guardar la configuración.')
    }
  }

  const handleRunNow = async () => {
    try {
      const res = await runNow()
      const parts = [`${res.sent} enviado(s)`]
      if (res.skipped) parts.push(`${res.skipped} omitido(s) (ya se habían enviado)`)
      if (res.failed) parts.push(`${res.failed} con error`)
      success(parts.join(' · '))
    } catch (err: any) {
      showError(err?.response?.data?.error ?? 'No se pudo ejecutar la corrida.')
    }
  }

  const fmtDate = (s: string) =>
    new Date(s).toLocaleString('es-ES', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' })

  if (isLoading || !form) {
    return (
      <div style={{ padding: 24 }}>
        <Skeleton height={40} radius={12} />
        <div style={{ marginTop: 16 }}><Skeleton height={280} radius={16} /></div>
      </div>
    )
  }

  return (
    <div style={{ padding: 24, maxWidth: 1500, margin: '0 auto' }}>
      <div style={{ marginBottom: 24 }}>
        <h1 style={{ margin: 0, fontSize: 28, fontWeight: 800, color: '#0f172a' }}>Configuración</h1>
        <p style={{ margin: '6px 0 0', color: '#64748b', fontSize: 14 }}>
          Ajustes globales de la plataforma. Solo visible para superadmins.
        </p>
      </div>

      {/* Dos columnas en pantallas anchas, una sola cuando no entran. Sin media
          queries: auto-fit colapsa la grilla por sí solo. */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(460px, 1fr))', gap: 24, alignItems: 'start' }}>
        <div style={{ background: '#fff', border: '1px solid #e2e8f0', borderRadius: 16, padding: 24 }}>
        <h2 style={{ margin: '0 0 4px', fontSize: 17, fontWeight: 800, color: '#0f172a', display: 'flex', alignItems: 'center', gap: 8 }}>
          <Clock size={19} /> Envío automático de reportes
        </h2>
        <p style={{ margin: '0 0 20px', color: '#64748b', fontSize: 13 }}>
          Cada empresa recibe por correo el reporte de jornadas de su equipo, con PDF y Excel adjuntos.
        </p>

        <div className={styles['permissions-group']} style={{ marginBottom: 20 }}>
          <label className={styles['checkbox-label']}>
            <span>Activar envío automático</span>
            <input
              type="checkbox"
              checked={form.enabled}
              onChange={(e) => setForm({ ...form, enabled: e.target.checked })}
            />
          </label>
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: 16 }}>
          <div className={styles['form-group']}>
            <label>Frecuencia</label>
            <Select
              fullWidth
              value={form.frequency}
              onChange={(v) => setForm({ ...form, frequency: v as ReportFrequency })}
              options={FREQUENCIES}
            />
          </div>

          {form.frequency === 'weekly' && (
            <div className={styles['form-group']}>
              <label>Día de la semana</label>
              <Select fullWidth value={form.weekday} onChange={(v) => setForm({ ...form, weekday: Number(v) })} options={WEEKDAYS} />
            </div>
          )}

          {form.frequency === 'monthly' && (
            <div className={styles['form-group']}>
              <label>Día del mes</label>
              <Select fullWidth value={form.day_of_month} onChange={(v) => setForm({ ...form, day_of_month: Number(v) })} options={DAYS_OF_MONTH} />
              <small style={{ color: '#94a3b8', fontSize: 12 }}>Máximo 28, para que exista en todos los meses.</small>
            </div>
          )}

          <div className={styles['form-group']}>
            <label>Hora</label>
            <Select fullWidth value={form.hour} onChange={(v) => setForm({ ...form, hour: Number(v) })} options={HOURS} />
          </div>

          <div className={styles['form-group']}>
            <label>Minutos</label>
            <Select fullWidth value={form.minute} onChange={(v) => setForm({ ...form, minute: Number(v) })} options={MINUTES} />
          </div>

          <div className={styles['form-group']}>
            <label>Zona horaria</label>
            <Select
              fullWidth
              value={form.timezone}
              onChange={(v) => setForm({ ...form, timezone: String(v) })}
              options={TIMEZONES.map((tz) => ({ value: tz, label: tz }))}
            />
          </div>
        </div>

        <div style={{ marginTop: 20, padding: '12px 14px', background: '#eff6ff', border: '1px solid #bfdbfe', borderRadius: 10, color: '#1d4ed8', fontSize: 13, fontWeight: 600 }}>
          {form.enabled ? describe(form) : 'El envío automático está desactivado.'}
        </div>

        <div style={{ display: 'flex', gap: 10, marginTop: 20, flexWrap: 'wrap' }}>
          <Button leftIcon={<Save size={16} />} loading={isSaving} onClick={handleSave}>
            Guardar cambios
          </Button>
          <Button variant="secondary" leftIcon={<Play size={16} />} loading={isRunning} onClick={handleRunNow}>
            Enviar ahora (prueba)
          </Button>
        </div>
        <p style={{ margin: '10px 0 0', color: '#94a3b8', fontSize: 12 }}>
          "Enviar ahora" ignora la hora programada, pero no reenvía un período que ya se entregó.
        </p>
      </div>

      <div style={{ background: '#fff', border: '1px solid #e2e8f0', borderRadius: 16, padding: 24 }}>
        <h2 style={{ margin: '0 0 16px', fontSize: 16, fontWeight: 800, color: '#0f172a' }}>Últimos envíos</h2>
        {runs.length === 0 ? (
          <p style={{ color: '#94a3b8', fontSize: 14, margin: 0 }}>Todavía no se envió ningún reporte automático.</p>
        ) : (
          <div className={styles['users-table'] || 'users-table'}>
            <table>
              <thead>
                <tr>
                  <th>Empresa</th>
                  <th>Período</th>
                  <th>Estado</th>
                  <th>Fecha</th>
                </tr>
              </thead>
              <tbody>
                {runs.map((r) => (
                  <tr key={r.id}>
                    <td>
                      <div style={{ fontWeight: 600, color: '#0f172a' }}>{r.recipient_name || `#${r.tenant_id}`}</div>
                      <div style={{ fontSize: 12, color: '#94a3b8' }}>{r.recipient_email}</div>
                    </td>
                    <td style={{ fontSize: 13, color: '#475569' }}>{r.period_key}</td>
                    <td>
                      {r.status === 'sent' ? (
                        <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, background: '#dcfce7', color: '#15803d', padding: '4px 10px', borderRadius: 999, fontSize: 12, fontWeight: 700 }}>
                          <CheckCircle2 size={13} /> Enviado
                        </span>
                      ) : (
                        <span title={r.error} style={{ display: 'inline-flex', alignItems: 'center', gap: 6, background: '#fee2e2', color: '#b91c1c', padding: '4px 10px', borderRadius: 999, fontSize: 12, fontWeight: 700 }}>
                          <XCircle size={13} /> Error
                        </span>
                      )}
                    </td>
                    <td style={{ fontSize: 13, color: '#64748b', whiteSpace: 'nowrap' }}>{fmtDate(r.created_at)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        </div>
      </div>
    </div>
  )
}
