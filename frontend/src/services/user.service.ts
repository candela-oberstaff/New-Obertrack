import api from './client'
import type { User, PaginatedResponse } from '../types'

export const userService = {
  getAll: async (params?: { role?: string; page?: number; limit?: number }) => {
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
  assignToManager: async (professionalId: number, managerId: number | null) => {
    const { data } = await api.post<User>(`/users/${professionalId}/assign-manager`, { manager_id: managerId })
    return data
  },
  promoteToManager: async (userId: number) => {
    const { data } = await api.post<User>(`/users/${userId}/promote-manager`)
    return data
  },
}
