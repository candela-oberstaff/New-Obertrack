import type { WorkHour } from '../../types'

export const DAYS_ES = ['Dom', 'Lun', 'Mar', 'Mié', 'Jue', 'Vie', 'Sáb']
export const MONTHS_ES = ['Enero', 'Febrero', 'Marzo', 'Abril', 'Mayo', 'Junio', 'Julio', 'Agosto', 'Septiembre', 'Octubre', 'Noviembre', 'Diciembre']
export const JORNADA_COMPLETA = 8
export const MEDIA_JORNADA = 4

type DayRecord = Pick<WorkHour, 'work_type' | 'hours_worked' | 'absence_hours'>

/**
 * Horas de ausencia de una jornada. La API omite el 0 y los registros antiguos
 * no guardaban absence_hours; esos eran todos de jornada de 8h, de ahí el
 * respaldo sobre JORNADA_COMPLETA.
 */
export const absenceHoursOf = (wh: DayRecord): number => {
  if (wh.work_type !== 'absence') return 0
  if (wh.absence_hours) return wh.absence_hours
  return Math.max(0, JORNADA_COMPLETA - wh.hours_worked)
}

/** Etiqueta de un día trabajado entero: media jornada (4h) o jornada completa. */
export const fullDayLabel = (wh: DayRecord): string =>
  wh.hours_worked > 0 && wh.hours_worked <= MEDIA_JORNADA ? 'Media Jornada' : 'Jornada Completa'

export const parseLocalDate = (dateStr: string): Date => {
  const [year, month, day] = dateStr.split('T')[0].split('-').map(Number)
  return new Date(year, month - 1, day)
}
