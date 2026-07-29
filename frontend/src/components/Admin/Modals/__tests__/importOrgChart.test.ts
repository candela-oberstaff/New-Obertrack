import { describe, it, expect } from 'vitest'
import { buildOrgGroups } from '../ImportUsersModal'

// El árbol del organigrama es lo único que la previsualización arma del lado
// del cliente, así que sus casos raros (círculos, managers de otra empresa,
// filas incompletas) se fijan acá.

type Row = Parameters<typeof buildOrgGroups>[0][number]

const row = (n: number, nombre: string, email: string, empresa: string, reportaA = '', status: Row['status'] = 'ok'): Row => ({
  row: n,
  status,
  data: { nombre, email, empresa, reporta_a: reportaA },
})

describe('buildOrgGroups', () => {
  it('agrupa por empresa y cuelga a cada quien de su manager', () => {
    const rows = [
      row(2, 'Ricardo', 'ricardo@a.test', 'Alfa'),
      row(3, 'Martín', 'martin@a.test', 'Alfa', 'ricardo@a.test'),
      row(4, 'Valentina', 'valentina@a.test', 'Alfa', 'martin@a.test'),
      row(5, 'Camila', 'camila@b.test', 'Beta'),
      row(6, 'Tomás', 'tomas@b.test', 'Beta', 'camila@b.test'),
    ]
    const groups = buildOrgGroups(rows, true)

    expect(groups.map(g => g.company)).toEqual(['Alfa', 'Beta'])

    const alfa = groups[0]
    // Solo Ricardo es raíz: los otros dos cuelgan de alguien del archivo.
    expect(alfa.roots).toHaveLength(1)
    expect(alfa.roots[0].label).toBeNull()
    expect(alfa.roots[0].rows.map(r => r.data.nombre)).toEqual(['Ricardo'])
    expect(alfa.orphans).toHaveLength(0)
    expect(alfa.crossCompany).toHaveLength(0)
  })

  it('agrupa bajo un encabezado propio a quienes reportan a alguien que ya existe', () => {
    const rows = [
      row(2, 'Nuevo Uno', 'uno@a.test', 'Alfa', 'jefe.existente@a.test'),
      row(3, 'Nuevo Dos', 'dos@a.test', 'Alfa', 'jefe.existente@a.test'),
    ]
    const [alfa] = buildOrgGroups(rows, true)

    expect(alfa.roots).toHaveLength(1)
    expect(alfa.roots[0].label).toBe('jefe.existente@a.test')
    expect(alfa.roots[0].rows).toHaveLength(2)
    expect(alfa.orphans).toHaveLength(0)
  })

  it('separa al manager que está en el archivo pero en otra empresa', () => {
    const rows = [
      row(2, 'Cruce', 'cruce@a.test', 'Alfa', 'jefe@b.test'),
      row(3, 'Jefe Beta', 'jefe@b.test', 'Beta'),
    ]
    const [alfa] = buildOrgGroups(rows, true)

    // No debe leerse como "ya existe en el sistema": está en el archivo.
    expect(alfa.roots).toHaveLength(0)
    expect(alfa.crossCompany.map(r => r.row)).toEqual([2])
    expect(alfa.orphans).toHaveLength(0)
  })

  it('manda al bucket de círculo solo a quienes forman parte de él', () => {
    const rows = [
      row(2, 'Base', 'base@a.test', 'Alfa'),
      row(3, 'Ana', 'ana@a.test', 'Alfa', 'bruno@a.test'),
      row(4, 'Bruno', 'bruno@a.test', 'Alfa', 'ana@a.test'),
      row(5, 'Ajeno', 'ajeno@a.test', 'Alfa', 'base@a.test'),
    ]
    const [alfa] = buildOrgGroups(rows, true)

    expect(alfa.orphans.map(r => r.data.nombre).sort()).toEqual(['Ana', 'Bruno'])
    // Ajeno cuelga de Base, así que se alcanza y no es huérfano.
    expect(alfa.orphans.map(r => r.data.nombre)).not.toContain('Ajeno')
  })

  it('no da por huérfana a una fila incompleta sin email', () => {
    const rows = [row(2, 'Sin Email', '', 'Alfa'), row(3, 'Normal', 'n@a.test', 'Alfa')]
    const [alfa] = buildOrgGroups(rows, true)

    expect(alfa.orphans).toHaveLength(0)
    expect(alfa.roots[0].rows).toHaveLength(2)
  })

  it('en modo empresa arma un solo grupo', () => {
    const rows = [
      row(2, 'Jefa', 'jefa@a.test', ''),
      row(3, 'Reporte', 'rep@a.test', '', 'jefa@a.test'),
    ]
    const groups = buildOrgGroups(rows, false)

    expect(groups).toHaveLength(1)
    expect(groups[0].roots[0].rows.map(r => r.data.nombre)).toEqual(['Jefa'])
  })
})
