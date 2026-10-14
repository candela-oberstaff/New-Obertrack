import styles from '../Admin.module.css'

// Selector del tipo de jornada del profesional: completa (8h) o media (4h).
//
// Se guarda en el empleo activo (employments.schedule_type), el mismo campo que
// se edita desde la pestaña Horarios de la empresa, y decide cuántas horas vale
// la jornada que el profesional registra en su panel de Horas. Solo existe para
// profesionales: el resto de roles no registra jornadas.

export const SCHEDULE_FULL_TIME = 'Full-time'
export const SCHEDULE_PART_TIME = 'Part-time'

const TYPES = [
  { value: SCHEDULE_FULL_TIME, label: 'Jornada completa · 8h', hint: 'Registra jornadas de 8 horas.' },
  { value: SCHEDULE_PART_TIME, label: 'Media jornada · 4h', hint: 'Registra jornadas de 4 horas; ausencias y recuperaciones se limitan a 4h.' },
]

/** Etiqueta legible del tipo de jornada; vacío cuenta como jornada completa. */
export function workdayLabel(scheduleType: string | undefined): string {
  return (TYPES.find(t => t.value === scheduleType) ?? TYPES[0]).label
}

interface WorkdayTypeSelectorProps {
  /** Vacío equivale a jornada completa, igual que en el backend. */
  value: string | undefined
  onChange: (value: string) => void
}

export function WorkdayTypeSelector({ value, onChange }: WorkdayTypeSelectorProps) {
  const current = value === SCHEDULE_PART_TIME ? SCHEDULE_PART_TIME : SCHEDULE_FULL_TIME

  return (
    <div className={styles['form-group']}>
      <label>Tipo de jornada</label>
      <div className={styles['level-group']} role="radiogroup" aria-label="Tipo de jornada">
        {TYPES.map(t => (
          <button
            key={t.value}
            type="button"
            role="radio"
            aria-checked={current === t.value}
            className={`${styles['level-btn']} ${current === t.value ? styles['active'] : ''}`}
            onClick={() => onChange(t.value)}
          >
            {t.label}
          </button>
        ))}
      </div>
      <p className={styles['level-hint']}>
        {TYPES.find(t => t.value === current)?.hint}
      </p>
    </div>
  )
}
