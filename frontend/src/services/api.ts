import axios from 'axios'
import type { User, Task, WorkHour, PaginatedResponse } from '../types'

const api = axios.create({
  baseURL: '/api',
})

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token')
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

export const authService = {
  login: async (email: string, password: string) => {
    const { data } = await api.post<{ user: User; access_token: string }>('/auth/login', { email, password })
    return data
  },
  register: async (userData: { name: string; email: string; password: string; user_type?: string; company_name?: string }) => {
    const { data } = await api.post<{ user: User; access_token: string }>('/auth/register', userData)
    return data
  },
  me: async () => {
    const { data } = await api.get<User>('/auth/me')
    return data
  },
}

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
  getEmployees: async () => {
    const { data } = await api.get<User[]>('/users/employees')
    return data
  },
}

export const taskService = {
  getAll: async (params?: { status?: string; priority?: string; page?: number; limit?: number }) => {
    const { data } = await api.get<PaginatedResponse<Task>>('/tasks', { params })
    return data
  },
  getById: async (id: number) => {
    const { data } = await api.get<Task>(`/tasks/${id}`)
    return data
  },
  create: async (taskData: Partial<Task>) => {
    const { data } = await api.post<Task>('/tasks', taskData)
    return data
  },
  update: async (id: number, taskData: Partial<Task>) => {
    const { data } = await api.put<Task>(`/tasks/${id}`, taskData)
    return data
  },
  delete: async (id: number) => {
    await api.delete(`/tasks/${id}`)
  },
  toggleCompletion: async (id: number) => {
    const { data } = await api.post<Task>(`/tasks/${id}/toggle-completion`)
    return data
  },
  addComment: async (id: number, content: string) => {
    const { data } = await api.post(`/tasks/${id}/comments`, { content })
    return data
  },
}

export const workHourService = {
  getAll: async (params?: { user_id?: string; start_date?: string; end_date?: string; page?: number; limit?: number }) => {
    const { data } = await api.get<PaginatedResponse<WorkHour>>('/work-hours', { params })
    return data
  },
  create: async (workHourData: Partial<WorkHour>) => {
    const { data } = await api.post<WorkHour>('/work-hours', workHourData)
    return data
  },
  update: async (id: number, workHourData: Partial<WorkHour>) => {
    const { data } = await api.put<WorkHour>(`/work-hours/${id}`, workHourData)
    return data
  },
  approve: async (ids: number[]) => {
    const { data } = await api.post('/work-hours/approve', { ids })
    return data
  },
  getSummary: async () => {
    const { data } = await api.get<{ total_hours: number; approved_hours: number; pending_hours: number }>('/work-hours/summary')
    return data
  },
  getPending: async () => {
    const { data } = await api.get<WorkHour[]>('/work-hours/pending')
    return data
  },
}

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
  getUsers: async (params?: { user_type?: string; is_active?: string; page?: number; limit?: number }) => {
    const { data } = await api.get('/admin/users', { params })
    return data
  },
  createUser: async (userData: any) => {
    const { data } = await api.post('/admin/users', userData)
    return data
  },
  updateUser: async (id: number, userData: any) => {
    const { data } = await api.put(`/admin/users/${id}`, userData)
    return data
  },
  deleteUser: async (id: number) => {
    await api.delete(`/admin/users/${id}`)
  },
}

export default api
