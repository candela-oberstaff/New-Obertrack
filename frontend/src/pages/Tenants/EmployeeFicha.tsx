import { Clock, CheckSquare, CalendarClock, ShieldCheck } from 'lucide-react'
import type { EmployeeSummary, User } from '../../types'
import { formatDateOnly } from '../../utils/date'
import styles from './Tenants.module.css'

// Estado del portero de inducción, en el idioma de quien atiende: lo que
// importa desde aquí no es el valor del campo sino si la persona puede entrar.
const ONBOARDING_STATE: Record<string, { label: string; pill: string }> = {
  not_required: { label: 'Acceso directo', pill: styles.pillNeutral },
  pending: { label: 'Inducción pendiente', pill: styles.pillWarn },
  passed: { label: 'Inducción aprobada', pill: styles.pillSuccess },
  blocked: { label: 'Bloqueado por intentos', pill: styles.pillDanger },
}

/**
 * Antigüedad en la plataforma. La fecha de alta sola no distingue a quien acaba
 * de entrar (y por eso no tiene jornadas) de quien lleva un año sin registrar
 * nada, que es justo lo que se viene a mirar aquí.
 */
export function timeSince(iso?: string | null): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (isNaN(d.getTime())) return ''
  const days = Math.floor((Date.now() - d.getTime()) / 86_400_000)
  if (days <= 0) return 'hoy'
  if (days === 1) return 'ayer'
  if (days < 30) return `hace ${days} días`
  const months = Math.floor(days / 30)
  if (months < 12) return `hace ${months} ${months === 1 ? 'mes' : 'meses'}`
  const years = Math.floor(days / 365)
  return `hace ${years} ${years === 1 ? 'año' : 'años'}`
}

/** Un dato de la ficha. El hueco se dice, no se esconde: un cargo sin rellenar
 *  es información para quien tiene que completar el expediente. */
function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className={styles.infoItem}>
      <span>{label}</span>
      <strong>{children ?? <em className={styles.fieldEmpty}>Sin registrar</em>}</strong>
    </div>
  )
}

/**
 * Ficha del profesional: quién es, quién lo supervisa, desde cuándo está y si
 * puede entrar. Vive aquí y no dentro de la pantalla porque el atajo (el modal
 * del listado) enseña exactamente lo mismo; si cada uno tuviera su copia, se
 * irían separando al añadir un campo.
 */
export function EmployeeFicha({ user, managerName, compact = false }: { user: User; managerName?: string; compact?: boolean }) {
  // Del detalle a lo general: la ciudad dice más que el país, pero el país solo
  // también sirve. Location es el campo libre de quien no rellenó los otros.
  const place = [user.city, user.state, user.country].filter(Boolean).join(', ') || user.location?.trim()
  const onboarding = ONBOARDING_STATE[user.onboarding_status || 'not_required']

  return (
    <div className={`${styles.infoGrid} ${compact ? styles.noMargin : ''}`}>
      <Field label="Cargo">{user.job_title?.trim() || null}</Field>
      <Field label="Manager">
        {managerName
          ? managerName
          : user.manager_id
            ? `#${user.manager_id}`
            : user.is_manager
              ? <em className={styles.fieldNote}>Es manager</em>
              : null}
      </Field>
      <Field label="Ubicación">{place || null}</Field>
      <Field label="Teléfono">{user.phone_number?.trim() || null}</Field>
      <Field label="Alta">
        {user.created_at
          ? <>{formatDateOnly(user.created_at)} <em className={styles.fieldNote}>· {timeSince(user.created_at)}</em></>
          : null}
      </Field>
      <Field label="Acceso">
        <em className={`${styles.pill} ${onboarding.pill}`}>{onboarding.label}</em>
        {user.email_verified_at && (
          <em className={styles.fieldOk}>
            <ShieldCheck size={13} /> correo verificado
          </em>
        )}
      </Field>
    </div>
  )
}

/** Los tres números de cabecera del profesional. */
export function EmployeeKpis({ summary, compact = false }: { summary?: EmployeeSummary | null; compact?: boolean }) {
  return (
    <div className={`${styles.kpis} ${compact ? styles.noMargin : ''}`}>
      <div className={styles.kpiCard}>
        <div className={styles.kpiIcon} style={{ background: 'linear-gradient(135deg, var(--primary), var(--primary-dark))' }}>
          <Clock size={24} />
        </div>
        <div>
          <span className={styles.kpiValue}>{summary?.hours_this_month?.toFixed(1) ?? '0.0'} h</span>
          <span className={styles.kpiLabel}>Horas este mes</span>
        </div>
      </div>
      <div className={styles.kpiCard}>
        <div className={styles.kpiIcon} style={{ background: 'linear-gradient(135deg, #8b5cf6, #7c3aed)' }}>
          <CheckSquare size={24} />
        </div>
        <div>
          <span className={styles.kpiValue}>{summary?.tasks_completed ?? 0}/{summary?.tasks_assigned ?? 0}</span>
          <span className={styles.kpiLabel}>Tareas completadas</span>
        </div>
      </div>
      <div className={styles.kpiCard}>
        <div className={styles.kpiIcon} style={{ background: 'linear-gradient(135deg, #10b981, #059669)' }}>
          <CalendarClock size={24} />
        </div>
        <div>
          <span className={styles.kpiValue}>
            {summary?.last_active ? new Date(summary.last_active).toLocaleDateString('es-ES') : '—'}
          </span>
          <span className={styles.kpiLabel}>Última actividad</span>
        </div>
      </div>
    </div>
  )
}
