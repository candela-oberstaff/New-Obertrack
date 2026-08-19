import { Clock } from 'lucide-react'
import { DatePicker, Select, toISODate, fromISODate } from '../../ui'
import styles from './DateTimeField.module.css'

interface DateTimeFieldProps {
  /** Momento en ISO con zona, o null si no hay nada elegido. */
  value?: string | null
  onChange: (value: string | null) => void
  /** Primer día elegible, en AAAA-MM-DD. */
  minDate?: string
  ariaLabel?: string
}

/** Horas en pasos de media hora: programar un anuncio al minuto no aporta. */
const TIME_OPTIONS = Array.from({ length: 48 }, (_, i) => {
  const value = `${String(Math.floor(i / 2)).padStart(2, '0')}:${i % 2 ? '30' : '00'}`
  return { value, label: value }
})

const DEFAULT_TIME = '09:00'

/**
 * Fecha y hora con los componentes de la app: el calendario propio y un
 * desplegable de horas. El <input type="datetime-local"> nativo abría el
 * calendario del sistema operativo, que no se parece en nada al resto de
 * Obertrack y cambia de aspecto en cada navegador.
 */
export function DateTimeField({ value, onChange, minDate, ariaLabel }: DateTimeFieldProps) {
  const date = value ? new Date(value) : null
  const valid = date && !Number.isNaN(date.getTime())

  const datePart = valid ? toISODate(date) : ''
  const timePart = valid
    ? `${String(date.getHours()).padStart(2, '0')}:${date.getMinutes() < 30 ? '00' : '30'}`
    : DEFAULT_TIME

  // Se emite un único momento ISO: las dos mitades son una sola decisión.
  const emit = (nextDate: string, nextTime: string) => {
    if (!nextDate) {
      onChange(null)
      return
    }
    const base = fromISODate(nextDate)
    if (!base) {
      onChange(null)
      return
    }
    const [hours, minutes] = nextTime.split(':').map(Number)
    base.setHours(hours, minutes, 0, 0)
    onChange(base.toISOString())
  }

  return (
    <div className={styles['field']}>
      <DatePicker
        value={datePart}
        onChange={(next) => emit(next, timePart)}
        min={minDate}
        clearable
        fullWidth
        ariaLabel={ariaLabel}
        placeholder="Elegir fecha"
      />
      <Select
        options={TIME_OPTIONS}
        value={timePart}
        onChange={(next) => emit(datePart, String(next))}
        leftIcon={<Clock size={15} />}
        // Con 48 horas en la lista, buscar es más rápido que desplazarse.
        searchable
        disabled={!datePart}
        ariaLabel="Hora"
      />
    </div>
  )
}
