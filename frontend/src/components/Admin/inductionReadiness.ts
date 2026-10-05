import type { InductionBlock, InductionProgram } from '../../services/induction.service'

/**
 * Qué le falta a un programa o a un bloque para estar listo, dicho en el
 * idioma de quien arma la inducción. Es puro a propósito: el panel lo muestra
 * en la lista y en el editor (con el borrador sin guardar), y se prueba solo.
 *
 * - blocker: la inducción no funciona bien así (un bloque sin preguntas no se
 *   puede aprobar; un programa sin bloques no se emite).
 * - warning: funciona, pero casi seguro no es lo que se quiere.
 * - tip: opcional, se sugiere.
 */
export type IssueLevel = 'blocker' | 'warning' | 'tip'

export interface ReadinessIssue {
  level: IssueLevel
  text: string
}

/** Por debajo de este mínimo, aprobar el cuestionario casi no exige nada. */
export const LOW_PASSING_SCORE = 50

type BlockLike = Pick<InductionBlock, 'name' | 'question_count' | 'passing_score' | 'tutorial_id' | 'tutorial_visible'>

export interface ProgramDraft {
  isActive: boolean
  isDefault: boolean
  defaultPassingScore: number
  blocks: BlockLike[]
  companyCount: number
  hasCertificate: boolean
}

export function draftFromProgram(p: InductionProgram): ProgramDraft {
  return {
    isActive: p.is_active,
    isDefault: p.is_default,
    defaultPassingScore: p.default_passing_score,
    blocks: p.blocks ?? [],
    companyCount: p.company_ids?.length ?? p.company_count,
    hasCertificate: !!p.certificate_template_id,
  }
}

/** Avisos de un bloque suelto. fallbackPassingScore = el mínimo que hereda si no trae uno. */
export function blockIssues(b: BlockLike, fallbackPassingScore: number): ReadinessIssue[] {
  const issues: ReadinessIssue[] = []
  if (b.question_count === 0) {
    issues.push({ level: 'blocker', text: 'No tiene preguntas: nadie puede aprobarlo.' })
  }
  const passing = b.passing_score ?? fallbackPassingScore
  if (passing < LOW_PASSING_SCORE) {
    issues.push({ level: 'warning', text: `Se aprueba con solo ${passing}%.` })
  }
  if (b.tutorial_id && b.tutorial_visible) {
    issues.push({ level: 'warning', text: 'Su video está visible en Novedades: lo ve toda su audiencia.' })
  }
  return issues
}

export function programIssues(d: ProgramDraft): ReadinessIssue[] {
  const issues: ReadinessIssue[] = []
  if (d.blocks.length === 0) {
    issues.push({ level: 'blocker', text: 'No tiene bloques: agrega al menos uno.' })
  }
  for (const b of d.blocks) {
    for (const issue of blockIssues(b, d.defaultPassingScore)) {
      issues.push({ ...issue, text: `«${b.name}»: ${lowerFirst(issue.text)}` })
    }
  }
  if (!d.isActive) {
    issues.push({ level: 'warning', text: 'Está apagado: nadie lo recibe.' })
  } else if (!d.isDefault && d.companyCount === 0) {
    issues.push({ level: 'warning', text: 'No tiene empresas asignadas: nadie lo recibe.' })
  }
  if (!d.hasCertificate) {
    issues.push({ level: 'tip', text: 'No emite certificado al completarlo.' })
  }
  return issues
}

/** Listo = sin bloqueos ni advertencias (las sugerencias no cuentan). */
export function isReady(issues: ReadinessIssue[]): boolean {
  return !issues.some((i) => i.level !== 'tip')
}

function lowerFirst(s: string): string {
  return s.charAt(0).toLowerCase() + s.slice(1)
}
