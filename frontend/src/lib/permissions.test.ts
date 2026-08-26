import { describe, it, expect } from 'vitest'
import { roleLabel, roleBadgeClass, hierarchyLabel } from './permissions'

// Manager y supervisor no son tipos de cuenta: son niveles marcados encima de un
// profesional. Cada pantalla que lo resolvía por su cuenta se dejaba alguno fuera —el
// listado general mostraba "Profesional" a un supervisor, y otras tres pantallas
// mostraban "manager" con `is_manager ? 'manager' : user_type`, que no contempla el
// nivel de arriba—. Esto fija la regla en un solo sitio.

describe('roleLabel', () => {
  it('muestra el nivel más alto de un profesional', () => {
    // Un supervisor es TAMBIÉN manager: decir "Manager" de quien supervisa se queda
    // corto, así que gana el de arriba.
    expect(roleLabel({ user_type: 'profesional', is_manager: true, is_supervisor: true })).toBe('Supervisor')
    expect(roleLabel({ user_type: 'profesional', is_manager: true })).toBe('Manager')
    expect(roleLabel({ user_type: 'profesional' })).toBe('Profesional')
  })

  it('nombra los tipos de cuenta como se leen en pantalla', () => {
    expect(roleLabel({ user_type: 'empleador' })).toBe('Empresa')
    expect(roleLabel({ user_type: 'customer_success' })).toBe('Customer Success')
    expect(roleLabel({ user_type: 'analista_it' })).toBe('Analista IT')
    expect(roleLabel({ user_type: 'superadmin' })).toBe('Superadmin')
  })

  it('no confunde el nivel de un customer success con el de un profesional', () => {
    // En customer_success la bandera de manager significa otra cosa: la jerarquía de
    // soporte. Ahí el rol que importa es el tipo de cuenta.
    expect(roleLabel({ user_type: 'customer_success', is_manager: true })).toBe('Customer Success')
  })

  it('sobrevive a lo que no conoce', () => {
    expect(roleLabel(null)).toBe('—')
    expect(roleLabel({ user_type: 'rol_futuro' })).toBe('rol_futuro')
  })
})

describe('roleBadgeClass', () => {
  it('agrupa a quien tiene gente a cargo bajo el mismo color', () => {
    // El color agrupa y el texto distingue: tres tonos para tres niveles obligarían a
    // aprenderse una leyenda.
    expect(roleBadgeClass({ user_type: 'profesional', is_supervisor: true, is_manager: true })).toBe('manager')
    expect(roleBadgeClass({ user_type: 'profesional', is_manager: true })).toBe('manager')
    expect(roleBadgeClass({ user_type: 'profesional' })).toBe('profesional')
    expect(roleBadgeClass({ user_type: 'empleador' })).toBe('empleador')
  })
})

describe('hierarchyLabel', () => {
  it('devuelve null para quien no tiene gente a cargo', () => {
    // Cada pantalla decide entonces si no pinta nada o cae a otra etiqueta; por eso
    // esta función NO inventa un "Profesional" por su cuenta.
    expect(hierarchyLabel({})).toBeNull()
    expect(hierarchyLabel({ is_manager: true })).toBe('Manager')
    expect(hierarchyLabel({ is_supervisor: true })).toBe('Supervisor')
  })
})
