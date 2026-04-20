import api from './client'

export const adminService = {
  getDashboard: async () => {
    const { data } = await api.get('/admin/dashboard')
    return data
  },
  getCompanies: async () => {
    const { data } = await api.get('/admin/companies')
    return data
  },
  getInactiveUsers: async (days: number = 7) => {
    const { data } = await api.get('/admin/inactive-users', { params: { days } })
    return data
  },
  getRecentActivity: async () => {
    const { data } = await api.get('/admin/recent-activity')
    return data
  },
  getStats: async () => {
    const { data } = await api.get('/admin/stats')
    return data
  },
  getUsers: async (params?: { user_type?: string; is_active?: string; is_manager?: string; page?: number; limit?: number }) => {
    const { data } = await api.get('/admin/users', { params })
    return data
  },
  createUser: async (userData: Record<string, unknown>) => {
    const { data } = await api.post('/admin/users', userData)
    return data
  },
  updateUser: async (id: number, userData: Record<string, unknown>) => {
    const { data } = await api.put(`/admin/users/${id}`, userData)
    return data
  },
  deleteUser: async (id: number) => {
    await api.delete(`/admin/users/${id}`)
  },
  resetPassword: async (id: number, newPassword: string) => {
    const { data } = await api.post(`/admin/users/${id}/reset-password`, { new_password: newPassword })
    return data
  },
}
