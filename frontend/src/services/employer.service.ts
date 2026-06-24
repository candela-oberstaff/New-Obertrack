import api from './client'
import type { User } from '../types'

// Gestión del expediente por el EMPLEADOR (solo profesionales de su empresa).
// El backend acota el acceso a su propia empresa (requireExpedienteOwnership).
export const employerService = {
  // Crea un profesional en la empresa del empleador. El backend genera una
  // contraseña TEMPORAL que devuelve en claro UNA sola vez (mostrar y compartir
  // por canal seguro). Email duplicado -> 400/409 con mensaje del backend.
  createEmployee: async (
    payload: {
      name: string
      email: string
      job_title?: string
      phone_number?: string
      country?: string
      state?: string
      city?: string
      location?: string
      manager_id?: number
    },
  ): Promise<{ user: User; temp_password: string }> => {
    const { data } = await api.post('/employer/employees', payload)
    return data
  },
  updateEmployee: async (
    userId: number,
    payload: {
      name?: string
      email?: string
      job_title?: string
      phone_number?: string
      country?: string
      city?: string
      location?: string
      is_active?: boolean
      is_manager?: boolean
      manager_id?: number
    },
  ): Promise<User> => {
    const { data } = await api.put(`/employer/users/${userId}`, payload)
    return data
  },
  // Elimina (soft delete) un profesional de la empresa del empleador. 409 si el
  // profesional es manager con equipo a su cargo (reasignar primero).
  deleteEmployee: async (userId: number): Promise<void> => {
    await api.delete(`/employer/users/${userId}`)
  },
  resetEmployeePassword: async (userId: number): Promise<{ temp_password: string }> => {
    const { data } = await api.post(`/employer/users/${userId}/reset-password`)
    return data
  },
  importPreview: async (file: File) => {
    const fd = new FormData()
    fd.append('file', file)
    const { data } = await api.post('/employer/import/preview', fd, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
    return data
  },
  importExecute: async (payload: { companies: any[]; professionals: any[] }) => {
    const { data } = await api.post('/employer/import/execute', payload)
    return data
  },
  downloadImportTemplate: async () => {
    const res = await api.get('/employer/import/template', { responseType: 'blob' })
    const url = URL.createObjectURL(res.data as Blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'plantilla_profesionales_obertrack.xlsx'
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
  },
  // Resuelve el empleo de un profesional dentro de la empresa del empleador.
  resolveEmployment: async (userId: number) => {
    const { data } = await api.get(`/employer/users/${userId}/employment`)
    return data
  },
  // Alias semántico para el flujo multi-manager (mismo endpoint que resolveEmployment).
  // El backend devuelve un EmploymentView con .id y .company_id en el primer nivel.
  getEmployerEmployment: async (
    userId: number,
  ): Promise<{ id: number; company_id: number } & Record<string, unknown>> => {
    const { data } = await api.get(`/employer/users/${userId}/employment`)
    return data
  },
  // ── Multi-manager por empleo (Fase 3, gateado por flag) ─────────────────────
  // Flag de features: indica si el front debe mostrar el modo multi-manager.
  getFeatures: async (): Promise<{ multi_manager_reads: boolean }> => {
    const { data } = await api.get('/features')
    return data
  },
  // Profesionales a cargo de un manager (para el bloqueo "reasigna primero").
  getManagerReports: async (userId: number) => {
    const { data } = await api.get(`/employer/users/${userId}/reports`)
    return data
  },
  // Lista los managers de un empleo (principal primero, luego por nombre).
  getEmployerEmploymentManagers: async (
    userId: number,
    employmentId: number,
  ): Promise<{ manager_id: number; name: string; is_primary: boolean }[]> => {
    const { data } = await api.get(`/employer/users/${userId}/employments/${employmentId}/managers`)
    return data
  },
  // Agrega un manager ADICIONAL (no principal) al empleo.
  addEmployerEmploymentManager: async (userId: number, employmentId: number, managerId: number) => {
    const { data } = await api.post(`/employer/users/${userId}/employments/${employmentId}/managers`, { manager_id: managerId })
    return data
  },
  // Quita el vínculo de un manager con el empleo (auto-promueve el siguiente si era principal).
  removeEmployerEmploymentManager: async (userId: number, employmentId: number, managerId: number) => {
    const { data } = await api.delete(`/employer/users/${userId}/employments/${employmentId}/managers/${managerId}`)
    return data
  },
  // Marca un manager como principal del empleo (actualiza espejos).
  setPrimaryEmployerEmploymentManager: async (userId: number, employmentId: number, managerId: number) => {
    const { data } = await api.put(`/employer/users/${userId}/employments/${employmentId}/managers/${managerId}/primary`)
    return data
  },
  getExpediente: async (employmentId: number) => {
    const { data } = await api.get(`/employer/employments/${employmentId}/expediente`)
    return data
  },
  addNote: async (
    employmentId: number,
    payload: { kind: 'note' | 'evaluation'; content: string; rating?: number | null; visibility: 'private' | 'shared' },
  ) => {
    const { data } = await api.post(`/employer/employments/${employmentId}/notes`, payload)
    return data
  },
  updateNote: async (
    employmentId: number,
    noteId: number,
    payload: { kind: 'note' | 'evaluation'; content: string; rating?: number | null; visibility: 'private' | 'shared' },
  ) => {
    const { data } = await api.put(`/employer/employments/${employmentId}/notes/${noteId}`, payload)
    return data
  },
  deleteNote: async (employmentId: number, noteId: number) => {
    await api.delete(`/employer/employments/${employmentId}/notes/${noteId}`)
  },
  addDocument: async (
    employmentId: number,
    payload: { title?: string; file_name: string; file_url: string; file_size?: number; mime_type?: string; visibility: 'private' | 'shared'; expires_at?: string },
  ) => {
    const { data } = await api.post(`/employer/employments/${employmentId}/documents`, payload)
    return data
  },
  updateDocument: async (
    employmentId: number,
    docId: number,
    payload: { title?: string; visibility: 'private' | 'shared'; expires_at?: string },
  ) => {
    const { data } = await api.put(`/employer/employments/${employmentId}/documents/${docId}`, payload)
    return data
  },
  deleteDocument: async (employmentId: number, docId: number) => {
    await api.delete(`/employer/employments/${employmentId}/documents/${docId}`)
  },
}
