import { AlertTriangle, CheckCircle2, Lightbulb, XCircle } from 'lucide-react'

import { isReady, type ReadinessIssue } from './inductionReadiness'
import styles from './InductionSettings.module.css'

const ICON = { blocker: XCircle, warning: AlertTriangle, tip: Lightbulb }

/** Insignia corta: «Listo» o cuántas cosas faltan. */
export function ReadinessBadge({ issues }: { issues: ReadinessIssue[] }) {
  const pending = issues.filter((i) => i.level !== 'tip').length
  if (pending === 0) {
    return (
      <span className={styles.tagOk}>
        <CheckCircle2 size={12} /> Listo
      </span>
    )
  }
  const blocked = issues.some((i) => i.level === 'blocker')
  return (
    <span className={blocked ? styles.tagDanger : styles.tagWarn}>
      <AlertTriangle size={12} /> {pending === 1 ? 'Falta 1 cosa' : `Faltan ${pending} cosas`}
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
            <Icon size={15} /> <span>{issue.text}</span>
          </div>
        )
      })}
    </div>
  )
}
