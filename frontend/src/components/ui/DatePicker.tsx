import { useState, useRef, useEffect, useCallback, useMemo } from 'react'
import { createPortal } from 'react-dom'
import { Calendar, ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight } from 'lucide-react'
import './DatePicker.css'

export interface DatePickerProps {
  /** Valor en AAAA-MM-DD (lo que hablaba el <input type="date"> nativo). '' = vacío. */
  value: string
  onChange: (value: string) => void
  /** Límites inclusivos, también en AAAA-MM-DD. */
  min?: string
  max?: string
  placeholder?: string
  /** Añade el botón "Borrar" al pie del calendario. */
  clearable?: boolean
  disabled?: boolean
  required?: boolean
  name?: string
  id?: string
  className?: string
  fullWidth?: boolean
  /** Versión estrecha para ponerlo dentro de una fila o una barra de filtros. */
  compact?: boolean
  ariaLabel?: string
  title?: string
}

const pad = (n: number) => String(n).padStart(2, '0')

/** Fecha -> AAAA-MM-DD. A mano y no con toISOString(), que convierte a UTC y
 *  devuelve el día anterior en cualquier huso al oeste de Greenwich. */
export function toISODate(d: Date): string {
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

/** AAAA-MM-DD -> Date local (mediodía no hace falta: se construye por partes).
 *  Rechaza lo que no existe: 2026-02-31 desbordaría a marzo en vez de fallar. */
export function fromISODate(s?: string | null): Date | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(s ?? '')
  if (!m) return null
  const [y, mo, da] = [Number(m[1]), Number(m[2]), Number(m[3])]
  const d = new Date(y, mo - 1, da)
  if (d.getFullYear() !== y || d.getMonth() !== mo - 1 || d.getDate() !== da) return null
  return d
}

/** Lo que se puede teclear: 13/08/2026, 13-8-2026 o 2026-08-13. El año de dos
 *  cifras no se adivina — "20" podría ser 1920 o 2020 y en fechas de ingreso el
 *  error pasa desapercibido. */
function parseTyped(raw: string): string | null {
  const s = raw.trim()
  if (!s) return ''
  let m = /^(\d{4})[-/](\d{1,2})[-/](\d{1,2})$/.exec(s)
  if (m) return fromISODate(`${m[1]}-${pad(Number(m[2]))}-${pad(Number(m[3]))}`) ? `${m[1]}-${pad(Number(m[2]))}-${pad(Number(m[3]))}` : null
  m = /^(\d{1,2})[-/.](\d{1,2})[-/.](\d{4})$/.exec(s)
  if (m) {
    const iso = `${m[3]}-${pad(Number(m[2]))}-${pad(Number(m[1]))}`
    return fromISODate(iso) ? iso : null
  }
  return null
}

/** AAAA-MM-DD -> 13/08/2026, que es como se leen las fechas en el resto de la app. */
function formatDisplay(iso: string): string {
  const d = fromISODate(iso)
  return d ? `${pad(d.getDate())}/${pad(d.getMonth() + 1)}/${d.getFullYear()}` : ''
}

const WEEKDAYS = ['L', 'M', 'X', 'J', 'V', 'S', 'D']

/** Lunes primero, como en los calendarios de aquí (getDay() pone el domingo a 0). */
function mondayIndex(d: Date): number {
  return (d.getDay() + 6) % 7
}

/** Las 6 semanas que se pintan de un mes, con los días de relleno de los meses
 *  vecinos: una rejilla de alto fijo no da saltos al cambiar de mes. */
function buildGrid(year: number, month: number): Date[] {
  const first = new Date(year, month, 1)
  const start = new Date(year, month, 1 - mondayIndex(first))
  return Array.from({ length: 42 }, (_, i) => new Date(start.getFullYear(), start.getMonth(), start.getDate() + i))
}

function monthLabel(year: number, month: number): string {
  const raw = new Date(year, month, 1).toLocaleDateString('es-ES', { month: 'long', year: 'numeric' })
  return raw.charAt(0).toUpperCase() + raw.slice(1)
}

/**
 * Selector de fecha propio, hermano del <Select>: mismo disparador, mismo
 * portal y mismo cierre por clic fuera o Escape.
 *
 * Se teclea Y se elige con el ratón a propósito. El calendario solo sirve para
 * "el jueves que viene"; quien carga jornadas todos los días o corrige un
 * ingreso de 2019 escribe la fecha, y obligarle a navegar meses sería más lento
 * que el <input type="date"> al que sustituye.
 */
