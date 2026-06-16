import api from './client'

// Gestión del expediente por el EMPLEADOR (solo profesionales de su empresa).
// El backend acota el acceso a su propia empresa (requireExpedienteOwnership).
export const employerService = {
  // Resuelve el empleo de un profesional dentro de la empresa del empleador.
  resolveEmployment: async (userId: number) => {
    const { data } = await api.get(`/employer/users/${userId}/employment`)
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
