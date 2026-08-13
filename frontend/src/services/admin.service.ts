import api from './client'
import { uploadService } from './upload.service'

/**
 * Vías por las que el equipo contacta con una empresa. Email y WhatsApp los
 * registra la plataforma al enviar; llamada y reunión ocurren fuera y las anota
 * una persona. Debe coincidir con models.IsValidCompanyContactChannel.
 */
export type TenantContactChannel = 'email' | 'whatsapp' | 'call' | 'meeting'

/** Archivo colgado de una entrada del expediente o de uno de sus comentarios. */
export interface CompanyAttachment {
  id: number
  event_id: number
  comment_id?: number | null
  file_name: string
  file_size: number
  mime_type: string
  author: string
  created_at: string
}

/** Comentario sobre una entrada del expediente. */
export interface CompanyComment {
  id: number
  event_id: number
  content: string
  author: string
  created_at: string
  edited_at?: string | null
  attachments?: CompanyAttachment[]
}

/** Lo que cuelga de una entrada del expediente. */
export interface EventThread {
  comments: CompanyComment[]
  attachments: CompanyAttachment[]
}

/**
 * URL de descarga de un adjunto.
 *
 * Va por una ruta que comprueba permisos y no por /api/uploads/:filename, que
 * solo exige estar autenticado: aquí hay capturas de incidencias y documentos
 * internos que no puede ver ni la propia empresa. Sirve igual para <img src> y
 * para un enlace, porque la sesión viaja en cookie.
 */
export function tenantAttachmentUrl(tenantId: number, attachmentId: number): string {
  return `/api/admin/tenants/${tenantId}/attachments/${attachmentId}/download`
}

/** Un ticket visto desde la ficha de la empresa. */
export interface TenantTicket {
  id: number
  /** internal = alerta de la plataforma; whatsapp/zoho = conversación. */
  origin: string
  title: string
  stage: string
  status: string
  /** Sobre quién va: el contacto de WhatsApp o el profesional de la alerta. */
  about: string
  assignee: string
  created_at: string
  updated_at: string
}

/** Una incidencia que alcanzó a un profesional, con SU respuesta. */
export interface UserIncident {
  id: number
  title: string
  description: string
  kind: string
  country: string
  state: string
  status: string
  created_at: string
  closed_at?: string | null
  /** pendiente | contactado | ok | sin_respuesta */
  response_status: string
  response_note: string
  responded_at?: string | null
}

export interface TrashItem {
  type: string
  type_label: string
  id: number
  title: string
  subtitle: string
  deleted_at: string | null
}
export interface TrashTypeInfo {
  key: string
  label: string
}
export interface TrashFailure {
  type: string
  id: number
  error: string
}

export interface ProfessionalLocation {
  id: number
  name: string
  email: string
  phone_number: string
  avatar: string
  country: string
  state: string
  city: string
  company: string
  is_active: boolean
}

export type IncidentStatus = 'pendiente' | 'contactado' | 'ok' | 'sin_respuesta'

export interface IncidentCounts {
  affected: number
  pendiente: number
  contactado: number
  ok: number
  sin_respuesta: number
}

export interface Incident {
  id: number
  title: string
  kind: string
  country: string
  state: string
  status: string
  created_at: string
  closed_at: string | null
  counts: IncidentCounts
}

export interface EmergencyTemplate {
  id: number
  title: string
  subject: string
  body: string
  created_at: string
}

