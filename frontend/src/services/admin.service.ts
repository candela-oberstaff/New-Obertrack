import api from './client'

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
  getAbsenceReport: async (params?: { month?: number; year?: number }) => {
    const { data } = await api.get('/admin/absence-report', { params })
    return data
  },
  getStats: async () => {
    const { data } = await api.get('/admin/stats')
    return data
  },
  getUsers: async (params?: { user_type?: string; is_active?: string; is_manager?: string; page?: number; limit?: number; q?: string }) => {
    const { data } = await api.get('/admin/users', { params })
    return data
  },
  createUser: async (userData: Record<string, unknown>) => {
    const { data } = await api.post('/admin/users', userData)
    return data
  },
  updateUser: async (id: number, userData: Record<string, unknown>) => {
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
  getTenants: async () => {
    const { data } = await api.get('/admin/tenants')
    return data
  },
  getSeniority: async () => {
    const { data } = await api.get<{ data: any[] }>('/admin/seniority')
    return data.data || []
  },
  // ── Archivados (bajas de empleo + cuentas desactivadas) ─────────────────────
  getArchived: async () => {
    const { data } = await api.get<{ data: any[] }>('/admin/archived')
    return data.data || []
  },
  getTenantArchived: async (tenantId: number) => {
    const { data } = await api.get<{ data: any[] }>(`/admin/tenants/${tenantId}/archived`)
    return data.data || []
  },
  reactivateEmployment: async (userId: number, employmentId: number) => {
    await api.post(`/admin/users/${userId}/employments/${employmentId}/reactivate`)
  },
  reactivateUser: async (userId: number) => {
    await api.put(`/admin/users/${userId}`, { is_active: true })
  },
  // ── Membresías (multi-empresa) ──────────────────────────────────────────────
  getUserEmployments: async (userId: number) => {
    const { data } = await api.get<{ data: any[] }>(`/admin/users/${userId}/employments`)
    return data.data || []
  },
  addUserEmployment: async (userId: number, payload: { company_id: number; job_title?: string; start_reason?: string; manager_id?: number }) => {
    const { data } = await api.post(`/admin/users/${userId}/employments`, payload)
    return data
  },
  endUserEmployment: async (userId: number, employmentId: number, endReason: string) => {
    await api.post(`/admin/users/${userId}/employments/${employmentId}/end`, { end_reason: endReason })
  },
  // Setea el manager de un empleo concreto; si es la empresa activa del usuario,
  // el backend espeja users.manager_id.
  updateEmploymentManager: async (userId: number, employmentId: number, managerId: number | null) => {
    const { data } = await api.put(`/admin/users/${userId}/employments/${employmentId}/manager`, { manager_id: managerId })
    return data
  },
  // ── Multi-manager por empleo (Fase 3, gateado por flag) ─────────────────────
  // Flag de features: indica si el front debe mostrar el modo multi-manager.
  getFeatures: async (): Promise<{ multi_manager_reads: boolean }> => {
    const { data } = await api.get('/features')
    return data
  },
  // Lista los managers de un empleo (principal primero, luego por nombre).
  getEmploymentManagers: async (
    userId: number,
    employmentId: number,
  ): Promise<{ manager_id: number; name: string; is_primary: boolean }[]> => {
    const { data } = await api.get(`/admin/users/${userId}/employments/${employmentId}/managers`)
    return data
  },
  // Agrega un manager ADICIONAL (no principal) al empleo.
  addEmploymentManager: async (userId: number, employmentId: number, managerId: number) => {
    const { data } = await api.post(`/admin/users/${userId}/employments/${employmentId}/managers`, { manager_id: managerId })
    return data
  },
  // Quita el vínculo de un manager con el empleo (auto-promueve el siguiente si era principal).
  removeEmploymentManager: async (userId: number, employmentId: number, managerId: number) => {
    const { data } = await api.delete(`/admin/users/${userId}/employments/${employmentId}/managers/${managerId}`)
    return data
  },
  // Marca un manager como principal del empleo (actualiza espejos).
  setPrimaryEmploymentManager: async (userId: number, employmentId: number, managerId: number) => {
    const { data } = await api.put(`/admin/users/${userId}/employments/${employmentId}/managers/${managerId}/primary`)
    return data
  },
  // Lista los profesionales a cargo de un manager (para mostrar a quién reasignar).
  getManagerReports: async (userId: number) => {
    const { data } = await api.get(`/admin/users/${userId}/reports`)
    return data
  },
  // Asigna (o desasigna si managerId es null) un manager a varios profesionales.
  bulkAssignManager: async (professionalIds: number[], managerId: number | null) => {
    const { data } = await api.post('/admin/bulk-assign-manager', { professional_ids: professionalIds, manager_id: managerId })
    return data
  },
  // ── Expediente laboral (resumen + evaluaciones/notas + documentos) ──────────
  getExpediente: async (userId: number, employmentId: number) => {
    const { data } = await api.get(`/admin/users/${userId}/employments/${employmentId}/expediente`)
    return data
  },
  addExpedienteNote: async (
    userId: number,
    employmentId: number,
    payload: { kind: 'note' | 'evaluation'; content: string; rating?: number | null; visibility: 'private' | 'shared' },
  ) => {
    const { data } = await api.post(`/admin/users/${userId}/employments/${employmentId}/notes`, payload)
    return data
  },
  updateExpedienteNote: async (
    userId: number,
    employmentId: number,
    noteId: number,
    payload: { kind: 'note' | 'evaluation'; content: string; rating?: number | null; visibility: 'private' | 'shared' },
  ) => {
    const { data } = await api.put(`/admin/users/${userId}/employments/${employmentId}/notes/${noteId}`, payload)
    return data
  },
  deleteExpedienteNote: async (userId: number, employmentId: number, noteId: number) => {
    await api.delete(`/admin/users/${userId}/employments/${employmentId}/notes/${noteId}`)
  },
  addExpedienteDocument: async (
    userId: number,
    employmentId: number,
    payload: { title?: string; file_name: string; file_url: string; file_size?: number; mime_type?: string; visibility: 'private' | 'shared'; expires_at?: string },
  ) => {
    const { data } = await api.post(`/admin/users/${userId}/employments/${employmentId}/documents`, payload)
    return data
  },
  updateExpedienteDocument: async (
    userId: number,
    employmentId: number,
    docId: number,
    payload: { title?: string; visibility: 'private' | 'shared'; expires_at?: string },
  ) => {
    const { data } = await api.put(`/admin/users/${userId}/employments/${employmentId}/documents/${docId}`, payload)
    return data
  },
  deleteExpedienteDocument: async (userId: number, employmentId: number, docId: number) => {
    await api.delete(`/admin/users/${userId}/employments/${employmentId}/documents/${docId}`)
  },
  // Registra un intento de contacto (email/WhatsApp/chat) al profesional, para
  // que quede en su expediente. Best-effort: no debe bloquear la acción.
  logContact: async (userId: number, channel: 'email' | 'whatsapp' | 'chat') => {
    try { await api.post(`/users/${userId}/contacts`, { channel }) } catch { /* no bloquear */ }
  },
  getFollowUps: async (kind: 'inactivity' | 'absence') => {
    const { data } = await api.get<{ data: any[] }>('/follow-ups', { params: { kind } })
    return data.data || []
  },
  createFollowUp: async (payload: { user_id: number; kind: 'inactivity' | 'absence'; status: string; note?: string }) => {
    const { data } = await api.post('/follow-ups', payload)
    return data
  },
  getTenant: async (id: number) => {
    const { data } = await api.get(`/admin/tenants/${id}`)
    return data
  },
  getTenantEmployees: async (id: number) => {
    const { data } = await api.get(`/admin/tenants/${id}/employees`)
    return data
  },
  getEmployeeTracking: async (id: number) => {
    const { data } = await api.get(`/admin/employees/${id}/tracking`)
    return data
  },
  getTenantActivity: async (id: number) => {
    const { data } = await api.get(`/admin/tenants/${id}/activity`)
    return data
  },
  createTenant: async (tenantData: { company_name: string; user_id?: number; name?: string; email?: string; password?: string }) => {
    const { data } = await api.post('/admin/tenants', tenantData)
    return data
  },
  suspendTenant: async (id: number) => {
    const { data } = await api.post(`/admin/tenants/${id}/suspend`)
    return data
  },
  activateTenant: async (id: number) => {
    const { data } = await api.post(`/admin/tenants/${id}/activate`)
    return data
  },
}
