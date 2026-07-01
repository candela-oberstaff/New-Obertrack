import api from './client'
import type { ProfileChangeRequest } from '../types'

export const profileChangeService = {
  create: async (changes: Record<string, string>, note?: string) => {
    const { data } = await api.post<ProfileChangeRequest>('/profile/change-requests', { changes, note })
    return data
  },
  getMine: async () => {
    const { data } = await api.get<ProfileChangeRequest | null>('/profile/change-request')
    return data
  },
  getForUser: async (userId: number) => {
    const { data } = await api.get<ProfileChangeRequest | null>(`/admin/profile-change-requests/user/${userId}`)
    return data
  },
  apply: async (reqId: number, values: Record<string, string>) => {
    await api.post(`/admin/profile-change-requests/${reqId}/apply`, { values })
  },
  reject: async (reqId: number, reason: string) => {
    await api.post(`/admin/profile-change-requests/${reqId}/reject`, { reason })
  },
}
