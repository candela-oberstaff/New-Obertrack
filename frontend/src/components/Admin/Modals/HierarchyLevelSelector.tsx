import styles from '../Admin.module.css'

// Selector del nivel en la jerarquía: Profesional / Manager / Supervisor.
//
// Vive aparte porque lo usan los DOS modales de usuario —crear y editar— y tenerlo
// duplicado es justo lo que hizo que divergieran: durante un tiempo sólo se podía
// marcar a alguien como manager DESPUÉS de crearlo, y había que dar el alta en dos
// pasos sin ninguna razón.
//
// Los tres niveles son excluyentes y acumulativos: un supervisor es un manager que
// además tiene managers a su cargo. Por eso se representan como un control de una
// línea y no como dos casillas sueltas, que dejarían expresar el estado imposible
// "supervisor pero no manager".

export interface HierarchyLevels {
  is_manager?: boolean
  is_supervisor?: boolean
}

interface HierarchyLevelSelectorProps {
  value: HierarchyLevels
  onChange: (levels: { is_manager: boolean; is_supervisor: boolean }) => void
  /** Etiqueta del grupo. Por defecto, la que usan ambos modales. */
  label?: string
}

const LEVELS = [
  { value: 'profesional', label: 'Profesional', hint: 'Registra sus horas y tareas. No aprueba a nadie.' },
  { value: 'manager', label: 'Manager', hint: 'Aprueba las horas de los profesionales a su cargo.' },
  { value: 'supervisor', label: 'Supervisor', hint: 'Tiene managers a su cargo: ve y aprueba todo lo que cuelga de él.' },
]

/** Traduce las dos banderas al nivel que representan. */
export function levelOf(value: HierarchyLevels): string {
  if (value.is_supervisor) return 'supervisor'
  if (value.is_manager) return 'manager'
  return 'profesional'
}

export function HierarchyLevelSelector({ value, onChange, label = 'Nivel en la jerarquía' }: HierarchyLevelSelectorProps) {
  const current = levelOf(value)

  // Supervisor implica manager: se emiten SIEMPRE las dos banderas juntas para que
  // no exista forma de dejarlas incoherentes desde la interfaz. El backend impone la
  // misma regla, así que esto es coherencia, no la única defensa.
  const select = (level: string) => onChange({
    is_manager: level !== 'profesional',
    is_supervisor: level === 'supervisor',
  })

  return (
    <div className={styles['form-group']}>
      <label>{label}</label>
      <div className={styles['level-group']} role="radiogroup" aria-label={label}>
        {LEVELS.map(l => (
          <button
            key={l.value}
            type="button"
            role="radio"
            aria-checked={current === l.value}
            className={`${styles['level-btn']} ${current === l.value ? styles['active'] : ''}`}
            onClick={() => select(l.value)}
          >
            {l.label}
          </button>
        ))}
      </div>
      {/* La pista explica qué gana la persona con cada nivel. Sin ella, "manager" y
          "supervisor" se eligen a ojo y acaban repartidos al azar. */}
      <p className={styles['level-hint']}>
        {LEVELS.find(l => l.value === current)?.hint}
      </p>
    </div>
  )
}
