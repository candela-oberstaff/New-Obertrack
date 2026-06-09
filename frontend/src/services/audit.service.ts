import api from './client'

export interface AuditLog {
  id: number
  kind: 'activity' | 'data'
  actor_id?: number | null
  actor_email: string
  actor_role: string
  tenant_id?: number | null
  action: string
  module: string
  entity_type?: string
  entity_id?: string
  changes?: string
  method: string
  path: string
  target_id?: string
  status: number
  success: boolean
  ip: string
  user_agent: string
  created_at: string
}

export interface AuditLogParams {
  page?: number
  limit?: number
  email?: string
  module?: string
  action?: string
  kind?: string
  entity_type?: string
  entity_id?: string
  success?: string
  start_date?: string
  end_date?: string
  q?: string
}

export const auditService = {
  getAuditLogs: async (params: AuditLogParams) => {
    const { data } = await api.get<{ data: AuditLog[]; total: number; page: number; limit: number }>(
      '/admin/audit-logs',
      { params },
    )
    return data
  },
}
