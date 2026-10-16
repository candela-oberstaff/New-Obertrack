import api from './client'

// Servidor MCP de solo lectura (asistentes de IA). Backend: internal/mcp.

/** Clave de react-query del estado: la comparten el menú y la página. */
export const MCP_STATUS_KEY = ['mcp', 'status'] as const

export interface McpStatus {
  enabled: boolean
  paused: boolean
  server_url: string
  role: string
  role_label: string
  scope?: string
  quota: { daily_calls: number; max_rows: number }
  used_today: number
}

export interface McpConnection {
  id: number
  client_name: string
  personal: boolean
  created_at: string
  last_used_at: string | null
  expires_at: string
  revoked_at?: string | null
  revoke_reason?: string
  ip?: string
  user_id?: number
  user_name?: string
  user_email?: string
}

export interface McpQuota {
  role_name: string
  label: string
  enabled: boolean
  daily_calls: number
  max_rows: number
}

export interface McpSettings {
  paused: boolean
  minimal_data: boolean
  extra_redirect_domains: string
}

export interface McpAlert {
  id: number
  user_id: number
  user_name: string
  user_email: string
  kind: 'quota_80' | 'quota_100' | 'token_reuse' | string
  detail: string
  created_at: string
}

export interface McpOverview {
  settings: McpSettings
  quotas: McpQuota[]
  db_status: { mode: 'dedicated' | 'general' | 'fallback'; detail: string; since: string }
  alerts: McpAlert[]
  server_url: string
  totals: { active_connections: number; calls_today: number; users_today: number }
}

export interface McpUsage {
  days: number
  by_user_day: {
    day: string
    user_id: number
    user_name: string
    user_email: string
    calls: number
    errors: number
    rows: number
    avg_ms: number
  }[]
  by_tool: { tool: string; calls: number; errors: number; avg_ms: number }[]
}

export const mcpService = {
  getStatus: async () => (await api.get<McpStatus>('/mcp/status')).data,
  listConnections: async () => (await api.get<McpConnection[]>('/mcp/connections')).data,
  revokeConnection: async (id: number) => { await api.delete(`/mcp/connections/${id}`) },
  listTokens: async () => (await api.get<McpConnection[]>('/mcp/tokens')).data,
  createToken: async (name: string) =>
    (await api.post<{ token: string; connection: McpConnection }>('/mcp/tokens', { name })).data,
  revokeToken: async (id: number) => { await api.delete(`/mcp/tokens/${id}`) },

  // Panel "Uso de IA" (solo superadmin).
  getOverview: async () => (await api.get<McpOverview>('/admin/mcp/overview')).data,
  listAllConnections: async (status: 'active' | 'all', search: string) =>
    (await api.get<McpConnection[]>('/admin/mcp/connections', { params: { status, search: search || undefined } })).data,
  adminRevoke: async (id: number, reason: string) => {
    await api.post(`/admin/mcp/connections/${id}/revoke`, { reason })
  },
  getUsage: async (days: number) => (await api.get<McpUsage>('/admin/mcp/usage', { params: { days } })).data,
  updateQuota: async (q: McpQuota) =>
    (await api.put<McpQuota>(`/admin/mcp/quotas/${q.role_name}`, {
      enabled: q.enabled, daily_calls: q.daily_calls, max_rows: q.max_rows,
    })).data,
  updateSettings: async (s: McpSettings) => (await api.put<McpSettings>('/admin/mcp/settings', s)).data,
  markAlertsSeen: async (ids?: number[]) => { await api.post('/admin/mcp/alerts/seen', { ids }) },
  purge: async () => (await api.post<{ connections: number; tokens: number; codes: number }>('/admin/mcp/purge')).data,
}
