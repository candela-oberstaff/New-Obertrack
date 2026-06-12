import api from './client'
import type { CompanyRole, CompanyGroup, User } from '../types'

// companyId solo aplica para superadmins (eligen empresa); las cuentas empresa
// quedan acotadas a su propio tenant en el backend.
const scope = (companyId?: number | null) =>
  companyId ? { company_id: companyId } : undefined

export interface RolePayload {
  name?: string
  description?: string
  permissions?: string
}

export interface GroupPayload {
  name?: string
  description?: string
}

export const rbacService = {
  // ── Roles ──────────────────────────────────────────────────────────────────
  listRoles: async (companyId?: number | null): Promise<CompanyRole[]> => {
    const { data } = await api.get<{ data: CompanyRole[] }>('/roles', { params: scope(companyId) })
    return data.data || []
  },
  createRole: async (payload: RolePayload, companyId?: number | null): Promise<CompanyRole> => {
    const { data } = await api.post<CompanyRole>('/roles', payload, { params: scope(companyId) })
    return data
  },
  updateRole: async (id: number, payload: RolePayload, companyId?: number | null): Promise<CompanyRole> => {
    const { data } = await api.put<CompanyRole>(`/roles/${id}`, payload, { params: scope(companyId) })
    return data
  },
  deleteRole: async (id: number, companyId?: number | null) => {
    await api.delete(`/roles/${id}`, { params: scope(companyId) })
  },
  getRoleUsers: async (id: number, companyId?: number | null): Promise<User[]> => {
    const { data } = await api.get<{ data: User[] }>(`/roles/${id}/users`, { params: scope(companyId) })
    return data.data || []
  },
  assignRole: async (id: number, userIds: number[], companyId?: number | null) => {
    await api.post(`/roles/${id}/users`, { user_ids: userIds }, { params: scope(companyId) })
  },
  unassignRole: async (id: number, userId: number, companyId?: number | null) => {
    await api.delete(`/roles/${id}/users`, { data: { user_id: userId }, params: scope(companyId) })
  },

  /** Roles y grupos de un usuario concreto del tenant. */
  getUserRBAC: async (userId: number, companyId?: number | null): Promise<{ roles: CompanyRole[]; groups: CompanyGroup[] }> => {
    const { data } = await api.get<{ roles: CompanyRole[]; groups: CompanyGroup[] }>(`/rbac/users/${userId}`, { params: scope(companyId) })
    return { roles: data.roles || [], groups: data.groups || [] }
  },

  // ── Grupos ─────────────────────────────────────────────────────────────────
  listGroups: async (companyId?: number | null): Promise<CompanyGroup[]> => {
    const { data } = await api.get<{ data: CompanyGroup[] }>('/groups', { params: scope(companyId) })
    return data.data || []
  },
  createGroup: async (payload: GroupPayload, companyId?: number | null): Promise<CompanyGroup> => {
    const { data } = await api.post<CompanyGroup>('/groups', payload, { params: scope(companyId) })
    return data
  },
  updateGroup: async (id: number, payload: GroupPayload, companyId?: number | null): Promise<CompanyGroup> => {
    const { data } = await api.put<CompanyGroup>(`/groups/${id}`, payload, { params: scope(companyId) })
    return data
  },
  deleteGroup: async (id: number, companyId?: number | null) => {
    await api.delete(`/groups/${id}`, { params: scope(companyId) })
  },
  getGroupMembers: async (id: number, companyId?: number | null): Promise<User[]> => {
    const { data } = await api.get<{ data: User[] }>(`/groups/${id}/members`, { params: scope(companyId) })
    return data.data || []
  },
  addGroupMembers: async (id: number, userIds: number[], companyId?: number | null) => {
    await api.post(`/groups/${id}/members`, { user_ids: userIds }, { params: scope(companyId) })
  },
  removeGroupMember: async (id: number, userId: number, companyId?: number | null) => {
    await api.delete(`/groups/${id}/members`, { data: { user_id: userId }, params: scope(companyId) })
  },
}
