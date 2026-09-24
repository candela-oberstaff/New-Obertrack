import { CircleDashed, Users, User as UserIcon } from 'lucide-react'
import styles from '../../../pages/Tasks.module.css'
import { TaskMemberMenu, type MemberOption } from './TaskMemberMenu'

/**
 * De quién son las tareas que se están mirando: de todo el equipo, mías, o de
 * un integrante concreto. Las tres son la misma pregunta, así que viven en el
 * mismo grupo en vez de repartirse por la cabecera.
 *
 * 'member' no se elige pulsando un botón propio sino escogiendo a alguien en el
 * menú: el estado y la persona van siempre juntos y no pueden contradecirse.
 */
export type TaskScope = 'all' | 'mine' | 'member'

interface TaskScopeToggleProps {
  scope: TaskScope
  onChange: (scope: TaskScope) => void
  allCount: number
  mineCount: number
  /** Integrantes del tablero, con sus tareas sin terminar. */
  memberOptions: MemberOption[]
  selectedMemberId: number | null
  onSelectMember: (id: number | null) => void
  /** Deja fuera lo ya terminado, en cualquiera de los tres ámbitos. */
  onlyPending: boolean
  onOnlyPendingChange: (value: boolean) => void
}

export function TaskScopeToggle({
  scope,
  onChange,
  allCount,
  mineCount,
  memberOptions,
  selectedMemberId,
  onSelectMember,
  onlyPending,
  onOnlyPendingChange,
}: TaskScopeToggleProps) {
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
            onClick={() => {
              // Volver a Todas/Mis tareas suelta al integrante elegido: si no,
              // el menú seguiría mostrando un nombre que ya no filtra nada.
              onSelectMember(null)
              onChange(opt.value)
            }}
            aria-pressed={active}
            title={opt.value === 'mine' ? 'Solo las tareas asignadas a mí' : 'Todas las tareas del tablero'}
          >
            <Icon size={14} />
            {opt.label}
            <span className={styles['scope-toggle-count']}>{opt.count}</span>
          </button>
        )
      })}

      <TaskMemberMenu
        options={memberOptions}
        selectedId={selectedMemberId}
        onSelect={onSelectMember}
      />

      {/* "Pendientes" es lo que de verdad se pide al preguntar por alguien, pero
          aplica igual a los otros dos ámbitos, así que es un interruptor aparte
          y no una cuarta pestaña. */}
      <button
        type="button"
        className={`${styles['scope-toggle-btn']} ${onlyPending ? styles['scope-toggle-btn-active'] : ''}`}
        onClick={() => onOnlyPendingChange(!onlyPending)}
        aria-pressed={onlyPending}
        title="Ocultar las tareas ya terminadas"
      >
        <CircleDashed size={14} />
        Solo pendientes
      </button>
    </div>
  )
}
