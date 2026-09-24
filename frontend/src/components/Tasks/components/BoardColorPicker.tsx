import { useState } from 'react'
import { Check, Pipette } from 'lucide-react'
import { ColorPickerPanel } from './ColorPickerPanel'
import { BOARD_COLORS, isLightColor, normalizeHex } from './colorUtils'
import styles from './BoardColorPicker.module.css'

/**
 * Selector del color de un tablero.
 *
 * El color ya se usaba en toda la pantalla —el punto del selector, la franja de
 * cada tarjeta—, pero elegirlo era un `<input type="color">` sin estilo: un
 * recuadro gris que no parecía un control y que, al pulsarlo, abría la ventana
 * de color del sistema operativo. Aquí la paleta está a la vista y "otro color"
 * despliega un panel propio (ColorPickerPanel), no el del navegador.
 */

interface BoardColorPickerProps {
  value: string
  onChange: (color: string) => void
  /** Para enlazar la etiqueta del formulario con el grupo. */
  ariaLabel?: string
}

export function BoardColorPicker({ value, onChange, ariaLabel = 'Color del tablero' }: BoardColorPickerProps) {
  const normalized = normalizeHex(value) ?? (value || '').toLowerCase()
  // Un color que no está en la paleta es uno a medida: el gotero se marca como
  // elegido en lugar de dejar la selección sin dueño.
  const isCustom = !!normalized && !BOARD_COLORS.some((c) => c.toLowerCase() === normalized)
  const [open, setOpen] = useState(isCustom)

  return (
    <>
      <div className={styles.picker} role="group" aria-label={ariaLabel}>
        {BOARD_COLORS.map((color) => {
          const selected = normalized === color.toLowerCase()
          return (
            <button
              key={color}
              type="button"
              className={`${styles.swatch} ${selected ? styles.selected : ''}`}
              style={{ backgroundColor: color }}
              onClick={() => onChange(color)}
              aria-label={`Color ${color}`}
              aria-pressed={selected}
              title={color}
            >
              {selected && (
                // Sobre un tono claro un tic blanco no se ve; sobre uno oscuro,
                // uno negro tampoco.
                <span className={styles.check} style={isLightColor(color) ? { color: '#060b23' } : undefined}>
                  <Check size={16} strokeWidth={3} />
                </span>
              )}
            </button>
          )
        })}

        <button
          type="button"
          className={`${styles.custom} ${isCustom ? styles.customActive : ''}`}
          style={isCustom ? { backgroundColor: value } : undefined}
          onClick={() => setOpen((v) => !v)}
          aria-expanded={open}
          aria-label="Elegir otro color"
          title="Elegir otro color"
        >
          <Pipette size={15} />
        </button>

        <span className={styles.value}>{normalized || value}</span>
      </div>

      {open && <ColorPickerPanel value={normalized || BOARD_COLORS[0]} onChange={onChange} />}
    </>
  )
}
