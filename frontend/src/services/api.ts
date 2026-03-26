import axios from 'axios'
import type { User, Task, WorkHour, PaginatedResponse, CreateTaskInput, Board, CreateBoardInput } from '../types'

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
  register: async (userData: { name: string; email: string; password: string; user_type?: string; company_name?: string; empleador_id?: number }) => {
    const { data } = await api.post<{ user: User; access_token: string }>('/auth/register', userData)
    return data
  },
  me: async () => {
    const { data } = await api.get<User>('/auth/me')
    return data
  },
  getEmployers: async () => {
    const { data } = await api.get<{ id: number; name: string; company_name: string }[]>('/auth/employers')
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

export const taskService = {
  getAll: async (params?: { status?: string; priority?: string; page?: number; limit?: number }) => {
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
  getUsers: async (params?: { user_type?: string; is_active?: string; is_manager?: string; page?: number; limit?: number }) => {
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
  resetPassword: async (id: number, newPassword: string) => {
    const { data } = await api.post(`/admin/users/${id}/reset-password`, { new_password: newPassword })
    return data
  },
}

export const boardService = {
  getAll: async () => {
    const { data } = await api.get<Board[]>('/boards')
    return data
  },
  getById: async (id: number) => {
    const { data } = await api.get<Board>(`/boards/${id}`)
    return data
  },
  create: async (boardData: CreateBoardInput) => {
    const { data } = await api.post<Board>('/boards', boardData)
    return data
  },
  update: async (id: number, boardData: CreateBoardInput) => {
    const { data } = await api.put<Board>(`/boards/${id}`, boardData)
    return data
  },
  delete: async (id: number) => {
    await api.delete(`/boards/${id}`)
  },
  addPhase: async (boardId: number, phase: { name: string; color?: string }) => {
    const { data } = await api.post<Board>(`/boards/${boardId}/phases`, phase)
    return data
  },
  removePhase: async (boardId: number, phaseId: number) => {
    const { data } = await api.delete<Board>(`/boards/${boardId}/phases/${phaseId}`)
    return data
  },
  reorderPhases: async (boardId: number, phaseIds: number[]) => {
    const { data } = await api.put<Board>(`/boards/${boardId}/phases/reorder`, { phase_ids: phaseIds })
    return data
  },
}

export interface UploadResponse {
  url: string
  filename: string
  size: number
  type: string
}

export const uploadService = {
  upload: async (file: File): Promise<UploadResponse> => {
    const formData = new FormData()
    formData.append('file', file)
    const { data } = await api.post<UploadResponse>('/uploads', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
    return data
  },
}

export interface Notification {
  id: number
  user_id: number
  type: string
  title: string
  message: string
  data?: string
  read_at?: string
  created_at: string
}

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

export interface Channel {
  id: number
  name: string
  description: string
  type: 'public' | 'private'
  created_by: number
  unread_count: number
  created_at: string
}

export interface ChannelMessage {
  id: number
  channel_id: number
  user_id: number
  content: string
  attachment?: string
  file_name?: string
  file_size?: number
  file_type?: string
  is_edited?: boolean
  is_deleted?: boolean
  is_pinned?: boolean
  parent_id?: number
  reactions?: MessageReaction[]
  created_at: string
  user?: User
  tempId?: string
}

export interface MessageReaction {
  id: number
  message_id: number
  user_id: number
  emoji: string
  user?: User
}

export interface UserStatus {
  user_id: number
  status: 'online' | 'away' | 'offline'
  last_seen: string
}

export interface DMChannel extends Channel {
  recipient?: User
}

export const channelService = {
  getChannels: async (): Promise<Channel[]> => {
    const { data } = await api.get<Channel[]>('/channels')
    return data
  },
  getAllUsers: async (): Promise<User[]> => {
    const { data } = await api.get<User[]>('/channels/all-users')
    return data
  },
  createChannel: async (channel: { name: string; description?: string; type: string; member_ids?: number[] }) => {
    const { data } = await api.post<Channel>('/channels', channel)
    return data
  },
  getChannel: async (id: number) => {
    const { data } = await api.get<Channel>(`/channels/${id}`)
    return data
  },
  updateChannel: async (id: number, channel: { name?: string; description?: string }) => {
    const { data } = await api.put<Channel>(`/channels/${id}`, channel)
    return data
  },
  deleteChannel: async (id: number) => {
    await api.delete(`/channels/${id}`)
  },
  getMessages: async (channelId: number): Promise<ChannelMessage[]> => {
    const { data } = await api.get<ChannelMessage[]>(`/channels/${channelId}/messages`)
    return data
  },
  sendMessage: async (channelId: number, message: { content: string; attachment?: string; file_name?: string }) => {
    const { data } = await api.post<ChannelMessage>(`/channels/${channelId}/messages`, message)
    return data
  },
  editMessage: async (channelId: number, messageId: number, content: string) => {
    const { data } = await api.put<ChannelMessage>(`/channels/${channelId}/messages/${messageId}`, { content })
    return data
  },
  deleteMessage: async (channelId: number, messageId: number) => {
    await api.delete(`/channels/${channelId}/messages/${messageId}`)
  },
  getMembers: async (channelId: number) => {
    const { data } = await api.get<User[]>(`/channels/${channelId}/members`)
    return data
  },
  addMember: async (channelId: number, userId: number) => {
    await api.post(`/channels/${channelId}/members`, { user_id: userId })
  },
  removeMember: async (channelId: number, userId: number) => {
    await api.delete(`/channels/${channelId}/members`, { data: { user_id: userId } })
  },
  joinChannel: async (channelId: number) => {
    await api.post(`/channels/${channelId}/join`)
  },
  leaveChannel: async (channelId: number) => {
    await api.post(`/channels/${channelId}/leave`)
  },
  pinMessage: async (channelId: number, messageId: number) => {
    await api.post(`/channels/${channelId}/pin/${messageId}`)
  },
  unpinMessage: async (channelId: number, messageId: number) => {
    await api.post(`/channels/${channelId}/unpin/${messageId}`)
  },
  getPinnedMessages: async (channelId: number): Promise<ChannelMessage[]> => {
    const { data } = await api.get<ChannelMessage[]>(`/channels/${channelId}/pinned`)
    return data
  },
  addReaction: async (channelId: number, messageId: number, emoji: string) => {
    const { data } = await api.post<MessageReaction>(`/channels/${channelId}/messages/${messageId}/reactions`, { emoji })
    return data
  },
  removeReaction: async (channelId: number, messageId: number, emoji: string) => {
    await api.delete(`/channels/${channelId}/messages/${messageId}/reactions`, { data: { emoji } })
  },
  getThreadReplies: async (channelId: number, messageId: number): Promise<ChannelMessage[]> => {
    const { data } = await api.get<ChannelMessage[]>(`/channels/${channelId}/messages/${messageId}/replies`)
    return data
  },
  sendThreadReply: async (channelId: number, messageId: number, content: string) => {
    const { data } = await api.post<ChannelMessage>(`/channels/${channelId}/messages/${messageId}/replies`, { content })
    return data
  },
  starMessage: async (messageId: number) => {
    await api.post(`/channels/star/${messageId}`)
  },
  unstarMessage: async (messageId: number) => {
    await api.delete(`/channels/star/${messageId}`)
  },
  getStarredMessages: async (): Promise<ChannelMessage[]> => {
    const { data } = await api.get<ChannelMessage[]>('/channels/starred')
    return data
  },
  updateStatus: async (status: 'online' | 'away' | 'offline') => {
    await api.post('/channels/status', { status })
  },
  getStatuses: async (userIds: number[]): Promise<UserStatus[]> => {
    const { data } = await api.get<UserStatus[]>('/channels/statuses', { params: { user_ids: userIds.join(',') } })
    return data
  },
  searchMessages: async (channelId: number, query: string): Promise<ChannelMessage[]> => {
    const { data } = await api.get<ChannelMessage[]>(`/channels/${channelId}/search`, { params: { q: query } })
    return data
  },
  createDM: async (recipientId: number): Promise<DMChannel> => {
    const { data } = await api.post<DMChannel>('/channels/dm', { recipient_id: recipientId })
    return data
  },
}

export default api
