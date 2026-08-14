import api from './client'
import type { User, PaginatedResponse } from '../types'

export const userService = {
  getAll: async (params?: { role?: string; page?: number; limit?: number; q?: string; company_id?: number }) => {
    const { data } = await api.get<PaginatedResponse<User>>('/users', { params })
    return data
  },
  getById: async (id: number) => {
    const { data } = await api.get<User>(`/users/${id}`)
    return data
  },
  create: async (userData: Partial<User> & { password?: string }) => {
    const { data } = await api.post<User>('/users', userData)
    return data
  },
  update: async (id: number, userData: Partial<User>) => {
    const { data } = await api.put<User>(`/users/${id}`, userData)
    return data
  },
  delete: async (id: number) => {
    await api.delete(`/users/${id}`)
  },
  changePassword: async (id: number, currentPassword: string, newPassword: string) => {
    await api.post(`/users/${id}/change-password`, {
      current_password: currentPassword,
      new_password: newPassword,
    })
  },
  getEmployees: async () => {
    const { data } = await api.get<User[]>('/users/employees')
    return data
  },
  getMyTeam: async () => {
    const { data } = await api.get<User[]>('/users/my-team')
    return data
  },
  // companyId lo manda el organigrama del superadmin para clavar el cambio en la
  // empresa que se está mirando: un profesional puede tener empleo en varias, y
  // sin esto el vínculo acabaría escrito en la que resulte ser su activa.
  assignToManager: async (professionalId: number, managerId: number | null, companyId?: number) => {
    const { data } = await api.post<User>(`/users/${professionalId}/assign-manager`, {
      manager_id: managerId,
      ...(companyId ? { company_id: companyId } : {}),
    })
    return data
  },
  // Fija el nivel en la cadena de mando. Los dos flags son opcionales e
  // independientes: omitir uno lo deja como está. Sin ninguno, alterna manager
  // (comportamiento histórico del botón "Promover / Quitar rol").
  promoteToManager: async (userId: number, isManager?: boolean, isSupervisor?: boolean) => {
    const body: Record<string, boolean> = {}
    if (typeof isManager === 'boolean') body.is_manager = isManager
    if (typeof isSupervisor === 'boolean') body.is_supervisor = isSupervisor
    const { data } = await api.post<User>(
      `/users/${userId}/promote-manager`,
      Object.keys(body).length > 0 ? body : undefined,
    )
    return data
  },
  // Mueve todos los reportes activos del manager (todas las empresas) a otro
  // manager, o los desasigna si newManagerId es null. Devuelve { reassigned: n }.
  reassignTeam: async (managerId: number, newManagerId: number | null) => {
    const { data } = await api.post<{ reassigned: number }>(`/users/${managerId}/reassign-team`, { new_manager_id: newManagerId })
    return data
  },
}