export function DatePicker({
  value,
  onChange,
  min,
  max,
  placeholder = 'dd/mm/aaaa',
  clearable = false,
  disabled = false,
  required = false,
  name,
  id,
  className = '',
  fullWidth = false,
  compact = false,
  ariaLabel,
  title,
}: DatePickerProps) {
  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)
  const popRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const [pos, setPos] = useState<{ top?: number; bottom?: number; left: number } | null>(null)

  // Lo tecleado mientras se escribe. Null = el campo enseña `value`; solo existe
  // entre la primera tecla y el commit, para no pelearse con quien escribe.
  const [draft, setDraft] = useState<string | null>(null)

  const selected = fromISODate(value)
  const minDate = fromISODate(min)
  const maxDate = fromISODate(max)

  // Mes visible. Arranca en el de la fecha elegida y, si no hay, en el de hoy
  // recortado a los límites: con max="hoy" abrir en un mes entero deshabilitado
  // sería desconcertante.
  const initialMonth = useMemo(() => {
    const base = selected ?? (maxDate && maxDate < new Date() ? maxDate : minDate && minDate > new Date() ? minDate : new Date())
    return { year: base.getFullYear(), month: base.getMonth() }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value])
  const [view, setView] = useState(initialMonth)
  useEffect(() => { if (open) setView(initialMonth) }, [open, initialMonth])

  const isOutOfRange = useCallback((d: Date) => {
    if (minDate && toISODate(d) < toISODate(minDate)) return true
    if (maxDate && toISODate(d) > toISODate(maxDate)) return true
    return false
  }, [minDate, maxDate])

  const updatePosition = useCallback(() => {
    if (!containerRef.current) return
    const r = containerRef.current.getBoundingClientRect()
    // Más aire que el menú del Select (6px) a propósito: el calendario es una
    // superficie grande y pegada al campo se leía como parte de él.
    const GAP = 10
    const MARGIN = 8
    const PANEL = 330 // alto real del calendario; con esto decide arriba/abajo
    const below = window.innerHeight - r.bottom - GAP - MARGIN
    const above = r.top - GAP - MARGIN
    if (below < PANEL && above > below) {
      setPos({ bottom: window.innerHeight - r.top + GAP, left: r.left })
    } else {
      setPos({ top: r.bottom + GAP, left: r.left })
    }
  }, [])

  useEffect(() => {
    if (!open) return
    updatePosition()
    const onReflow = () => updatePosition()
    window.addEventListener('scroll', onReflow, true)
    window.addEventListener('resize', onReflow)
    return () => {
      window.removeEventListener('scroll', onReflow, true)
      window.removeEventListener('resize', onReflow)
    }
  }, [open, updatePosition])

  useEffect(() => {
    if (!open) return
    const handleClick = (e: MouseEvent) => {
      const t = e.target as Node
      if (containerRef.current?.contains(t) || popRef.current?.contains(t)) return
      setOpen(false)
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [open])

  useEffect(() => {
    if (!open) return
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') { setOpen(false); inputRef.current?.focus() }
    }
    document.addEventListener('keydown', handleKey)
    return () => document.removeEventListener('keydown', handleKey)
  }, [open])

  // Confirma lo tecleado. Lo que no se entiende no se guarda ni se queda a medias
  // en pantalla: vuelve a la última fecha válida, que es lo que sigue guardado.
  const commitDraft = useCallback(() => {
    if (draft === null) return
    const parsed = parseTyped(draft)
    setDraft(null)
    if (parsed === null) return
    if (parsed === '') { if (value !== '') onChange(''); return }
    const d = fromISODate(parsed)
    if (!d || isOutOfRange(d)) return
    if (parsed !== value) onChange(parsed)
  }, [draft, value, onChange, isOutOfRange])

  const pick = useCallback((d: Date) => {
    if (isOutOfRange(d)) return
    setDraft(null)
    onChange(toISODate(d))
    setOpen(false)
    inputRef.current?.focus()
  }, [isOutOfRange, onChange])

  const shiftMonth = (delta: number) => setView(v => {
    const d = new Date(v.year, v.month + delta, 1)
    return { year: d.getFullYear(), month: d.getMonth() }
  })

  const todayISO = toISODate(new Date())
  const grid = buildGrid(view.year, view.month)

  return (
    <div
      ref={containerRef}
      className={`ui-datepicker ${fullWidth ? 'ui-datepicker--full' : ''} ${compact ? 'ui-datepicker--compact' : ''} ${disabled ? 'ui-datepicker--disabled' : ''} ${className}`}
    >
      <input
        ref={inputRef}
        id={id}
        name={name}
        type="text"
        inputMode="numeric"
        autoComplete="off"
        className="ui-datepicker__input"
        value={draft ?? formatDisplay(value)}
        placeholder={placeholder}
        disabled={disabled}
        required={required}
        aria-label={ariaLabel}
        title={title}
        onChange={e => setDraft(e.target.value)}
        onBlur={commitDraft}
        onKeyDown={e => {
          if (e.key === 'Enter') { e.preventDefault(); commitDraft(); setOpen(false) }
          if (e.key === 'ArrowDown' && !open) { e.preventDefault(); updatePosition(); setOpen(true) }
        }}
      />
      <button
        type="button"
        className="ui-datepicker__toggle"
        disabled={disabled}
        aria-haspopup="dialog"
        aria-expanded={open}
        aria-label="Abrir calendario"
        tabIndex={-1}
        onClick={() => {
          if (disabled) return
          commitDraft()
          if (!open) updatePosition()
          setOpen(v => !v)
        }}
      >
        <Calendar size={16} />
      </button>

      {open && pos && createPortal(
        <div
          ref={popRef}
          className="ui-datepicker__panel"
          role="dialog"
          aria-label="Calendario"
          style={{ position: 'fixed', top: pos.top ?? 'auto', bottom: pos.bottom ?? 'auto', left: pos.left, zIndex: 9999 }}
        >
          {/* Doble flecha = año. Sin ella, corregir un ingreso de 2019 son
              ochenta clics de mes. */}
          <div className="ui-datepicker__header">
            <button type="button" className="ui-datepicker__nav" onClick={() => shiftMonth(-12)} aria-label="Año anterior"><ChevronsLeft size={16} /></button>
            <button type="button" className="ui-datepicker__nav" onClick={() => shiftMonth(-1)} aria-label="Mes anterior"><ChevronLeft size={16} /></button>
            <span className="ui-datepicker__month" aria-live="polite">{monthLabel(view.year, view.month)}</span>
            <button type="button" className="ui-datepicker__nav" onClick={() => shiftMonth(1)} aria-label="Mes siguiente"><ChevronRight size={16} /></button>
            <button type="button" className="ui-datepicker__nav" onClick={() => shiftMonth(12)} aria-label="Año siguiente"><ChevronsRight size={16} /></button>
          </div>

          <div className="ui-datepicker__weekdays">
            {WEEKDAYS.map((w, i) => <span key={i}>{w}</span>)}
          </div>

          <div className="ui-datepicker__grid">
            {grid.map(d => {
              const iso = toISODate(d)
              const outside = d.getMonth() !== view.month
              const blocked = isOutOfRange(d)
              return (
                <button
                  key={iso}
                  type="button"
                  className={[
                    'ui-datepicker__day',
                    outside ? 'ui-datepicker__day--outside' : '',
                    iso === value ? 'ui-datepicker__day--selected' : '',
                    iso === todayISO ? 'ui-datepicker__day--today' : '',
                  ].filter(Boolean).join(' ')}
                  disabled={blocked}
                  // El número solo se repite dentro de la rejilla (el 9 de este
                  // mes y el del vecino): quien navega a ciegas necesita la
                  // fecha entera para saber a cuál está a punto de saltar.
                  aria-label={d.toLocaleDateString('es-ES', { day: 'numeric', month: 'long', year: 'numeric' })}
                  aria-current={iso === todayISO ? 'date' : undefined}
                  aria-pressed={iso === value}
                  onClick={() => pick(d)}
                >
                  {d.getDate()}
                </button>
              )
            })}
          </div>

          <div className="ui-datepicker__footer">
            {clearable ? (
              <button type="button" className="ui-datepicker__action" onClick={() => { setDraft(null); onChange(''); setOpen(false) }}>
                Borrar
              </button>
            ) : <span />}
            <button
              type="button"
              className="ui-datepicker__action ui-datepicker__action--primary"
              disabled={isOutOfRange(new Date())}
              onClick={() => pick(new Date())}
            >
              Hoy
            </button>
          </div>
        </div>,
        document.body,
      )}
    </div>
  )
}
