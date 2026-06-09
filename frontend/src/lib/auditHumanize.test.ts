import { describeAudit, moduleLabel, originLabel } from './auditHumanize'
import type { AuditLog } from '../services/audit.service'

function log(partial: Partial<AuditLog>): AuditLog {
  return {
    id: 1, kind: 'activity', actor_email: '', actor_role: '', action: '', module: '',
    method: 'GET', path: '/', status: 200, success: true, ip: '', user_agent: '',
    created_at: '', ...partial,
  } as AuditLog
}

describe('auditHumanize', () => {
  it('describes auth events naturally', () => {
    expect(describeAudit(log({ action: 'auth.login' }))).toBe('Inició sesión')
    expect(describeAudit(log({ action: 'auth.logout' }))).toBe('Cerró sesión')
  })

  it('phrases user actions per verb and entity', () => {
    expect(describeAudit(log({ action: 'tasks.created', entity_type: 'tasks', entity_id: '7' })))
      .toBe('Creó tarea #7')
    expect(describeAudit(log({ action: 'work_hours.approve', entity_type: 'work_hours' })))
      .toBe('Aprobó horas')
  })

  it('uses noun phrasing for automatic data changes', () => {
    expect(describeAudit(log({ kind: 'data', action: 'boards.deleted', entity_type: 'boards', entity_id: '3' })))
      .toBe('Eliminación de tablero #3')
  })

  it('labels module and origin', () => {
    expect(moduleLabel('work-hours')).not.toBe('—')
    expect(moduleLabel(undefined)).toBe('—')
    expect(originLabel(log({ kind: 'data' }))).toBe('Sistema (automático)')
    expect(originLabel(log({ kind: 'activity' }))).toBe('Acción de usuario')
  })
})
