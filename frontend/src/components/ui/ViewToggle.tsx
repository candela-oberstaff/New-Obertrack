import type { ReactNode } from 'react'
import './ViewToggle.css'

// Conmutador de vista: el control segmentado de "cuadrícula / lista / calendario".
//
// Vivía suelto dentro de Tareas, en marcado y CSS propios. Al necesitarlo también en
// Soporte, se extrae en vez de copiarlo: dos copias del mismo control es como empiezan
// a divergir dos pantallas que deberían sentirse iguales.
//
// Es sólo la presentación. Quién recuerda la vista elegida —y dónde— es de cada
// pantalla: Tareas la guarda por persona en el navegador, y no todas tienen por qué.

export interface ViewOption<T extends string> {
  value: T
  /** Icono del botón. El control es de iconos: el texto va en el título. */
  icon: ReactNode
  /** Qué vista es, para el tooltip y para quien navega con lector de pantalla. */
  label: string
}

interface ViewToggleProps<T extends string> {
  value: T
  onChange: (value: T) => void
  options: ViewOption<T>[]
  /** Nombre del grupo para lectores de pantalla. */
  ariaLabel?: string
  className?: string
}

export function ViewToggle<T extends string>({
  value,
  onChange,
  options,
  ariaLabel = 'Cambiar vista',
  className = '',
}: ViewToggleProps<T>) {
  return (
    <div className={`ui-view-toggle ${className}`} role="group" aria-label={ariaLabel}>
      {options.map((opt) => (
        <button
          key={opt.value}
          type="button"
          className={`ui-view-toggle__btn ${value === opt.value ? 'ui-view-toggle__btn--active' : ''}`}
          onClick={() => onChange(opt.value)}
          title={opt.label}
          aria-label={opt.label}
          aria-pressed={value === opt.value}
        >
          {opt.icon}
        </button>
      ))}
    </div>
  )
}
