import api from './client'
import type { WorkHour, PaginatedResponse } from '../types'

export const workHourService = {
  getAll: async (params?: { user_id?: string; start_date?: string; end_date?: string; page?: number; limit?: number; company_id?: number }) => {
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
  reject: async (ids: number[], reason: string) => {
    const { data } = await api.post('/work-hours/reject', { ids, reason })
    return data
  },
  getSummary: async (params?: { company_id?: number; user_id?: string }) => {
    const { data } = await api.get<{ total_hours: number; approved_hours: number; pending_hours: number; rejected_hours: number }>('/work-hours/summary', { params })
    return data
  },
  getPending: async (params?: { company_id?: number; user_id?: string }) => {
    const { data } = await api.get<WorkHour[]>('/work-hours/pending', { params })
    return data
  },
}
