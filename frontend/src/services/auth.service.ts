import api from './client'
import type { User } from '../types'

export const authService = {
  // Tokens now live in httpOnly cookies; responses only carry the user object.
  login: async (email: string, password: string) => {
    const { data } = await api.post<{ user: User }>('/auth/login', { email, password })
    return data
  },
  register: async (userData: {
    name: string;
    email: string;
    password: string;
    user_type?: string;
    company_name?: string;
    industry?: string;
    empleador_id?: number;
    phone_number?: string;
    country?: string;
    state?: string;
    location?: string;
    address?: string;
    job_title?: string;
  }) => {
    const { data } = await api.post<{ user: User }>('/auth/register', userData)
    return data
  },
  me: async () => {
    const { data } = await api.get<User>('/auth/me')
    return data
  },
  logout: async () => {
    await api.post('/auth/logout')
  },
  getPublicCompanies: async () => {
    const { data } = await api.get<{ id: number; name: string }[]>('/auth/companies')
    return data
  },
  forgotPassword: async (email: string) => {
    const { data } = await api.post<{ message: string }>('/auth/forgot-password', { email })
    return data
  },
  resetPassword: async (token: string, new_password: string) => {
    const { data } = await api.post<{ message: string }>('/auth/reset-password', { token, new_password })
    return data
  },
}
