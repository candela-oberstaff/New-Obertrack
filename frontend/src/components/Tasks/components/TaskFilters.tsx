import { useState, useRef, useEffect } from 'react'
import { Filter, X, Calendar, ArrowUpDown, Clock, AlertTriangle } from 'lucide-react'
import styles from '../../../pages/Tasks.module.css'

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
            <select
              className={styles['task-filters-select']}
              value={filters.priority}
              onChange={(e) => onChange({ ...filters, priority: e.target.value })}
            >
              {PRIORITY_OPTIONS.map((opt) => (
                <option key={opt.value} value={opt.value}>{opt.label}</option>
              ))}
            </select>
          </div>

          <div className={styles['task-filters-section']}>
            <label className={styles['task-filters-label']}>
              <Calendar size={13} /> Fecha de entrega
            </label>
            <div className={styles['task-filters-date-row']}>
              <div className={styles['task-filters-date-field']}>
                <span className={styles['task-filters-date-label']}>Desde</span>
                <input
                  type="date"
                  className={styles['task-filters-date-input']}
                  value={filters.dateFrom}
                  onChange={(e) => onChange({ ...filters, dateFrom: e.target.value })}
                />
              </div>
              <div className={styles['task-filters-date-field']}>
                <span className={styles['task-filters-date-label']}>Hasta</span>
                <input
                  type="date"
                  className={styles['task-filters-date-input']}
                  value={filters.dateTo}
                  onChange={(e) => onChange({ ...filters, dateTo: e.target.value })}
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
