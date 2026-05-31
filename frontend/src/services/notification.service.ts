import api from './client'
import type { Notification } from '../types'

export const notificationService = {
  getAll: async (): Promise<Notification[]> => {
    const { data } = await api.get<Notification[]>('/notifications')
    return data
  },
  getUnreadCount: async (): Promise<number> => {
    const { data } = await api.get<{ count: number }>('/notifications/unread-count')
    return data.count
  },
  markAsRead: async (id: number) => {
    await api.post(`/notifications/${id}/read`)
  },
  markAllAsRead: async () => {
    await api.post('/notifications/read-all')
  },
}
