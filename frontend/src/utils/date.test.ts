import { describe, it, expect, vi, afterEach } from 'vitest'
import { daysSince, formatDaysSince, formatRelative } from './date'

// La antigüedad en columna se cuenta por días de CALENDARIO, no por múltiplos de
// 24 h: una tarjeta movida anoche tiene que leerse "ayer" aunque hayan pasado 14
// horas. Es la diferencia entre un dato que el usuario reconoce y uno que le
// parece que miente.

function withNow<T>(iso: string, fn: () => T): T {
  vi.useFakeTimers()
  vi.setSystemTime(new Date(iso))
  try {
    return fn()
  } finally {
    vi.useRealTimers()
  }
}

afterEach(() => {
  vi.useRealTimers()
})

describe('daysSince', () => {
  it('cuenta por días de calendario, no por ventanas de 24 horas', () => {
    // Movida ayer a las 22:00, ahora son las 12:00 de hoy: 14 h reales, 1 día.
    const days = withNow('2026-08-20T12:00:00', () =>
      daysSince('2026-08-19T22:00:00'),
    )
    expect(days).toBe(1)
  })

  it('devuelve 0 el mismo día, por temprano que se haya movido', () => {
    const days = withNow('2026-08-20T23:59:00', () =>
      daysSince('2026-08-20T00:01:00'),
    )
    expect(days).toBe(0)
  })

  it('cuenta varios días', () => {
    const days = withNow('2026-08-20T09:00:00', () =>
      daysSince('2026-08-16T18:00:00'),
    )
    expect(days).toBe(4)
  })

  it('nunca devuelve negativos si el reloj del cliente va atrasado', () => {
    const days = withNow('2026-08-20T09:00:00', () =>
      daysSince('2026-08-22T09:00:00'),
    )
    expect(days).toBe(0)
  })

  it('devuelve null sin fecha o con una fecha ilegible', () => {
    expect(daysSince(undefined)).toBeNull()
    expect(daysSince(null)).toBeNull()
    expect(daysSince('')).toBeNull()
    expect(daysSince('no es una fecha')).toBeNull()
  })
})

describe('formatDaysSince', () => {
  it('usa lenguaje natural en los casos cercanos', () => {
    expect(withNow('2026-08-20T12:00:00', () => formatDaysSince('2026-08-20T08:00:00'))).toBe('hoy')
    expect(withNow('2026-08-20T12:00:00', () => formatDaysSince('2026-08-19T22:00:00'))).toBe('ayer')
    expect(withNow('2026-08-20T12:00:00', () => formatDaysSince('2026-08-16T10:00:00'))).toBe('hace 4 días')
  })

  it('devuelve cadena vacía sin fecha, para poder incrustarlo sin condicionales', () => {
    // Las tareas anteriores a la bitácora que nadie ha movido no tienen sello, y
    // la ficha prefiere no decir nada antes que inventar un "hoy" falso.
    expect(formatDaysSince(undefined)).toBe('')
  })
})

describe('formatRelative', () => {
  // La bitácora de una automatización se lee para responder "¿esto acaba de pasar?".
  // Un timestamp completo obliga a hacer la resta mentalmente; esto no.
  it('usa escalas cortas para lo reciente', () => {
    const ahora = new Date('2026-08-20T12:00:00')
    withNow(ahora.toISOString(), () => {
      expect(formatRelative('2026-08-20T11:59:30')).toBe('ahora')
      expect(formatRelative('2026-08-20T11:55:00')).toBe('hace 5 min')
      expect(formatRelative('2026-08-20T09:00:00')).toBe('hace 3 h')
      expect(formatRelative('2026-08-18T12:00:00')).toBe('hace 2 d')
    })
  })

  // Pasada una semana la escala relativa deja de informar: "hace 34 d" no lo lee
  // nadie, así que se muestra la fecha.
  it('pasa a fecha por encima de una semana', () => {
    withNow('2026-08-20T12:00:00', () => {
      expect(formatRelative('2026-07-01T12:00:00')).toMatch(/jul/)
    })
  })

  it('tolera fechas ausentes o ilegibles', () => {
    expect(formatRelative(undefined)).toBe('')
    expect(formatRelative('no es una fecha')).toBe('')
  })
})
