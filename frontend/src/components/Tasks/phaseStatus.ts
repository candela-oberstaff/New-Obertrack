import type { Phase } from './types'

/** Límite de la columna tasks.status en la base. */
export const STATUS_MAX_LENGTH = 50

/**
 * Identificador de columna de una fase: es lo que se guarda como `status` de la
 * tarea al arrastrarla, y también con lo que se agrupan y cuentan las tarjetas.
 *
 * Una fase creada desde el tablero no trae `status` (el backend solo guarda
 * nombre, color y orden), así que se deriva del nombre. Vive aquí porque lo
 * calculan cuatro pantallas y basta con que una lo haga distinto para que las
 * tarjetas dejen de caer en su columna.
 *
 * El recorte es lo que arregla el 500 al arrastrar: con un nombre de más de 20
 * caracteres el valor no cabía en la columna y Postgres rechazaba el UPDATE.
 */
export function phaseStatusId(phase: Pick<Phase, 'name' | 'status'>): string {
  const base = phase.status || phase.name.toLowerCase().replace(/\s+/g, '_')
  // Recorte por code points (Array.from), no por unidades UTF-16 (.slice):
  // así nunca se parte un emoji por la mitad (Postgres rechaza el surrogate
  // suelto) y coincide con el recorte por runas de PhaseColumnID en el backend.
  return Array.from(base).slice(0, STATUS_MAX_LENGTH).join('')
}
