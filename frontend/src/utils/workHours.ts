/**
 * Etiquetas de los tres tipos de jornada. Estaban duplicadas como ternarios en
 * cada tabla, y en más de una el ternario solo distinguía dos casos: todo lo
 * que no era 'complete' se pintaba como "Ausencia", así que una recuperación
 * de horas —que es lo contrario de una ausencia— salía marcada como tal.
 */
export const WORK_TYPE: Record<string, string> = {
  complete: 'Jornada',
  absence: 'Ausencia',
  recover: 'Recuperación',
}
