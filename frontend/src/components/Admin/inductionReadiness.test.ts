import { describe, expect, it } from 'vitest'

import { blockIssues, isReady, programIssues, type ProgramDraft } from './inductionReadiness'

const block = (over: Partial<ProgramDraft['blocks'][number]> = {}) => ({
  name: 'Bienvenida',
  question_count: 3,
  passing_score: null,
  ...over,
})

const draft = (over: Partial<ProgramDraft> = {}): ProgramDraft => ({
  isActive: true,
  isDefault: true,
  defaultPassingScore: 70,
  blocks: [block()],
  recipientCount: 0,
  hasCertificate: true,
  ...over,
})

describe('programIssues', () => {
  it('un programa completo está listo', () => {
    const issues = programIssues(draft())
    expect(issues).toEqual([])
    expect(isReady(issues)).toBe(true)
  })

  it('sin bloques es un bloqueo', () => {
    const issues = programIssues(draft({ blocks: [] }))
    expect(issues[0]).toMatchObject({ level: 'blocker' })
    expect(isReady(issues)).toBe(false)
  })

  it('nombra el bloque sin preguntas', () => {
    const issues = programIssues(draft({ blocks: [block({ name: 'Seguridad', question_count: 0 })] }))
    expect(issues).toContainEqual({ level: 'blocker', text: '«Seguridad»: no tiene preguntas: nadie puede aprobarlo.' })
  })

  it('el mínimo bajo cuenta el heredado del programa', () => {
    expect(programIssues(draft({ defaultPassingScore: 30 }))[0]).toMatchObject({ level: 'warning', text: expect.stringContaining('30%') })
    // El propio del bloque manda sobre el del programa.
    expect(programIssues(draft({ defaultPassingScore: 30, blocks: [block({ passing_score: 80 })] }))).toEqual([])
  })

  it('un programa no por defecto sin empresas no lo recibe nadie', () => {
    expect(programIssues(draft({ isDefault: false }))[0].text).toContain('No tiene empresas ni profesionales')
    expect(programIssues(draft({ isDefault: false, recipientCount: 2 }))).toEqual([])
  })

  it('apagado se avisa una sola vez, aunque tampoco tenga empresas', () => {
    const issues = programIssues(draft({ isDefault: false, isActive: false }))
    expect(issues.map((i) => i.text)).toEqual(['Está apagado: nadie lo recibe.'])
  })

  it('sin certificado es solo una sugerencia', () => {
    const issues = programIssues(draft({ hasCertificate: false }))
    expect(issues).toEqual([{ level: 'tip', text: 'No emite certificado al completarlo.' }])
    expect(isReady(issues)).toBe(true)
  })
})

describe('blockIssues', () => {
  it('un bloque con preguntas y mínimo razonable no tiene avisos', () => {
    expect(blockIssues(block(), 70)).toEqual([])
  })
})
