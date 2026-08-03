import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ClipboardList, Send } from 'lucide-react'
import { adminService } from '../../services/api'
import { InductionStatusPanel } from '../../components/Admin/InductionStatusPanel'
import { Button, Select, Skeleton } from '../../components/ui'
import { useNotification } from '../../context/NotificationContext'
import styles from './Tenants.module.css'

// Tipos de gestión que se registran a mano desde la ficha. 'emergencia' existe
// en el backend pero la crea el módulo de incidencias al avisar a alguien:
// ofrecerla aquí produciría emergencias sin incidencia detrás.
const KINDS = [
  { value: 'inactivity', label: 'Inactividad' },
  { value: 'absence', label: 'Ausencias' },
]

const STATUSES = [
  { value: 'contacted', label: 'Contactado' },
  { value: 'justified', label: 'Justificado' },
  { value: 'escalated', label: 'Escalado' },
]

const KIND_LABEL: Record<string, string> = {
  inactivity: 'Inactividad',
  absence: 'Ausencias',
  emergencia: 'Emergencia',
}

const STATUS_PILL: Record<string, { label: string; className: string }> = {
  contacted: { label: 'Contactado', className: styles.pillInfo },
  justified: { label: 'Justificado', className: styles.pillSuccess },
  escalated: { label: 'Escalado', className: styles.pillWarn },
}

interface Props {
  userId: number
  /** Empleo del profesional en esta empresa; sin él no hay bitácora que leer. */
  employmentId?: number
  isProfessional: boolean
  canManage: boolean
}

/**
 * Seguimiento de un profesional: en qué punto está su inducción y qué ha hecho
 * el equipo con él (la bitácora de gestión). Van juntos porque responden a la
 * misma pregunta antes de escribirle: ¿ya puede entrar, y ya hablamos con él?
 */
export function EmployeeSeguimiento({ userId, employmentId, isProfessional, canManage }: Props) {
  const qc = useQueryClient()
  const { success, error: showError } = useNotification()

  // Las gestiones viajan dentro del expediente: es la misma bitácora, y pedirla
  // aparte sería una segunda consulta para los mismos registros.
  const { data: expediente, isLoading } = useQuery({
    queryKey: ['expediente', userId, employmentId],
    queryFn: () => adminService.getExpediente(userId, employmentId!),
    enabled: !!employmentId,
  })
  const gestiones: any[] = expediente?.gestiones ?? []

  const [kind, setKind] = useState('inactivity')
  const [status, setStatus] = useState('contacted')
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setBusy(true)
    try {
      await adminService.createFollowUp({
        user_id: userId,
        kind: kind as 'inactivity' | 'absence',
        status,
        note: note.trim(),
      })
      setNote('')
      // La gestión nueva entra en el expediente, así que la pestaña de al lado
      // también queda obsoleta.
      qc.invalidateQueries({ queryKey: ['expediente', userId, employmentId] })
      success('Gestión registrada')
    } catch (err: any) {
      showError(err?.response?.data?.error ?? 'No se pudo registrar la gestión')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className={styles.stackSections}>
      <InductionStatusPanel userId={userId} canReset={canManage} isProfessional={isProfessional} />

      {canManage && (
        <form onSubmit={submit} className={styles.infoCard} style={{ marginBottom: 0 }}>
          <h2>Registrar gestión</h2>
          <div className={styles.formRow}>
            <div className={styles.formField}>
              <span className={styles.formLabel}>Motivo</span>
              <Select options={KINDS} value={kind} onChange={v => setKind(String(v))} ariaLabel="Motivo de la gestión" />
            </div>
            <div className={styles.formField}>
              <span className={styles.formLabel}>Resultado</span>
              <Select options={STATUSES} value={status} onChange={v => setStatus(String(v))} ariaLabel="Resultado de la gestión" />
            </div>
          </div>
          <textarea
            className={styles.textarea}
            value={note}
            onChange={e => setNote(e.target.value)}
            placeholder="Qué se habló, qué quedó pendiente…"
            rows={3}
            maxLength={2000}
          />
          <div className={styles.formActions}>
            <Button type="submit" disabled={busy}>
              <Send size={15} /> {busy ? 'Registrando…' : 'Registrar'}
            </Button>
          </div>
        </form>
      )}

      <section>
        <h2 className={styles.sectionTitle}>
          Historial de gestiones
          {gestiones.length > 0 && <span className={styles.sectionCount}>({gestiones.length})</span>}
        </h2>
        {!employmentId ? (
          <div className={styles.empty}>
            <ClipboardList size={40} />
            <p>Sin empleo registrado en esta empresa</p>
            <span className={styles.emptyHint}>
              La bitácora de gestión cuelga del empleo. Este profesional no tiene uno aquí,
              así que no hay historial que enseñar.
            </span>
          </div>
        ) : isLoading ? (
          <div className={styles.stack}>
            {Array.from({ length: 3 }).map((_, i) => <Skeleton key={i} height={56} radius={12} />)}
          </div>
        ) : gestiones.length === 0 ? (
          <div className={styles.empty}>
            <ClipboardList size={40} />
            <p>Sin gestiones registradas</p>
            <span className={styles.emptyHint}>
              Aquí queda constancia de cada vez que el equipo contacta a esta persona
              por inactividad o por sus ausencias.
            </span>
          </div>
        ) : (
          <div className={styles.stack}>
            {gestiones.map((g, i) => {
              const pill = STATUS_PILL[g.status] ?? { label: g.status, className: styles.pillNeutral }
              const when = new Date(g.created_at)
              return (
                <div key={i} className={styles.entry}>
                  <div className={styles.entryHead}>
                    <span className={styles.entryTitle}>{KIND_LABEL[g.kind] || g.kind}</span>
                    <span className={`${styles.pill} ${pill.className}`}>{pill.label}</span>
                    <span className={styles.entryMeta}>
                      {g.by_name || 'Sistema'} · {isNaN(when.getTime()) ? '—' : when.toLocaleDateString('es-ES')}
                    </span>
                  </div>
                  {g.note && <p className={styles.entryBody}>{g.note}</p>}
                </div>
              )
            })}
          </div>
        )}
      </section>
    </div>
  )
}
