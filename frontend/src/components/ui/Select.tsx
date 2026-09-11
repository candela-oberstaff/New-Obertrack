import { useState, useRef, useEffect, useCallback, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { ChevronDown, Check } from 'lucide-react'
import './Select.css'

export type SelectValue = string | number

export interface SelectOption {
  value: SelectValue
  label: string
  color?: string
  disabled?: boolean
}

export interface SelectProps {
  options: SelectOption[]
  value: SelectValue | SelectValue[] | null | undefined
  onChange: (value: any) => void
  placeholder?: string
  /** When true, the placeholder becomes a selectable option that returns '' */
  clearable?: boolean
  disabled?: boolean
  required?: boolean
  name?: string
  id?: string
  leftIcon?: ReactNode
  /** Extra class applied to the trigger button */
  className?: string
  fullWidth?: boolean
  ariaLabel?: string
  searchable?: boolean
  multiple?: boolean
}

export function Select({
  options,
  value,
  onChange,
  placeholder = 'Seleccionar...',
  clearable = false,
  disabled = false,
  required = false,
  name,
  id,
  leftIcon,
  className = '',
  fullWidth = false,
  ariaLabel,
  searchable = false,
  multiple = false,
}: SelectProps) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const searchRef = useRef<HTMLInputElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  // El menú se renderiza en un portal (position: fixed) para no quedar recortado
  // por contenedores con overflow (p. ej. el cuerpo de un Modal).
  const menuRef = useRef<HTMLDivElement>(null)
  const [menuPos, setMenuPos] = useState<{ top?: number; bottom?: number; left: number; width: number; maxHeight: number } | null>(null)

  const selectedValues: SelectValue[] = multiple
    ? (Array.isArray(value) ? value : (value != null && value !== '' ? [value] : []))
    : []
  const selected = !multiple ? options.find((o) => String(o.value) === String(value ?? '')) : null
  const selectedList = multiple ? options.filter((o) => selectedValues.some((v) => String(v) === String(o.value))) : []

  const showSearch = searchable || options.length > 5
  const visibleOptions = showSearch && query.trim()
    ? options.filter((o) => o.label.toLowerCase().includes(query.trim().toLowerCase()))
    : options

  const updatePosition = useCallback(() => {
    if (!containerRef.current) return
    const r = containerRef.current.getBoundingClientRect()
    const GAP = 6
    const MARGIN = 8
    const MENU_MAX = 280
    const below = window.innerHeight - r.bottom - GAP - MARGIN
    const above = r.top - GAP - MARGIN
    // Si abajo no cabe un menú razonable y arriba hay más espacio, abre hacia
    // arriba (p. ej. triggers en el footer de un Modal). En ambos casos la
    // altura se limita al espacio real para que el menú nunca salga del viewport.
    if (below < Math.min(MENU_MAX, 160) && above > below) {
      setMenuPos({
        bottom: window.innerHeight - r.top + GAP,
        left: r.left,
        width: r.width,
        maxHeight: Math.max(120, Math.min(MENU_MAX, above)),
      })
    } else {
      setMenuPos({
        top: r.bottom + GAP,
        left: r.left,
        width: r.width,
        maxHeight: Math.max(120, Math.min(MENU_MAX, below)),
      })
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
    if (open && showSearch) {
      const raf = requestAnimationFrame(() => searchRef.current?.focus())
      return () => cancelAnimationFrame(raf)
    }
    if (!open) setQuery('')
  }, [open, showSearch])

  // Close on outside click
  useEffect(() => {
    if (!open) return
    const handleClick = (e: MouseEvent) => {
      const t = e.target as Node
      if (containerRef.current?.contains(t) || menuRef.current?.contains(t)) return
      setOpen(false)
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [open])

  // Close on Escape
  useEffect(() => {
    if (!open) return
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('keydown', handleKey)
    return () => document.removeEventListener('keydown', handleKey)
  }, [open])

  const handleSelect = useCallback((val: SelectValue, isDisabled?: boolean) => {
    if (isDisabled) return
    if (multiple) {
      if (val === '') {
        onChange([])
        return
      }
      const exists = selectedValues.some((v) => String(v) === String(val))
      const next = exists
        ? selectedValues.filter((v) => String(v) !== String(val))
        : [...selectedValues, val]
      onChange(next)
    } else {
      onChange(val)
      setOpen(false)
    }
  }, [multiple, selectedValues, onChange])

  return (
    <div
      className={`ui-select ${fullWidth ? 'ui-select--full' : ''}`}
      ref={containerRef}
    >
      <button
        type="button"
        id={id}
        className={`ui-select__trigger ${open ? 'ui-select__trigger--open' : ''} ${disabled ? 'ui-select__trigger--disabled' : ''} ${className}`}
        onClick={() => {
          if (disabled) return
          if (!open) updatePosition()
          setOpen((v) => !v)
        }}
        disabled={disabled}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-label={ariaLabel}
      >
        <span className="ui-select__value">
          {leftIcon && <span className="ui-select__icon">{leftIcon}</span>}
          {multiple ? (
            selectedList.length === 0 ? (
              <span className="ui-select__placeholder">{placeholder}</span>
            ) : selectedList.length === 1 ? (
              <>
                {selectedList[0].color && (
                  <span className="ui-select__dot" style={{ backgroundColor: selectedList[0].color }} />
                )}
                <span className="ui-select__label">{selectedList[0].label}</span>
              </>
            ) : (
              <span className="ui-select__label" style={{ display: 'inline-flex', alignItems: 'center', gap: 6, minWidth: 0 }}>
                <span style={{
                  background: 'var(--primary-soft, #f5f2fb)',
                  color: 'var(--primary, #cc33cc)',
                  padding: '2px 8px',
                  borderRadius: 999,
                  fontSize: 12,
                  fontWeight: 600,
                  flexShrink: 0
                }}>
                  {selectedList.length} seleccionados
                </span>
                <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', color: '#64748b', fontSize: 13 }}>
                  {selectedList.map(o => o.label.split(' (')[0]).join(', ')}
                </span>
              </span>
            )
          ) : selected ? (
            <>
              {selected.color && (
                <span className="ui-select__dot" style={{ backgroundColor: selected.color }} />
              )}
              <span className="ui-select__label">{selected.label}</span>
            </>
          ) : (
            <span className="ui-select__placeholder">{placeholder}</span>
          )}
        </span>
        <ChevronDown size={16} className={`ui-select__chevron ${open ? 'ui-select__chevron--rotated' : ''}`} />
      </button>

      {open && menuPos && createPortal(
        <div
          ref={menuRef}
          className="ui-select__menu"
          role="listbox"
          style={{
            position: 'fixed',
            top: menuPos.top ?? 'auto',
            bottom: menuPos.bottom ?? 'auto',
            left: menuPos.left,
            right: 'auto',
            width: menuPos.width,
            maxHeight: menuPos.maxHeight,
            zIndex: 9999,
          }}
        >
          {showSearch && (
            <div style={{ padding: 6, position: 'sticky', top: 0, background: '#fff', borderBottom: '1px solid #eef2f7', zIndex: 1 }}>
              <input
                ref={searchRef}
                type="text"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Buscar..."
                style={{ width: '100%', padding: '7px 10px', border: '1px solid #e2e8f0', borderRadius: 8, fontSize: 14, outline: 'none', boxSizing: 'border-box' }}
              />
            </div>
          )}
          {clearable && (
            <button
              type="button"
              className={`ui-select__option ${(!multiple && !selected) || (multiple && selectedValues.length === 0) ? 'ui-select__option--selected' : ''}`}
              onClick={() => handleSelect('')}
              role="option"
              aria-selected={(!multiple && !selected) || (multiple && selectedValues.length === 0)}
            >
              <span className="ui-select__placeholder">{placeholder}</span>
              {((!multiple && !selected) || (multiple && selectedValues.length === 0)) && <Check size={15} />}
            </button>
          )}
          {showSearch && visibleOptions.length === 0 && (
            <div className="ui-select__option ui-select__option--disabled" style={{ cursor: 'default' }}>
              <span className="ui-select__placeholder">Sin resultados</span>
            </div>
          )}
          {visibleOptions.map((opt) => {
            const isSelected = multiple
              ? selectedValues.some((v) => String(v) === String(opt.value))
              : String(opt.value) === String(value ?? '')
            return (
              <button
                key={opt.value}
                type="button"
                className={`ui-select__option ${isSelected ? 'ui-select__option--selected' : ''} ${opt.disabled ? 'ui-select__option--disabled' : ''}`}
                onClick={() => handleSelect(opt.value, opt.disabled)}
                disabled={opt.disabled}
                role="option"
                aria-selected={isSelected}
              >
                <span className="ui-select__option-label">
                  {opt.color && <span className="ui-select__dot" style={{ backgroundColor: opt.color }} />}
                  {opt.label}
                </span>
                {isSelected && <Check size={15} />}
              </button>
            )
          })}
        </div>,
        document.body,
      )}

      {/* Hidden native select preserves form integration & required validation */}
      {(required || name) && (
        <select
          className="ui-select__native"
          multiple={multiple}
          value={multiple ? (Array.isArray(value) ? value.map(String) : []) : (value != null ? String(value) : '')}
          onChange={(e) => {
            if (multiple) {
              const selectedOpts = Array.from(e.target.selectedOptions).map((o) => o.value)
              onChange(selectedOpts)
            } else {
              onChange(e.target.value)
            }
          }}
          required={required}
          name={name}
          tabIndex={-1}
          aria-hidden="true"
        >
          <option value="" />
          {options.map((opt) => (
            <option key={opt.value} value={opt.value} />
          ))}
        </select>
      )}
    </div>
  )
}
