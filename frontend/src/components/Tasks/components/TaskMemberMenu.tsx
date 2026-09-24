import { useEffect, useRef, useState } from 'react'
import { Check, ChevronDown, UserSearch } from 'lucide-react'
import styles from './TaskMemberMenu.module.css'

/**
 * Ver el tablero desde la perspectiva de una sola persona.
 *
 * Es el tercer segmento del selector de "quién" —junto a Todas y Mis tareas—
 * porque responde a la misma pregunta. Hasta ahora ese selector era binario:
 * o todo el equipo o uno mismo, así que para saber qué le quedaba a alguien
 * había que recorrer las columnas buscando su avatar a ojo.
 *
 * Cada persona del menú muestra cuántas tareas tiene SIN terminar, que es el
 * número por el que se pregunta al pedir "las pendientes de fulano".
 */

export interface MemberOption {
  id: number
  name: string
  /** Tareas de esa persona sin terminar. */
  pending: number
}

interface TaskMemberMenuProps {
  options: MemberOption[]
  selectedId: number | null
  onSelect: (id: number | null) => void
}

function initials(name: string): string {
  const parts = (name || '').trim().split(/\s+/).filter(Boolean)
  if (parts.length === 0) return '?'
  return (parts[0][0] + (parts[1]?.[0] ?? '')).toUpperCase()
}

export function TaskMemberMenu({ options, selectedId, onSelect }: TaskMemberMenuProps) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    function handleClickOutside(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    function handleEscape(e: KeyboardEvent) {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', handleClickOutside)
    document.addEventListener('keydown', handleEscape)
    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
      document.removeEventListener('keydown', handleEscape)
    }
  }, [open])

  const selected = options.find((o) => o.id === selectedId) ?? null

  return (
    <div className={styles.wrapper} ref={ref}>
      <button
        type="button"
        className={`${styles.trigger} ${selected ? styles.active : ''}`}
        onClick={() => setOpen((v) => !v)}
        aria-haspopup="menu"
        aria-expanded={open}
        title={selected ? `Tareas de ${selected.name}` : 'Ver las tareas de un integrante'}
      >
        <UserSearch size={14} />
        <span className={styles.name}>{selected ? selected.name : 'Por integrante'}</span>
        {selected && <span className={styles.count}>{selected.pending}</span>}
        <ChevronDown size={13} />
      </button>

      {open && (
        <div className={styles.dropdown} role="menu" aria-label="Ver tareas de un integrante">
          <p className={styles.menuTitle}>Pendientes por integrante</p>

          {options.length === 0 && (
            <p className={styles.empty}>Este tablero todavía no tiene integrantes.</p>
          )}

          {options.map((option) => {
            const isSelected = option.id === selectedId
            return (
              <button
                key={option.id}
                type="button"
                role="menuitemradio"
                aria-checked={isSelected}
                className={`${styles.option} ${isSelected ? styles.optionActive : ''}`}
                onClick={() => {
                  // Volver a pulsar a quien ya está elegido quita el filtro: es
                  // la forma más corta de volver a ver todo el tablero.
                  onSelect(isSelected ? null : option.id)
                  setOpen(false)
                }}
              >
                <span className={styles.avatar}>{initials(option.name)}</span>
                <span className={styles.optionName}>{option.name}</span>
                <span
                  className={`${styles.optionCount} ${option.pending === 0 ? styles.optionCountEmpty : ''}`}
                  title={`${option.pending} sin terminar`}
                >
                  {option.pending}
                </span>
                {isSelected && <Check size={14} />}
              </button>
            )
          })}
        </div>
      )}
    </div>
  )
}