export interface IncidentProfessional {
  id: number
  name: string
  email: string
  phone_number: string
  company: string
  country: string
  state: string
  city: string
  is_active: boolean
  status: IncidentStatus
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
  // Feed global paginado por cursor: `before*` es el último evento ya mostrado.
  // Sin parámetros devuelve la primera página, como antes.
  getRecentActivity: async (params?: { limit?: number; before?: string; before_type?: string; before_id?: number }) => {
    const { data } = await api.get('/admin/recent-activity', { params })
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
  getProfessionalLocations: async (params: { country?: string; state?: string; active?: string }): Promise<{ professionals: ProfessionalLocation[] }> => {
    const { data } = await api.get('/admin/professionals/locations', { params })
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
  importPreview: async (file: File) => {
    const fd = new FormData()
    fd.append('file', file)
    const { data } = await api.post('/admin/import/preview', fd, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
    return data
  },
  importExecute: async (payload: { companies: any[]; professionals: any[] }) => {
    const { data } = await api.post('/admin/import/execute', payload)
    return data
  },
  downloadImportTemplate: async () => {
    const res = await api.get('/admin/import/template', { responseType: 'blob' })
    const url = URL.createObjectURL(res.data as Blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'plantilla_importacion_obertrack.xlsx'
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
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
  // Pestaña Actividad de la ficha de empresa: el mismo semáforo de inactividad
  // y reporte de ausencias del panel, ya acotados por el backend a esa empresa.
  getTenantInactiveUsers: async (tenantId: number, days = 1) => {
    const { data } = await api.get(`/admin/tenants/${tenantId}/inactive-users`, { params: { days } })
    return data || []
  },
  getTenantAbsenceReport: async (tenantId: number, params?: { month?: number; year?: number }) => {
    const { data } = await api.get(`/admin/tenants/${tenantId}/absence-report`, { params })
    return data
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
  // Corrige la fecha de ingreso de un empleo (AAAA-MM-DD). El alta la sella con
  // el día en que se cargó el profesional, que casi nunca es el día real.
  updateEmploymentStart: async (userId: number, employmentId: number, startedAt: string) => {
    const { data } = await api.put(`/admin/users/${userId}/employments/${employmentId}/start`, { started_at: startedAt })
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
  bulkDeleteUsers: async (userIds: number[]) => {
    const { data } = await api.post<{ deleted: number; skipped: { id: number; name: string; reason: string }[] }>('/admin/bulk-delete-users', { user_ids: userIds })
    return data
  },
  getTrash: async (types?: string[]) => {
    const { data } = await api.get<{ items: TrashItem[]; types: TrashTypeInfo[] }>('/admin/trash', {
      params: types && types.length ? { types: types.join(',') } : {},
    })
    return data
  },
  restoreTrash: async (items: { type: string; id: number }[]) => {
    const { data } = await api.post<{ restored: number; failed: TrashFailure[] }>('/admin/trash/restore', { items })
    return data
  },
  purgeTrash: async (items: { type: string; id: number }[]) => {
    const { data } = await api.post<{ purged: number; failed: TrashFailure[] }>('/admin/trash/purge', { items })
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
  getFollowUps: async (kind: 'inactivity' | 'absence' | 'emergencia') => {
    const { data } = await api.get<{ data: any[] }>('/follow-ups', { params: { kind } })
    return data.data || []
  },
  createFollowUp: async (payload: { user_id: number; kind: 'inactivity' | 'absence' | 'emergencia'; status: string; note?: string }) => {
    const { data } = await api.post('/follow-ups', payload)
    return data
  },
  bulkEmailProfessionals: async (payload: { user_ids: number[]; subject: string; body: string }): Promise<{ sent: number; failed: { id: number; email: string; error: string }[] }> => {
    const { data } = await api.post('/admin/professionals/bulk-email', payload)
    return data
  },
  /**
   * Entrega el acceso a la plataforma. 'invite' manda un enlace para crear la
   * contraseña; 'password' genera una clave temporal nueva y la envía en claro.
   *
   * La contraseña que genera la importación masiva no se puede reenviar: se
   * guarda hasheada. Esto emite un acceso nuevo, no reenvía el anterior.
   */
  sendAccessEmails: async (payload: { user_ids: number[]; mode: 'invite' | 'password' }): Promise<{
    sent: number
    total: number
    failed: { id: number; email: string; error: string }[]
  }> => {
    const { data } = await api.post('/admin/users/send-access', payload)
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
  // Soporte de UNA persona: los tickets que van sobre ella y las incidencias en
  // las que se la incluyó. Separado de los de la empresa a propósito.
  getEmployeeTickets: async (id: number) => {
    const { data } = await api.get<{ data: TenantTicket[] }>(`/admin/employees/${id}/tickets`)
    return data.data || []
  },
  getEmployeeIncidents: async (id: number) => {
    const { data } = await api.get<{ data: UserIncident[] }>(`/admin/employees/${id}/incidents`)
    return data.data || []
  },
  // Tickets de la empresa: alertas internas sobre su gente + conversaciones de
  // WhatsApp de su número, con el mismo criterio que el KPI de la cabecera.
  getTenantTickets: async (id: number) => {
    const { data } = await api.get<{ data: TenantTicket[] }>(`/admin/tenants/${id}/tickets`)
    return data.data || []
  },
  // Expediente de la empresa: paginado y filtrable por categoría y por persona.
  getTenantActivity: async (id: number, params?: { category?: string; user_id?: number; page?: number; limit?: number }) => {
    const { data } = await api.get(`/admin/tenants/${id}/activity`, { params })
    return data as {
      data: any[]
      total: number
      page: number
      limit: number
      counts: Record<string, number>
      /** Hilos de las entradas de esta página, por event_id. */
      threads: Record<number, EventThread>
    }
  },
  // Quién aparece en el expediente (sale de los propios movimientos, no de la
  // plantilla actual), para ofrecerlo como filtro.
  getTenantActivityPeople: async (id: number) => {
    const { data } = await api.get<{ data: { user_id: number; name: string }[] }>(
      `/admin/tenants/${id}/activity/people`,
    )
    return data.data || []
  },
  addTenantNote: async (id: number, detail: string) => {
    const { data } = await api.post(`/admin/tenants/${id}/notes`, { detail })
    return data
  },
  updateTenantNote: async (id: number, noteId: number, detail: string) => {
    const { data } = await api.put(`/admin/tenants/${id}/notes/${noteId}`, { detail })
    return data
  },
  setTenantNotePinned: async (id: number, noteId: number, pinned: boolean) => {
    const { data } = await api.put(`/admin/tenants/${id}/notes/${noteId}/pin`, { pinned })
    return data
  },
  deleteTenantNote: async (id: number, noteId: number) => {
    await api.delete(`/admin/tenants/${id}/notes/${noteId}`)
  },
  // Registra en el expediente que el equipo contactó con la empresa. El correo
  // y el WhatsApp lo llaman solos tras enviar; la llamada y la reunión se
  // anotan a mano desde la ficha.
  logTenantContact: async (id: number, channel: TenantContactChannel, detail?: string) => {
    const { data } = await api.post(`/admin/tenants/${id}/contacts`, { channel, detail: detail || '' })
    return data
  },
  // --- Hilo de una entrada del expediente (comentarios y archivos) ---
  addTenantComment: async (id: number, eventId: number, content: string) => {
    const { data } = await api.post(`/admin/tenants/${id}/events/${eventId}/comments`, { content })
    return data as CompanyComment
  },
  updateTenantComment: async (id: number, commentId: number, content: string) => {
    await api.put(`/admin/tenants/${id}/comments/${commentId}`, { content })
  },
  deleteTenantComment: async (id: number, commentId: number) => {
    await api.delete(`/admin/tenants/${id}/comments/${commentId}`)
  },
  /**
   * Sube el archivo y lo cuelga de la entrada. Son dos pasos porque el binario
   * va por el endpoint genérico de subidas y aquí solo se registra a qué
   * entrada pertenece; `stored_name` es el nombre con el que quedó en disco.
   */
  addTenantAttachment: async (
    id: number,
    eventId: number,
    file: File,
    commentId?: number,
  ): Promise<CompanyAttachment> => {
    const up = await uploadService.upload(file)
    const { data } = await api.post(`/admin/tenants/${id}/events/${eventId}/attachments`, {
      stored_name: up.filename,
      // El nombre visible es el original del archivo; el del disco lleva id de
      // usuario y marca de tiempo y no le dice nada a nadie.
      file_name: (file as File).name || up.filename,
      file_size: up.size,
      mime_type: up.type,
      comment_id: commentId,
    })
    return data as CompanyAttachment
  },
  deleteTenantAttachment: async (id: number, attachmentId: number) => {
    await api.delete(`/admin/tenants/${id}/attachments/${attachmentId}`)
  },
  // Notas fijadas: van en la cabecera del expediente, fuera de la cronología.
  getTenantPinnedNotes: async (id: number) => {
    const { data } = await api.get<{ data: any[] }>(`/admin/tenants/${id}/notes/pinned`)
    return data.data || []
  },
  createTenant: async (tenantData: { company_name: string; user_id?: number; name?: string; email?: string; password?: string }) => {
    const { data } = await api.post('/admin/tenants', tenantData)
    return data
  },
  getIncidents: async (): Promise<{ incidents: Incident[] }> => {
    const { data } = await api.get('/admin/incidents')
    return data
  },
  createIncident: async (payload: { title: string; description: string; kind: string; country: string; state: string }): Promise<{ incident: Incident }> => {
    const { data } = await api.post('/admin/incidents', payload)
    return data
  },
  getIncident: async (id: number): Promise<{ incident: Incident; professionals: IncidentProfessional[] }> => {
    const { data } = await api.get(`/admin/incidents/${id}`)
    return data
  },
  closeIncident: async (id: number): Promise<{ incident: Incident }> => {
    const { data } = await api.put(`/admin/incidents/${id}/close`)
    return data
  },
  broadcastIncident: async (id: number, payload: { subject: string; body: string }): Promise<{ sent: number; failed: { id: number; email: string; error: string }[] }> => {
    const { data } = await api.post(`/admin/incidents/${id}/broadcast`, payload)
    return data
  },
  setIncidentResponse: async (id: number, userId: number, payload: { status: IncidentStatus; note?: string }): Promise<{ ok: true }> => {
    const { data } = await api.put(`/admin/incidents/${id}/responses/${userId}`, payload)
    return data
  },
  getEmergencyTemplates: async (): Promise<{ templates: EmergencyTemplate[] }> => {
    const { data } = await api.get('/admin/emergency-templates')
    return data
  },
  createEmergencyTemplate: async (payload: { title: string; subject: string; body: string }): Promise<{ template: EmergencyTemplate }> => {
    const { data } = await api.post('/admin/emergency-templates', payload)
    return data
  },
  updateEmergencyTemplate: async (id: number, payload: { title: string; subject: string; body: string }): Promise<{ template: EmergencyTemplate }> => {
    const { data } = await api.put(`/admin/emergency-templates/${id}`, payload)
    return data
  },
  deleteEmergencyTemplate: async (id: number): Promise<{ ok: true }> => {
    const { data } = await api.delete(`/admin/emergency-templates/${id}`)
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
