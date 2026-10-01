import { useEffect, useRef, useState } from 'react'
import { ArrowUpDown, Check, LayoutGrid } from 'lucide-react'
import { BOARD_SORT_OPTIONS, DEFAULT_BOARD_SORT, type BoardSortKey } from './boardSort'
import styles from './BoardSortMenu.module.css'

/**
 * Elige el orden de la lista de tableros.
 *
 * Se usa en los dos sitios donde esa lista aparece —el desplegable de la
 * cabecera y la cuadrícula de "Selecciona un tablero"—, y ambos leen el mismo
 * estado: cambiar el orden en uno y encontrar el otro como estaba sería
 * desconcertante.
 */

interface BoardSortMenuProps {
  value: BoardSortKey
  onChange: (key: BoardSortKey) => void
  /** Muestra el nombre del criterio junto al icono (en la cuadrícula hay sitio). */
  showLabel?: boolean
  /** Alinea el menú a la derecha cuando el botón queda pegado al borde. */
  align?: 'left' | 'right'
}

export function BoardSortMenu({
  value,
  onChange,
  showLabel = false,
  align = 'left',
}: BoardSortMenuProps) {
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

  const current = BOARD_SORT_OPTIONS.find((o) => o.value === value) ?? BOARD_SORT_OPTIONS[0]
  const isDefault = value === DEFAULT_BOARD_SORT

  return (
    <div className={styles.wrapper} ref={ref}>
      <button
        type="button"
        className={[
          styles.toggle,
          showLabel ? styles.withLabel : '',
          isDefault ? '' : styles.active,
        ].filter(Boolean).join(' ')}
        onClick={() => setOpen((v) => !v)}
        title={`Ordenar tableros: ${current.label}`}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label="Ordenar tableros"
      >
        {/* El icono de tablero acompaña a las flechas: en una cabecera llena de
            controles de tarea, unas flechas solas no dicen qué se ordena. */}
        <LayoutGrid size={14} />
        <ArrowUpDown size={13} />
        {showLabel && <span className={styles.label}>{current.label}</span>}
      </button>

      {open && (
        <div
          className={`${styles.dropdown} ${align === 'right' ? styles.dropdownRight : ''}`}
          role="menu"
          aria-label="Ordenar tableros"
        >
          <p className={styles.menuTitle}>Ordenar tableros</p>
          {BOARD_SORT_OPTIONS.map((option) => {
            const selected = option.value === value
            return (
              <button
                key={option.value}
                type="button"
                role="menuitemradio"
                aria-checked={selected}
                className={`${styles.option} ${selected ? styles.optionActive : ''}`}
                onClick={() => {
                  onChange(option.value)
                  setOpen(false)
                }}
              >
                {option.label}
                {selected && <Check size={15} />}
              </button>
            )
          })}
        </div>
      )}
    </div>
  )
}
