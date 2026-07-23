import { Users, User as UserIcon } from 'lucide-react'
import styles from '../../../pages/Tasks.module.css'

export type TaskScope = 'all' | 'mine'

interface TaskScopeToggleProps {
  scope: TaskScope
  onChange: (scope: TaskScope) => void
  allCount: number
  mineCount: number
}

export function TaskScopeToggle({ scope, onChange, allCount, mineCount }: TaskScopeToggleProps) {
  const options: { value: TaskScope; label: string; count: number; icon: typeof Users }[] = [
    { value: 'all', label: 'Todas', count: allCount, icon: Users },
    { value: 'mine', label: 'Mis tareas', count: mineCount, icon: UserIcon },
  ]

  return (
    <div className={styles['scope-toggle']} role="group" aria-label="Ver tareas" data-tour="tasks-scope-toggle">
      {options.map((opt) => {
        const Icon = opt.icon
        const active = scope === opt.value
        return (
          <button
            key={opt.value}
            type="button"
            className={`${styles['scope-toggle-btn']} ${active ? styles['scope-toggle-btn-active'] : ''}`}
            onClick={() => onChange(opt.value)}
            aria-pressed={active}
            title={opt.value === 'mine' ? 'Solo las tareas asignadas a mí' : 'Todas las tareas del tablero'}
          >
            <Icon size={14} />
            {opt.label}
            <span className={styles['scope-toggle-count']}>{opt.count}</span>
          </button>
        )
      })}
    </div>
  )
}
