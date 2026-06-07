import api from './client'
import type { Task, PaginatedResponse, CreateTaskInput } from '../types'

export const taskService = {
  getAll: async (params?: { 
    status?: string; 
    priority?: string; 
    page?: number; 
    limit?: number; 
    assignee_id?: string;
    board_id?: number;
    start_date?: string;
    end_date?: string;
  }) => {
    const { data } = await api.get<PaginatedResponse<Task>>('/tasks', { params })
    return data
  },
  getById: async (id: number) => {
    const { data } = await api.get<Task>(`/tasks/${id}`)
    return data
  },
  create: async (taskData: CreateTaskInput) => {
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
  addAttachment: async (id: number, file: File) => {
    const formData = new FormData()
    formData.append('file', file)
    const { data } = await api.post(`/tasks/${id}/attachments`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
    return data
  },
  deleteAttachment: async (taskId: number, attachmentId: number) => {
    await api.delete(`/tasks/${taskId}/attachments/${attachmentId}`)
  },
}
