import { useState, useRef, useEffect, useCallback, type ReactNode } from 'react'
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
  value: SelectValue | null | undefined
  onChange: (value: SelectValue) => void
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
}: SelectProps) {
  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  const selected = options.find((o) => String(o.value) === String(value ?? ''))

  // Close on outside click
  useEffect(() => {
    if (!open) return
    const handleClick = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
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
    onChange(val)
    setOpen(false)
  }, [onChange])

  return (
    <div
      className={`ui-select ${fullWidth ? 'ui-select--full' : ''}`}
      ref={containerRef}
    >
      <button
        type="button"
        id={id}
        className={`ui-select__trigger ${open ? 'ui-select__trigger--open' : ''} ${disabled ? 'ui-select__trigger--disabled' : ''} ${className}`}
        onClick={() => !disabled && setOpen((v) => !v)}
        disabled={disabled}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-label={ariaLabel}
      >
        <span className="ui-select__value">
          {leftIcon && <span className="ui-select__icon">{leftIcon}</span>}
          {selected ? (
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

      {open && (
        <div className="ui-select__menu" role="listbox">
          {clearable && (
            <button
              type="button"
              className={`ui-select__option ${!selected ? 'ui-select__option--selected' : ''}`}
              onClick={() => handleSelect('')}
              role="option"
              aria-selected={!selected}
            >
              <span className="ui-select__placeholder">{placeholder}</span>
              {!selected && <Check size={15} />}
            </button>
          )}
          {options.map((opt) => {
            const isSelected = String(opt.value) === String(value ?? '')
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
        </div>
      )}

      {/* Hidden native select preserves form integration & required validation */}
      {(required || name) && (
        <select
          className="ui-select__native"
          value={value ?? ''}
          onChange={(e) => onChange(e.target.value)}
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
