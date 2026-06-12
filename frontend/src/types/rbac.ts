// Roles personalizados y grupos (equipos) por empresa.

export type PermissionLevel = 'none' | 'view' | 'edit'

export interface CompanyRole {
  id: number
  tenant_id: number
  name: string
  description: string
  /** Objeto JSON serializado: {"tasks":"edit","hours":"view",...} */
  permissions: string
  created_by: number
  user_count: number
  created_at: string
  updated_at: string
}

export interface CompanyGroup {
  id: number
  tenant_id: number
  name: string
  description: string
  created_by: number
  member_count: number
  created_at: string
  updated_at: string
}
