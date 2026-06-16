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
  // Cambia la empresa activa (multi-empresa). Devuelve el usuario actualizado.
  switchCompany: async (companyId: number) => {
    const { data } = await api.post<User>('/auth/switch-company', { company_id: companyId })
    return data
  },
  // Expediente propio: el profesional ve su CV vivo y el detalle de cada empleo.
  myEmployments: async () => {
    const { data } = await api.get<{ data: any[] }>('/me/employments')
    return data.data || []
  },
  // CV vivo: trayectoria unificada del profesional en todas las empresas.
  myCV: async () => {
    const { data } = await api.get('/me/cv')
    return data
  },
  myExpediente: async (employmentId: number) => {
    const { data } = await api.get(`/me/employments/${employmentId}/expediente`)
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
