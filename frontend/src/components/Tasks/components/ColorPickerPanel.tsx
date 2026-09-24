import { useCallback, useEffect, useRef, useState } from 'react'
import { hexToHsv, hsvToHex, normalizeHex, type HSV } from './colorUtils'
import styles from './BoardColorPicker.module.css'

/**
 * Selector de color a medida, con la cara de la aplicación.
 *
 * Reemplaza al que abría `<input type="color">`, que es el del sistema
 * operativo: cada equipo lo pinta distinto, no se puede estilizar y se abría
 * como una ventana ajena encima del modal.
 *
 * Va EN LÍNEA, empujando el contenido, en vez de flotar sobre él: el cuerpo del
 * modal tiene `overflow-y: auto`, así que un panel absoluto se recortaba al
 * salirse, que es justo lo que se veía.
 */

interface ColorPickerPanelProps {
  value: string
  onChange: (hex: string) => void
}

const FALLBACK: HSV = { h: 0, s: 0, v: 0 }

export function ColorPickerPanel({ value, onChange }: ColorPickerPanelProps) {
  const [hsv, setHsv] = useState<HSV>(() => hexToHsv(value) ?? FALLBACK)
  // Lo que se está escribiendo en el campo hex. Es estado aparte porque
  // "#1a2" a medio teclear todavía no es un color y no debe pisar la selección.
  const [hexDraft, setHexDraft] = useState(() => normalizeHex(value) ?? '#000000')
  const areaRef = useRef<HTMLDivElement>(null)

  // Si el color cambia desde fuera (se pulsa una muestra de la paleta), el
  // panel tiene que seguirlo.
  useEffect(() => {
    const next = hexToHsv(value)
    if (!next) return
    const current = hsvToHex(hsv)
    if (normalizeHex(value) !== current) {
      setHsv(next)
      setHexDraft(normalizeHex(value) ?? current)
    }
    // hsv se lee pero no dispara: solo interesa reaccionar al valor de fuera.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value])

  const apply = useCallback((next: HSV) => {
    setHsv(next)
    const hex = hsvToHex(next)
    setHexDraft(hex)
    onChange(hex)
  }, [onChange])

  // --- Cuadro de saturación (eje X) y brillo (eje Y) ---

  const pickFromPointer = useCallback((clientX: number, clientY: number) => {
    const rect = areaRef.current?.getBoundingClientRect()
    if (!rect || rect.width === 0 || rect.height === 0) return
    const s = Math.min(1, Math.max(0, (clientX - rect.left) / rect.width))
    const v = 1 - Math.min(1, Math.max(0, (clientY - rect.top) / rect.height))
    apply({ ...hsv, s, v })
  }, [apply, hsv])

  const handlePointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
    // Capturar el puntero es lo que permite seguir arrastrando aunque el cursor
    // se salga del cuadro, que es como se comporta cualquier selector de color.
    e.currentTarget.setPointerCapture(e.pointerId)
    pickFromPointer(e.clientX, e.clientY)
  }

  const handlePointerMove = (e: React.PointerEvent<HTMLDivElement>) => {
    if (e.currentTarget.hasPointerCapture(e.pointerId)) {
      pickFromPointer(e.clientX, e.clientY)
    }
  }

  // Sin esto el cuadro solo se puede usar con ratón.
  const handleAreaKeyDown = (e: React.KeyboardEvent<HTMLDivElement>) => {
    const step = e.shiftKey ? 0.1 : 0.02
    const moves: Record<string, Partial<HSV>> = {
      ArrowRight: { s: Math.min(1, hsv.s + step) },
      ArrowLeft: { s: Math.max(0, hsv.s - step) },
      ArrowUp: { v: Math.min(1, hsv.v + step) },
      ArrowDown: { v: Math.max(0, hsv.v - step) },
    }
    const move = moves[e.key]
    if (!move) return
    e.preventDefault()
    apply({ ...hsv, ...move })
  }

  const commitHex = (raw: string) => {
    const normalized = normalizeHex(raw)
    if (!normalized) return false
    const next = hexToHsv(normalized)
    if (!next) return false
    setHsv(next)
    setHexDraft(normalized)
    onChange(normalized)
    return true
  }

  const handleHexChange = (raw: string) => {
    setHexDraft(raw)
    // Mientras se teclea solo vale la forma larga. Con el atajo de tres
    // dígitos, escribir "#10b981" pasaba por "#10b" —que SÍ es un color
    // válido, el #1100bb— y el tablero parpadeaba en un color que nadie pidió
    // camino al que sí. El atajo se sigue aceptando, pero al salir del campo.
    if (/^#?[0-9a-fA-F]{6}$/.test(raw.trim())) {
      commitHex(raw)
    }
  }

  // Al salir del campo se acepta el atajo de tres dígitos; un hex a medio
  // escribir vuelve al color vigente en vez de quedarse como texto inválido.
  const handleHexBlur = () => {
    if (!commitHex(hexDraft)) setHexDraft(hsvToHex(hsv))
  }

  const hueColor = hsvToHex({ h: hsv.h, s: 1, v: 1 })
  const selected = hsvToHex(hsv)

  return (
    <div className={styles.panel}>
      <div
        ref={areaRef}
        className={styles.area}
        style={{ backgroundColor: hueColor }}
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onKeyDown={handleAreaKeyDown}
        role="slider"
        tabIndex={0}
        aria-label="Saturación y brillo"
        aria-valuetext={`Saturación ${Math.round(hsv.s * 100)}%, brillo ${Math.round(hsv.v * 100)}%`}
        aria-valuenow={Math.round(hsv.s * 100)}
        aria-valuemin={0}
        aria-valuemax={100}
      >
        <span
          className={styles.areaThumb}
          style={{
            left: `${hsv.s * 100}%`,
            top: `${(1 - hsv.v) * 100}%`,
            backgroundColor: selected,
          }}
        />
      </div>

      <div className={styles.controls}>
        <span className={styles.preview} style={{ backgroundColor: selected }} aria-hidden="true" />

        <div className={styles.sliders}>
          <input
            type="range"
            className={styles.hue}
            min={0}
            max={360}
            value={Math.round(hsv.h)}
            onChange={(e) => apply({ ...hsv, h: Number(e.target.value) })}
            aria-label="Tono"
          />

          <div className={styles.hexRow}>
            <label className={styles.hexLabel} htmlFor="color-hex-input">Hex</label>
            <input
              id="color-hex-input"
              type="text"
              className={styles.hexInput}
              value={hexDraft}
              onChange={(e) => handleHexChange(e.target.value)}
              onBlur={handleHexBlur}
              spellCheck={false}
              maxLength={7}
              aria-label="Código hexadecimal del color"
            />
          </div>
        </div>
      </div>
    </div>
  )
}
