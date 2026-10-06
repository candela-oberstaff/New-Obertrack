import { AlertTriangle, CheckCircle2, Lightbulb, XCircle } from 'lucide-react'

import { isReady, type ReadinessIssue } from './inductionReadiness'
import styles from './InductionSettings.module.css'

const ICON = { blocker: XCircle, warning: AlertTriangle, tip: Lightbulb }

/**
 * Insignia corta: «Listo» o el problema por su nombre («Mínimo bajo (19%)»).
 * Con varios, el primero (los bloqueos van antes) y cuántos más; el detalle
 * completo queda en el título al pasar el ratón.
 */
export function ReadinessBadge({ issues }: { issues: ReadinessIssue[] }) {
  const pending = issues.filter((i) => i.level !== 'tip')
  if (pending.length === 0) {
    return (
      <span className={styles.tagOk}>
        <CheckCircle2 size={12} /> Listo
      </span>
    )
  }
  const first = pending.find((i) => i.level === 'blocker') ?? pending[0]
  return (
    <span
      className={first.level === 'blocker' ? styles.tagDanger : styles.tagWarn}
      title={pending.map((i) => `• ${i.text} ${i.fix}`).join('\n')}
    >
      <AlertTriangle size={12} /> {first.short}
      {pending.length > 1 && ` y ${pending.length - 1} más`}
    </span>
  )
}

/** Lista de lo que falta, para el editor. Sin pendientes muestra el «todo listo». */
export default function ReadinessChecklist({ issues, readyText }: { issues: ReadinessIssue[]; readyText: string }) {
  return (
    <div className={isReady(issues) ? styles.readyBox : styles.readinessBox}>
      {isReady(issues) && (
        <div className={styles.readinessItem} data-level="ok">
          <CheckCircle2 size={15} /> <span>{readyText}</span>
        </div>
      )}
      {issues.map((issue) => {
        const Icon = ICON[issue.level]
        return (
          <div key={issue.text} className={styles.readinessItem} data-level={issue.level}>
            <Icon size={15} />
            <span>
              {issue.text} <span className={styles.readinessFix}>{issue.fix}</span>
            </span>
          </div>
        )
      })}
    </div>
  )
}
