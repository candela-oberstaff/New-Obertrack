import { useState } from 'react'
import { Mail, Send, AlertTriangle, Settings2, Power, PowerOff } from 'lucide-react'
import { useEmailSettings } from '../../hooks/useEmailSettings'
import { useNotification } from '../../context/NotificationContext'
import { useConfirm } from '../ui/ConfirmProvider'
import { Skeleton } from '../ui'
import type { EmailType } from '../../services/settings.service'
import styles from './Admin.module.css'

const CATEGORY_LABEL: Record<EmailType['category'], string> = {
  automatic: 'Automáticos',
  event: 'Por evento',
  manual: 'Manuales',
}

const CATEGORY_HINT: Record<EmailType['category'], string> = {
  automatic: 'Los envía el sistema solo, según su horario.',
  event: 'Salen cuando ocurre una acción en la plataforma.',
  manual: 'Los envía alguien del equipo desde la plataforma.',
}

const CATEGORY_ORDER: EmailType['category'][] = ['automatic', 'event', 'manual']

/**
 * Configuración → Correos: todas las salidas de correo del sistema, con su
 * interruptor y un envío de prueba para revisar el formato.
 */
export function EmailTypesPanel() {
  const { types, isLoading, toggle, togglingKey, toggleAll, isTogglingAll, sendTest, testingKey } = useEmailSettings()
  const { success, error: showError } = useNotification()
  const confirm = useConfirm()
  const [testEmail, setTestEmail] = useState('')

  const allOff = types.length > 0 && types.every((t) => !t.enabled)
  const offCount = types.filter((t) => !t.enabled).length

  const handleToggleAll = async (enabled: boolean) => {
    if (!enabled) {
      const ok = await confirm({
        title: '¿Apagar todos los correos?',
        message: 'Obertrack dejará de enviar CUALQUIER correo, incluidos los de recuperar y crear contraseña (las personas no podrán entrar por su cuenta) y el reporte de jornadas a las empresas. Las notificaciones dentro de la app no se ven afectadas. Puedes revertirlo desde aquí en cualquier momento.',
        confirmLabel: 'Apagar todo',
        variant: 'danger',
      })
      if (!ok) return
    }
    try {
      await toggleAll(enabled)
      success(enabled ? 'Todos los correos quedaron activos.' : 'Se apagaron todos los correos del sistema.')
    } catch {
      showError('No se pudo aplicar el cambio general.')
    }
  }

  const handleToggle = async (t: EmailType) => {
    // Apagar un correo esencial deja gente sin poder entrar: se confirma.
    if (t.enabled && t.essential) {
      const ok = await confirm({
        title: `¿Apagar «${t.name}»?`,
        message: 'Es un correo esencial: sin él, las personas no pueden crear ni recuperar su contraseña por su cuenta y habrá que asistirlas una por una.',
        confirmLabel: 'Apagarlo igualmente',
        variant: 'danger',
      })
      if (!ok) return
    }
    try {
      await toggle({ key: t.key, enabled: !t.enabled })
      success(!t.enabled ? `«${t.name}» activado.` : `«${t.name}» desactivado.`)
    } catch {
      showError('No se pudo cambiar el estado del correo.')
    }
  }

  const handleTest = async (t: EmailType) => {
    try {
      const res = await sendTest({ key: t.key, email: testEmail.trim() || undefined })
      success(res.message)
    } catch (e: any) {
      showError(e?.response?.data?.error || 'No se pudo enviar el correo de prueba.')
    }
  }

  if (isLoading) return <Skeleton height={220} />

  return (
    <div style={{ background: '#fff', border: '1px solid #e2e8f0', borderRadius: 16, padding: 24 }}>
      <h2 style={{ margin: '0 0 4px', fontSize: 17, fontWeight: 800, color: '#0f172a', display: 'flex', alignItems: 'center', gap: 8 }}>
        <Mail size={19} /> Correos
      </h2>
      <p style={{ margin: '0 0 16px', color: '#64748b', fontSize: 13 }}>
        Todas las salidas de correo de Obertrack. Apaga las que no quieras que salgan y envía una prueba para ver cómo llega el formato.
      </p>

      {/* Interruptor general + estado global, lo primero que se ve. */}
      <div
        style={{
          display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 16, flexWrap: 'wrap',
          padding: '14px 16px', marginBottom: 16, borderRadius: 12,
          background: allOff ? '#fef2f2' : '#f8fafc',
          border: `1px solid ${allOff ? '#fecaca' : '#e2e8f0'}`,
        }}
      >
        <div style={{ minWidth: 260, flex: 1 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontWeight: 700, fontSize: 14, color: allOff ? '#b91c1c' : '#0f172a' }}>
            {allOff ? <PowerOff size={16} /> : <Power size={16} />}
            {allOff ? 'Correo apagado por completo' : 'Correo activo'}
          </div>
          <p style={{ margin: '4px 0 0', fontSize: 12.5, color: allOff ? '#b91c1c' : '#64748b' }}>
            {allOff
              ? 'Obertrack no está enviando ningún correo. Las notificaciones dentro de la app siguen funcionando.'
              : offCount > 0
                ? `${offCount} de ${types.length} tipos están apagados.`
                : 'Todos los tipos de correo están activos.'}
          </p>
        </div>
        <button
          onClick={() => handleToggleAll(allOff)}
          disabled={isTogglingAll}
          style={{
            display: 'inline-flex', alignItems: 'center', gap: 7, border: 'none', borderRadius: 10,
            padding: '10px 16px', fontSize: 13, fontWeight: 700, cursor: isTogglingAll ? 'wait' : 'pointer',
            width: 'auto', whiteSpace: 'nowrap',
            background: allOff ? '#16a34a' : '#dc2626', color: '#fff',
          }}
        >
          {allOff ? <Power size={14} /> : <PowerOff size={14} />}
          {isTogglingAll ? 'Aplicando…' : allOff ? 'Encender todos' : 'Apagar todos'}
        </button>
      </div>

      <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap', marginBottom: 20, padding: '12px 14px', background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: 12 }}>
        <span style={{ fontSize: 13, fontWeight: 600, color: '#475569', whiteSpace: 'nowrap' }}>Enviar pruebas a:</span>
        <input
          type="email"
          value={testEmail}
          onChange={(e) => setTestEmail(e.target.value)}
          placeholder="tu-correo@empresa.com (por defecto, el tuyo)"
          style={{ flex: 1, minWidth: 240, padding: '8px 12px', border: '1px solid #cbd5e1', borderRadius: 10, fontSize: 13.5 }}
        />
      </div>

      {CATEGORY_ORDER.map((cat) => {
        const rows = types.filter((t) => t.category === cat)
        if (rows.length === 0) return null
        return (
          <div key={cat} style={{ marginBottom: 22 }}>
            <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, marginBottom: 10 }}>
              <h3 style={{ margin: 0, fontSize: 13, fontWeight: 800, color: '#334155', textTransform: 'uppercase', letterSpacing: '0.04em' }}>
                {CATEGORY_LABEL[cat]}
              </h3>
              <span style={{ fontSize: 12, color: '#94a3b8' }}>{CATEGORY_HINT[cat]}</span>
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
              {rows.map((t) => (
                <div
                  key={t.key}
                  style={{
                    display: 'flex', alignItems: 'flex-start', gap: 16, flexWrap: 'wrap',
                    padding: '14px 16px', border: '1px solid #e2e8f0', borderRadius: 12,
                    background: t.enabled ? '#fff' : '#f8fafc',
                    opacity: t.enabled ? 1 : 0.75,
                  }}
                >
                  <div style={{ flex: 1, minWidth: 260 }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                      <span style={{ fontWeight: 700, fontSize: 14, color: '#0f172a' }}>{t.name}</span>
                      {t.essential && (
                        <span title="Sin este correo, las personas no pueden crear ni recuperar su contraseña." style={{ display: 'inline-flex', alignItems: 'center', gap: 4, background: '#fef3c7', color: '#92400e', padding: '2px 8px', borderRadius: 999, fontSize: 11, fontWeight: 700 }}>
                          <AlertTriangle size={11} /> Esencial
                        </span>
                      )}
                      {!t.enabled && (
                        <span style={{ background: '#e2e8f0', color: '#475569', padding: '2px 8px', borderRadius: 999, fontSize: 11, fontWeight: 700 }}>
                          Apagado
                        </span>
                      )}
                    </div>
                    <p style={{ margin: '4px 0 0', fontSize: 13, color: '#475569' }}>{t.description}</p>
                    <p style={{ margin: '4px 0 0', fontSize: 12, color: '#94a3b8' }}>
                      <strong style={{ color: '#64748b' }}>Cuándo:</strong> {t.trigger}
                    </p>
                    <p style={{ margin: '2px 0 0', fontSize: 12, color: '#94a3b8' }}>
                      <strong style={{ color: '#64748b' }}>A quién:</strong> {t.recipient}
                    </p>
                    {t.managed_elsewhere && (
                      <p style={{ margin: '6px 0 0', fontSize: 12, color: '#7c3aed', display: 'inline-flex', alignItems: 'center', gap: 5 }}>
                        <Settings2 size={12} /> {t.managed_elsewhere}
                      </p>
                    )}
                  </div>

                  <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                    <button
                      onClick={() => handleTest(t)}
                      disabled={testingKey === t.key}
                      title="Enviar una muestra de este correo"
                      style={{ display: 'inline-flex', alignItems: 'center', gap: 6, border: '1px solid #e2e8f0', background: '#fff', color: '#334155', borderRadius: 10, padding: '8px 12px', fontSize: 13, fontWeight: 700, cursor: testingKey === t.key ? 'wait' : 'pointer', width: 'auto', whiteSpace: 'nowrap' }}
                    >
                      <Send size={13} /> {testingKey === t.key ? 'Enviando…' : 'Probar'}
                    </button>

                    {!t.managed_elsewhere && (
                      <label className={styles['checkbox-label']} style={{ margin: 0, cursor: togglingKey === t.key ? 'wait' : 'pointer' }}>
                        <input
                          type="checkbox"
                          checked={t.enabled}
                          disabled={togglingKey === t.key}
                          onChange={() => handleToggle(t)}
                        />
                      </label>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )
      })}
    </div>
  )
}
