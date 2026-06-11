import api from './client'
import type { User } from '../types'
import type { Channel, Message, DMChannel, MessageReaction, UserStatus } from '../types/chat'

export const channelService = {
  getChannels: async (companyId?: number | null): Promise<Channel[]> => {
    const params = companyId ? { company_id: companyId } : undefined
    const { data } = await api.get<Channel[]>('/channels', { params })
    return data
  },
  getAllUsers: async (companyId?: number | null): Promise<User[]> => {
    const params = companyId ? { company_id: companyId } : undefined
    const { data } = await api.get<User[]>('/channels/all-users', { params })
    return data
  },
  createChannel: async (channel: { name: string; description?: string; type: string; member_ids?: number[] }, companyId?: number | null) => {
    const params = companyId ? { company_id: companyId } : undefined
    const { data } = await api.post<Channel>('/channels', channel, { params })
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
  getMessages: async (channelId: number, before?: number): Promise<Message[]> => {
    const params = before ? { before } : undefined
    const { data } = await api.get<Message[]>(`/channels/${channelId}/messages`, { params })
    return data
  },
  sendMessage: async (channelId: number, message: { content: string; attachment?: string; file_name?: string }) => {
    const { data } = await api.post<Message>(`/channels/${channelId}/messages`, message)
    return data
  },
  editMessage: async (channelId: number, messageId: number, content: string) => {
    const { data } = await api.put<Message>(`/channels/${channelId}/messages/${messageId}`, { content })
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
  getPinnedMessages: async (channelId: number): Promise<Message[]> => {
    const { data } = await api.get<Message[]>(`/channels/${channelId}/pinned`)
    return data
  },
  addReaction: async (channelId: number, messageId: number, emoji: string) => {
    const { data } = await api.post<MessageReaction>(`/channels/${channelId}/messages/${messageId}/reactions`, { emoji })
    return data
  },
  removeReaction: async (channelId: number, messageId: number, emoji: string) => {
    await api.delete(`/channels/${channelId}/messages/${messageId}/reactions`, { data: { emoji } })
  },
  getThreadReplies: async (channelId: number, messageId: number): Promise<Message[]> => {
    const { data } = await api.get<Message[]>(`/channels/${channelId}/messages/${messageId}/replies`)
    return data
  },
  sendThreadReply: async (channelId: number, messageId: number, content: string) => {
    const { data } = await api.post<Message>(`/channels/${channelId}/messages/${messageId}/replies`, { content })
    return data
  },
  starMessage: async (messageId: number) => {
    await api.post(`/channels/star/${messageId}`)
  },
  unstarMessage: async (messageId: number) => {
    await api.delete(`/channels/star/${messageId}`)
  },
  getStarredMessages: async (): Promise<Message[]> => {
    const { data } = await api.get<Message[]>('/channels/starred')
    return data
  },
  updateStatus: async (status: 'online' | 'away' | 'offline') => {
    await api.post('/channels/status', { status })
  },
  getStatuses: async (userIds: number[]): Promise<UserStatus[]> => {
    const { data } = await api.get<UserStatus[]>('/channels/statuses', { params: { user_ids: userIds.join(',') } })
    return data
  },
  searchMessages: async (channelId: number, query: string): Promise<Message[]> => {
    const { data } = await api.get<Message[]>(`/channels/${channelId}/search`, { params: { q: query } })
    return data
  },
  createDM: async (recipientId: number, companyId?: number | null): Promise<DMChannel> => {
    const params = companyId ? { company_id: companyId } : undefined
    const { data } = await api.post<DMChannel>('/channels/dm', { recipient_id: recipientId }, { params })
    return data
  },
  markAsRead: async (id: number) => {
    await api.post(`/channels/${id}/read`)
  },
  getTotalUnreadCount: async (): Promise<number> => {
    const { data } = await api.get<{ total_unread: number }>('/channels/unread/total')
    return data.total_unread
  },
}
