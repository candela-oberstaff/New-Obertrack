import api from './client'
import type { User } from '../types'

export const authService = {
  login: async (email: string, password: string) => {
    const { data } = await api.post<{ user: User; access_token: string }>('/auth/login', { email, password })
    return data
  },
  register: async (userData: { 
    name: string; 
    email: string; 
    password: string; 
    user_type?: string; 
    company_name?: string;
    empleador_id?: number;
  }) => {
    const { data } = await api.post<{ user: User; access_token: string }>('/auth/register', userData)
    return data
  },
  me: async () => {
    const { data } = await api.get<User>('/auth/me')
    return data
  },
  getPublicCompanies: async () => {
    const { data } = await api.get<{ id: number; name: string }[]>('/auth/companies')
    return data
  },
}
