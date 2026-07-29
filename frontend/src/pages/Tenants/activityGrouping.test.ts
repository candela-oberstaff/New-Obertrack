import { describe, it, expect } from 'vitest'
import { groupByDay, dayLabel } from './activityGrouping'
import type { TenantActivity } from '../../hooks'

// "Ahora" fijo para que las etiquetas relativas no dependan del día en que se
// ejecuten los tests.
const NOW = new Date('2026-07-29T14:00:00')

function act(timestamp: string, type = 'work_hour'): TenantActivity {
  return { type, category: 'work', user: 'Alguien', user_id: 1, company: 'Acme', details: 'x', timestamp, event_id: 0 }
}

describe('dayLabel', () => {
  it('usa "Hoy" y "Ayer" para lo reciente', () => {
    expect(dayLabel(new Date('2026-07-29T09:00:00'), NOW)).toBe('Hoy')
    expect(dayLabel(new Date('2026-07-28T23:59:00'), NOW)).toBe('Ayer')
  })

  // Un movimiento de esta madrugada y otro de ayer por la noche están a pocas
  // horas, pero son días distintos: la etiqueta va por día natural, no por
  // diferencia de horas.
  it('separa por día natural, no por horas transcurridas', () => {
    expect(dayLabel(new Date('2026-07-29T00:30:00'), NOW)).toBe('Hoy')
    expect(dayLabel(new Date('2026-07-28T22:30:00'), NOW)).toBe('Ayer')
  })

  it('para el resto del año en curso da el día de la semana sin año', () => {
    const label = dayLabel(new Date('2026-07-24T13:44:00'), NOW)
    expect(label).toContain('julio')
    expect(label).not.toContain('2026')
  })

  it('incluye el año cuando el movimiento es de otro año', () => {
    expect(dayLabel(new Date('2025-12-31T10:00:00'), NOW)).toContain('2025')
  })
})

describe('groupByDay', () => {
  it('junta en un grupo los movimientos del mismo día', () => {
    const groups = groupByDay(
      [act('2026-07-29T12:00:00'), act('2026-07-29T09:00:00'), act('2026-07-28T18:00:00')],
      NOW,
    )
    expect(groups).toHaveLength(2)
    expect(groups[0].label).toBe('Hoy')
    expect(groups[0].items).toHaveLength(2)
    expect(groups[1].label).toBe('Ayer')
    expect(groups[1].items).toHaveLength(1)
  })

  it('conserva el orden que trae el servidor dentro de cada grupo', () => {
    const groups = groupByDay([act('2026-07-29T12:00:00'), act('2026-07-29T09:00:00')], NOW)
    expect(groups[0].items.map(i => i.timestamp)).toEqual([
      '2026-07-29T12:00:00',
      '2026-07-29T09:00:00',
    ])
  })

  it('sin movimientos no crea grupos', () => {
    expect(groupByDay([], NOW)).toEqual([])
  })

  // Si el orden viniera roto, es preferible ver dos grupos del mismo día que
  // una cronología que junta movimientos separados por el resto de la lista.
  it('no fusiona días iguales que no vengan seguidos', () => {
    const groups = groupByDay(
      [act('2026-07-29T12:00:00'), act('2026-07-28T18:00:00'), act('2026-07-29T08:00:00')],
      NOW,
    )
    expect(groups).toHaveLength(3)
    expect(groups.map(g => g.label)).toEqual(['Hoy', 'Ayer', 'Hoy'])
  })

  it('agrupa aparte lo que llegue con fecha ilegible en vez de romper', () => {
    const groups = groupByDay([act('no-es-una-fecha'), act('2026-07-29T12:00:00')], NOW)
    expect(groups[0].label).toBe('Fecha no disponible')
    expect(groups[0].key).toBe('sin-fecha')
    expect(groups[1].label).toBe('Hoy')
  })
})
