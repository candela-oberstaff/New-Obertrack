import { useState, useRef, useEffect } from 'react'
import { Filter, X, Calendar, ArrowUpDown, Clock, AlertTriangle } from 'lucide-react'
import { DatePicker } from '../../ui'
import styles from '../../../pages/Tasks.module.css'
import { Select } from '../../ui/Select'

export interface TaskFiltersState {
  priority: string
  dateFrom: string
  dateTo: string
  dateStatus: string
}

interface TaskFiltersProps {
  filters: TaskFiltersState
  onChange: (filters: TaskFiltersState) => void
}

const PRIORITY_OPTIONS = [
  { value: '', label: 'Todas' },
  { value: 'urgent', label: 'Urgente' },
  { value: 'high', label: 'Alta' },
  { value: 'medium', label: 'Media' },
  { value: 'low', label: 'Baja' },
]

const DATE_STATUS_OPTIONS = [
  { value: '', label: 'Cualquier fecha' },
  { value: 'overdue', label: 'Vencidas' },
  { value: 'today', label: 'Vencen hoy' },
  { value: 'week', label: 'Próximos 7 días' },
]

export const DEFAULT_FILTERS: TaskFiltersState = {
  priority: '',
  dateFrom: '',
  dateTo: '',
  dateStatus: '',
}

export function TaskFilters({ filters, onChange }: TaskFiltersProps) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  const hasActiveFilters = filters.priority !== '' || filters.dateFrom !== '' || filters.dateTo !== '' || filters.dateStatus !== ''

  const handleClear = () => {
    onChange(DEFAULT_FILTERS)
    setOpen(false)
  }

  return (
    <div className={styles['task-filters-wrapper']} ref={ref}>
      <button
        type="button"
        className={`${styles['task-filters-toggle']} ${hasActiveFilters ? styles['task-filters-active'] : ''}`}
        onClick={() => setOpen(!open)}
        title="Filtrar tareas"
      >
        <Filter size={14} />
        {hasActiveFilters && <span className={styles['task-filters-badge']} />}
      </button>

      {open && (
        <div className={styles['task-filters-dropdown']}>
          <div className={styles['task-filters-header']}>
            <span className={styles['task-filters-title']}>Filtros</span>
            <button type="button" className={styles['task-filters-clear-btn']} onClick={handleClear}>
              <X size={14} /> Limpiar
            </button>
          </div>

          <div className={styles['task-filters-section']}>
            <label className={styles['task-filters-label']}>
              <ArrowUpDown size={13} /> Urgencia
            </label>
            <Select
              value={filters.priority}
              onChange={(v) => onChange({ ...filters, priority: String(v) })}
              className="ui-select__trigger--compact"
              ariaLabel="Prioridad"
              fullWidth
              options={PRIORITY_OPTIONS.map((opt) => ({ value: opt.value, label: opt.label }))}
            />
          </div>

          <div className={styles['task-filters-section']}>
            <label className={styles['task-filters-label']}>
              <Calendar size={13} /> Fecha de entrega
            </label>
            <div className={styles['task-filters-date-row']}>
              <div className={styles['task-filters-date-field']}>
                <span className={styles['task-filters-date-label']}>Desde</span>
                <DatePicker
                  compact
                  fullWidth
                  clearable
                  value={filters.dateFrom}
                  onChange={(v) => onChange({ ...filters, dateFrom: v })}
                  ariaLabel="Desde"
                />
              </div>
              <div className={styles['task-filters-date-field']}>
                <span className={styles['task-filters-date-label']}>Hasta</span>
                <DatePicker
                  compact
                  fullWidth
                  clearable
                  value={filters.dateTo}
                  min={filters.dateFrom || undefined}
                  onChange={(v) => onChange({ ...filters, dateTo: v })}
                  ariaLabel="Hasta"
                />
              </div>
            </div>
          </div>

          <div className={styles['task-filters-section']}>
            <label className={styles['task-filters-label']}>
              <Clock size={13} /> Estado de fecha
            </label>
            <div className={styles['task-filters-chip-row']}>
              {DATE_STATUS_OPTIONS.map((opt) => (
                <button
                  key={opt.value}
                  type="button"
                  className={`${styles['task-filters-chip']} ${filters.dateStatus === opt.value ? styles['task-filters-chip-active'] : ''}`}
                  onClick={() => onChange({ ...filters, dateStatus: filters.dateStatus === opt.value ? '' : opt.value })}
                >
                  {opt.value === 'overdue' && <AlertTriangle size={12} />}
                  {opt.label}
                </button>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
