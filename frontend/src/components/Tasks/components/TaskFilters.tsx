import { useState, useRef, useEffect } from 'react'
import { Filter, X, Calendar, ArrowUpDown, Clock, AlertTriangle, ListOrdered } from 'lucide-react'
import { DatePicker } from '../../ui'
import styles from '../../../pages/Tasks.module.css'
import { Select } from '../../ui/Select'
import type { TaskSortMode } from '../taskSort'

export interface TaskFiltersState {
  priority: string
  dateFrom: string
  dateTo: string
  dateStatus: string
}

interface TaskFiltersProps {
  filters: TaskFiltersState
  onChange: (filters: TaskFiltersState) => void
  sortMode?: TaskSortMode
  /** Sin él no se muestra la sección de orden. */
  onSortChange?: (mode: TaskSortMode) => void
}

const SORT_OPTIONS: { value: TaskSortMode; label: string }[] = [
  { value: 'manual', label: 'Manual' },
  { value: 'due_asc', label: 'Entrega más próxima' },
  { value: 'due_desc', label: 'Entrega más lejana' },
]

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

export function TaskFilters({ filters, onChange, sortMode = 'manual', onSortChange }: TaskFiltersProps) {
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

  const isSorted = sortMode !== 'manual'

  const handleClear = () => {
    onChange(DEFAULT_FILTERS)
    onSortChange?.('manual')
    setOpen(false)
  }

  return (
    <div className={styles['task-filters-wrapper']} ref={ref}>
      <button
        type="button"
        className={`${styles['task-filters-toggle']} ${hasActiveFilters || isSorted ? styles['task-filters-active'] : ''}`}
        onClick={() => setOpen(!open)}
        title={isSorted ? 'Filtrar y ordenar (ordenada por fecha de entrega)' : 'Filtrar y ordenar tareas'}
        aria-label="Filtrar y ordenar tareas"
      >
        <Filter size={14} />
        {(hasActiveFilters || isSorted) && <span className={styles['task-filters-badge']} />}
      </button>

      {open && (
        <div className={styles['task-filters-dropdown']}>
          <div className={styles['task-filters-header']}>
            <span className={styles['task-filters-title']}>Filtros y orden</span>
            <button type="button" className={styles['task-filters-clear-btn']} onClick={handleClear}>
              <X size={14} /> Limpiar
            </button>
          </div>

          {onSortChange && (
            <div className={styles['task-filters-section']}>
              <label className={styles['task-filters-label']}>
                <ListOrdered size={13} /> Ordenar por
              </label>
              <div className={styles['task-filters-chip-row']} role="radiogroup" aria-label="Ordenar por">
                {SORT_OPTIONS.map((opt) => (
                  <button
                    key={opt.value}
                    type="button"
                    role="radio"
                    aria-checked={sortMode === opt.value}
                    className={`${styles['task-filters-chip']} ${sortMode === opt.value ? styles['task-filters-chip-active'] : ''}`}
                    onClick={() => onSortChange(opt.value)}
                  >
                    {opt.label}
                  </button>
                ))}
              </div>
              {isSorted && (
                <span className={styles['task-filters-date-label']} style={{ marginTop: 6, display: 'block' }}>
                  Las tareas sin fecha van al final. Mientras esté así, arrastrar dentro de la columna no cambia el orden.
                </span>
              )}
            </div>
          )}

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
