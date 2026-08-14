import { describe, it, expect } from 'vitest'
import { buildForest, descendantIds, dropRejection, teamSizes, type OrgPerson } from '../orgTree'

const person = (id: number, name: string, managerId: number | null, extra: Partial<OrgPerson> = {}): OrgPerson => ({
  user_id: id,
  employment_id: 100 + id,
  name,
  email: `${name.toLowerCase()}@x.com`,
  is_manager: false,
  is_active: true,
  manager_id: managerId,
  ...extra,
})

// Ana (supervisora) → María (manager) → Pedro, y Luis suelto sin manager.
const empresa = (): OrgPerson[] => [
  person(1, 'Ana', null, { is_manager: true, is_supervisor: true }),
  person(2, 'María', 1, { is_manager: true }),
  person(3, 'Pedro', 2),
  person(4, 'Luis', null),
]

describe('buildForest', () => {
  it('arma el árbol y ordena por nombre en cada nivel', () => {
    const roots = buildForest(empresa())

    expect(roots.map(r => r.person.name)).toEqual(['Ana', 'Luis'])
    expect(roots[0].children.map(c => c.person.name)).toEqual(['María'])
    expect(roots[0].children[0].children.map(c => c.person.name)).toEqual(['Pedro'])
  })

  it('sube a la raíz a quien apunta a un manager que no está en la lista', () => {
    // Es el caso del supervisor mirando SU rama: su propio jefe queda fuera.
    const rama = [person(2, 'María', 99, { is_manager: true }), person(3, 'Pedro', 2)]
    const roots = buildForest(rama)

    expect(roots.map(r => r.person.name)).toEqual(['María'])
    expect(roots[0].children.map(c => c.person.name)).toEqual(['Pedro'])
  })

  it('no pierde a nadie si los datos traen un círculo', () => {
    // A→B→A no cuelga de ninguna raíz: sin rescate desaparecerían del dibujo, y
    // son justo los que hay que ver para poder arreglarlos.
    const ciclo = [person(1, 'Ana', 2), person(2, 'Beto', 1)]
    const roots = buildForest(ciclo)

    const dibujados = new Set<number>()
    const walk = (n: any) => { dibujados.add(n.person.user_id); n.children.forEach(walk) }
    roots.forEach(walk)

    expect(dibujados).toEqual(new Set([1, 2]))
  })

  it('devuelve vacío sin gente', () => {
    expect(buildForest([])).toEqual([])
  })
})

describe('descendantIds', () => {
  it('baja a cualquier profundidad', () => {
    expect(descendantIds(1, empresa())).toEqual(new Set([2, 3]))
    expect(descendantIds(2, empresa())).toEqual(new Set([3]))
    expect(descendantIds(3, empresa())).toEqual(new Set())
  })

  it('termina aunque los datos tengan un círculo', () => {
    expect(descendantIds(1, [person(1, 'Ana', 2), person(2, 'Beto', 1)])).toEqual(new Set([2, 1]))
  })
})

describe('dropRejection', () => {
  it('permite mover a alguien bajo otra rama', () => {
    expect(dropRejection(3, 1, empresa())).toBeNull()
  })

  it('permite soltar en la raíz (quitar el manager)', () => {
    expect(dropRejection(3, null, empresa())).toBeNull()
  })

  it('rechaza que alguien sea su propio manager', () => {
    expect(dropRejection(1, 1, empresa())).toMatch(/su propio manager/)
  })

  it('rechaza el círculo: soltar a un jefe dentro de su propio equipo', () => {
    // Ana sobre Pedro la pondría bajo alguien que ya cuelga de ella.
    expect(dropRejection(1, 3, empresa())).toMatch(/círculo/)
  })

  it('rechaza a un manager destino inactivo', () => {
    const gente = empresa()
    gente[1].is_active = false // María
    // Luis, que no cuelga de ella: si se usara a Pedro, el chequeo de "ya está
    // ahí" cortaría antes y no se probaría nada.
    expect(dropRejection(4, 2, gente)).toMatch(/inactivo/)
  })

  it('no se queja si ya está donde lo sueltan', () => {
    expect(dropRejection(3, 2, empresa())).toBeNull()
  })
})

describe('la cuenta de empresa', () => {
  const conEmpresa = (): OrgPerson[] => [
    person(99, 'Acme S.A', null, { is_company: true }),
    person(1, 'Ana', 99, { is_manager: true, is_supervisor: true }),
    person(3, 'Pedro', 1),
  ]

  it('encabeza el árbol y todo cuelga de ella', () => {
    const roots = buildForest(conEmpresa())
    expect(roots.map(r => r.person.name)).toEqual(['Acme S.A'])
    expect(roots[0].children.map(c => c.person.name)).toEqual(['Ana'])
  })

  it('no se le puede poner un jefe', () => {
    expect(dropRejection(99, 1, conEmpresa())).toMatch(/cabeza del organigrama/)
  })

  it('siempre acepta que le suelten a alguien encima', () => {
    // Es el equivalente a quitar el manager: reportar directo a la empresa.
    expect(dropRejection(3, 99, conEmpresa())).toBeNull()
  })
})

describe('teamSizes', () => {
  it('cuenta a todo el que cuelga por debajo, no solo los directos', () => {
    const sizes = teamSizes(empresa())
    expect(sizes.get(1)).toBe(2) // María y Pedro
    expect(sizes.get(2)).toBe(1)
    expect(sizes.get(4)).toBe(0)
  })
})
