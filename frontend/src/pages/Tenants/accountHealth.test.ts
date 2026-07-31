import { describe, it, expect } from 'vitest'
import { daysSince, healthSignal } from './accountHealth'

/**
 * Las fechas se construyen en hora LOCAL y se serializan a ISO, no se escriben
 * como literales UTC: daysSince cuenta días de calendario tal y como los ve el
 * usuario, así que un literal "…T00:01:00Z" cae en un día distinto según dónde
 * corra el test y lo volvería inestable.
 */
const at = (y: number, m: number, d: number, h = 10) => new Date(y, m - 1, d, h).toISOString()
const NOW = new Date(2026, 6, 31, 10) // 31 de julio de 2026, hora local

describe('daysSince', () => {
  it('cuenta días de calendario, no periodos de 24 horas', () => {
    // Ayer a las 23:00 es "1 día" aunque hayan pasado 11 horas: es como lo lee
    // quien mira la ficha.
    expect(daysSince(at(2026, 7, 30, 23), NOW)).toBe(1)
    expect(daysSince(at(2026, 7, 31, 0), NOW)).toBe(0)
  })

  it('devuelve null cuando no hay fecha', () => {
    expect(daysSince(null, NOW)).toBeNull()
    expect(daysSince(undefined, NOW)).toBeNull()
    expect(daysSince('', NOW)).toBeNull()
  })

  it('devuelve null con una fecha ilegible en vez de NaN', () => {
    expect(daysSince('no-es-una-fecha', NOW)).toBeNull()
  })

  it('trata el futuro como hoy en lugar de dar días negativos', () => {
    expect(daysSince(at(2026, 8, 5), NOW)).toBe(0)
  })
})

describe('healthSignal', () => {
  it('distingue nunca de hace mucho', () => {
    // Las dos piden lo mismo (llamar), pero no significan lo mismo y el texto
    // no debe confundirlas.
    expect(healthSignal(null, NOW)).toMatchObject({ level: 'never', days: null, label: 'nunca' })
    expect(healthSignal(at(2020, 1, 1), NOW).level).toBe('stale')
  })

  it('usa lenguaje natural para lo reciente', () => {
    expect(healthSignal(at(2026, 7, 31, 8), NOW).label).toBe('hoy')
    expect(healthSignal(at(2026, 7, 30, 8), NOW).label).toBe('ayer')
    expect(healthSignal(at(2026, 7, 26), NOW).label).toBe('hace 5 días')
  })

  it('pasa a meses y años para no escupir números enormes', () => {
    expect(healthSignal(at(2026, 7, 1), NOW).label).toBe('hace 1 mes')      // 30 días
    expect(healthSignal(at(2026, 6, 1), NOW).label).toBe('hace 2 meses')    // 60 días
    expect(healthSignal(at(2026, 2, 1), NOW).label).toBe('hace 6 meses')    // 180 días
    expect(healthSignal(at(2024, 7, 31), NOW).label).toBe('hace más de 2 años')
  })

  it('escala el nivel en los umbrales de 30 y 90 días', () => {
    expect(healthSignal(at(2026, 7, 2), NOW).level).toBe('ok')    // 29 días
    expect(healthSignal(at(2026, 7, 1), NOW).level).toBe('warn')  // 30 días
    expect(healthSignal(at(2026, 5, 3), NOW).level).toBe('warn')  // 89 días
    expect(healthSignal(at(2026, 5, 2), NOW).level).toBe('stale') // 90 días
  })
})
