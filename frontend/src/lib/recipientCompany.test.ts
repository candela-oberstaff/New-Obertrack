import { describe, it, expect } from 'vitest'
import {
  buildCompanyIndex,
  companyIdOf,
  companyNameOf,
  companyOptions,
  matchesCompany,
  NO_COMPANY,
  type CompanyAwareUser,
} from './recipientCompany'

// Padrón mínimo con los cuatro casos que se dan de verdad: una empresa cuyo
// nombre comercial difiere del nombre de la cuenta, sus profesionales, otra
// empresa, y gente de casa que no cuelga de ninguna.
const empresaA: CompanyAwareUser = { id: 10, name: 'Osvell Empresa', user_type: 'empleador', company_name: 'Accés Vertical' }
const empresaB: CompanyAwareUser = { id: 20, name: 'Globex Corp', user_type: 'empleador', company_name: 'Globex Corp' }
const lidia: CompanyAwareUser = { id: 11, name: 'Lidia González', user_type: 'profesional', empleador_id: 10 }
const marco: CompanyAwareUser = { id: 12, name: 'Marco Ruiz', user_type: 'profesional', empleador_id: 10 }
const sofia: CompanyAwareUser = { id: 21, name: 'Sofía Navarro', user_type: 'profesional', empleador_id: 20 }
const superadmin: CompanyAwareUser = { id: 1, name: 'Carlos', user_type: 'superadmin' }

const padron = [empresaA, empresaB, lidia, marco, sofia, superadmin]
const index = buildCompanyIndex(padron)

describe('recipientCompany · a qué empresa pertenece cada quien', () => {
  it('la cuenta empleador es su propia empresa', () => {
    expect(companyIdOf(empresaA)).toBe(10)
    expect(companyNameOf(empresaA, index)).toBe('Accés Vertical')
  })

  it('el profesional hereda el nombre de su empleador, que no está en su propia fila', () => {
    // Es el punto entero de este módulo: lidia no guarda "Accés Vertical" en
    // ningún campo, solo el id 10.
    expect(companyNameOf(lidia, index)).toBe('Accés Vertical')
  })

  it('prefiere el nombre comercial al nombre de la cuenta', () => {
    // La cuenta se llama "Osvell Empresa" pero el cliente es "Accés Vertical";
    // filtrar por el nombre de la cuenta no le diría nada a nadie.
    expect(companyNameOf(empresaA, index)).not.toBe('Osvell Empresa')
  })

  it('quien no cuelga de ninguna empresa se queda sin nombre, no con uno inventado', () => {
    expect(companyNameOf(superadmin, index)).toBe('')
    expect(companyIdOf(superadmin)).toBe(0)
  })
})

describe('recipientCompany · el filtro que acota un envío', () => {
  it('"all" deja pasar a todo el mundo', () => {
    expect(padron.every(u => matchesCompany(u, index, 'all'))).toBe(true)
  })

  it('acota a la empresa elegida, incluyendo a su cuenta empleador', () => {
    const deAcces = padron.filter(u => matchesCompany(u, index, 'Accés Vertical'))
    expect(deAcces.map(u => u.id).sort((a, b) => a - b)).toEqual([10, 11, 12])
  })

  it('no se lleva por delante a la gente de otra empresa', () => {
    expect(matchesCompany(sofia, index, 'Accés Vertical')).toBe(false)
    expect(matchesCompany(superadmin, index, 'Accés Vertical')).toBe(false)
  })

  it('«Sin empresa» encuentra a quien no pertenece a ninguna', () => {
    const sueltos = padron.filter(u => matchesCompany(u, index, NO_COMPANY))
    expect(sueltos).toEqual([superadmin])
  })
})

describe('recipientCompany · el desplegable', () => {
  it('ordena por cuánta gente tiene y cuenta a todos, empleador incluido', () => {
    const opts = companyOptions(padron, index, 'all')
    expect(opts[0].label).toBe('Accés Vertical (3)')
    expect(opts[1].label).toBe('Globex Corp (2)')
  })

  it('ofrece «Sin empresa» solo cuando hay alguien así', () => {
    expect(companyOptions(padron, index, 'all').some(o => o.value === NO_COMPANY)).toBe(true)
    const soloEmpresa = [empresaA, lidia]
    expect(
      companyOptions(soloEmpresa, buildCompanyIndex(soloEmpresa), 'all').some(o => o.value === NO_COMPANY),
    ).toBe(false)
  })

  it('conserva la empresa elegida aunque el filtro de rol la deje en cero', () => {
    // Sin esto el desplegable saltaría solo a otro valor y el listado cambiaría
    // sin que nadie lo hubiera tocado — con un envío masivo a punto de salir.
    const soloGlobex = [empresaB, sofia]
    const opts = companyOptions(soloGlobex, index, 'Accés Vertical')
    expect(opts.find(o => o.value === 'Accés Vertical')?.label).toBe('Accés Vertical (0)')
  })
})
