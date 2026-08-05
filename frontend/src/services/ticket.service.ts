import api from './api';

export interface Contact {
  id: number;
  name: string;
  phone: string;
  email: string;
  company_name?: string;
  parent_contact_id?: number;
  parent_contact?: Contact;
}

export interface TicketMessage {
  id: number;
  ticket_id: number;
  sender_type: 'agent' | 'contact' | 'system';
  channel: 'whatsapp' | 'email' | 'note';
  content: string;
  created_at: string;
  // Vacío = no aplica (entrantes, notas y el histórico previo al outbox).
  delivery_status?: '' | 'pending' | 'sent' | 'failed';
}

export interface SupportAgent {
  id: number;
  name: string;
  email: string;
  zoho_agent_id: string;
}

export interface ZohoAgent {
  zoho_agent_id: string;
  name: string;
  email: string;
  user_id?: number | null;
}

export interface TicketTransfer {
  id: number;
  origin: 'internal' | 'zoho';
  ticket_ref: string;
  ticket_title: string;
  from_name: string;
  to_name: string;
  by_name: string;
  reason?: string;
  created_at: string;
}

export interface Ticket {
  id: number;
  /** Real Zoho Desk ticket ID (string) – use this for navigation & API calls */
  zoho_id: string;
  contact_id: number;
  contact?: Contact;
  title: string;
  channel?: string;
  /** Zoho ticket number (e.g. #1721) */
  ticket_number?: string;
  priority?: string;
  category?: string;
  module?: string;
  description?: string;
  stage: 'new' | 'in_progress' | 'waiting' | 'closed';
  status: string;
  assigned_to?: number | null;
  messages?: TicketMessage[];
  created_at: string;
  updated_at: string;
  sentiment?: string;
  customer_tone?: string;
  is_escalated?: boolean;
  web_url?: string;
  assignee_id?: string;
  assignee_name?: string;
  assignee_email?: string;
  department_id?: string;
  // 'obersuite' = alta llegada del puente de contratación: no es una
  // conversación, es el aviso de que hay alguien nuevo al que acompañar.
  origin?: 'zoho' | 'internal' | 'support' | 'whatsapp' | 'obersuite';
  /** Canal de chat asociado (solo origin 'support'): para navegar al chat */
  channel_id?: number;
  // Internal work-hour-rejection alert fields
  user_id?: number;
  professional_email?: string;
  professional_phone?: string;
  company_name?: string;
  rejected_by_name?: string;
  reason?: string;
  work_dates?: string;
}

export interface TicketStatusOption {
  value: string;
  label: string;
  status_type?: string;
  stage: Ticket['stage'];
}

export interface LinkedUser {
  id: number;
  name: string;
  email: string;
  phone_number: string;
  user_type: 'empleador' | 'profesional' | 'superadmin';
  company_name?: string;
  job_title?: string;
  country?: string;
  city?: string;
}

export interface TicketDetail {
  ticket: Ticket;
  linked_user: LinkedUser | null;
  zoho_id: string;
}

export interface WhatsAppChatTicket {
  zoho_id: string
  contact_name: string
  contact_phone: string
  subject: string
  status: string
  assignee_id?: string
  modified_time: string
  department_id?: string
}

export interface WhatsAppMessageDTO {
  id: string
  content: string
  direction: 'incoming' | 'outgoing'
  author_name: string
  author_type: string
  created_time: string
  delivery_status?: '' | 'pending' | 'sent' | 'failed'
}

/** Respuesta de /tickets/wa/lookup: qué se puede hacer con un teléfono. */
export interface WaChatLookup {
  /** Teléfono ya normalizado a dígitos; vacío si no había número. */
  digits: string
  /** Conversación existente, o 0 si no hay ninguna. */
  ticket_id: number
  /**
   * Si se puede responder desde la bandeja. False cuando no hay hilo y también
   * cuando lo hay pero nadie escribió desde el otro lado: la guarda de contacto
   * en frío rechazaría el envío.
   */
  can_reply: boolean
}

export interface WahaStatus {
  name: string
  status: string // "WORKING" | "SCAN_QR_CODE" | "STARTING" | "STOPPED" | "FAILED" ...
  qr?: {
    raw?: string
    image?: string // base64 (o data URI) del QR cuando hace falta escanear
  }
}

// Una sesión de WhatsApp con conversaciones guardadas. `current` distingue la
// que está conectada ahora de las huérfanas, que son las de números ya
// desvinculados: no aparecen en la bandeja pero siguen enteras en la base.
export interface WahaSessionInfo {
  session: string
  tickets: number
  messages: number
  current: boolean
}

export interface WahaPurgeCounts {
  tickets: number
  messages: number
  contacts: number
}

export const ticketService = {
  getTickets: async (): Promise<Ticket[]> => {
    const response = await api.get('/tickets')
    return response.data
  },

  getTicketStatuses: async (): Promise<TicketStatusOption[]> => {
    const response = await api.get('/tickets/statuses')
    return response.data
  },

  /** Updates the stage/status of an internal alert ticket (follow-up). */
  updateInternalTicket: async (id: number, data: { stage?: string; status?: string }): Promise<Ticket> => {
    const response = await api.put(`/tickets/internal/${id}`, data)
    return response.data
  },

  /** Marks an internal Obertrack alert ticket as resolved (closed). */
  resolveInternalTicket: async (id: number): Promise<Ticket> => {
    const response = await api.put(`/tickets/internal/${id}`, { stage: 'closed', status: 'closed' })
    return response.data
  },

  /** Lists active local support users (customer_success + superadmin) — internal ticket targets. */
  getSupportAgents: async (): Promise<SupportAgent[]> => {
    const response = await api.get('/tickets/agents')
    return response.data
  },

  /** Lists Zoho Desk agents (Zoho ticket transfer targets), cross-referenced with local users. */
  getZohoAgents: async (): Promise<ZohoAgent[]> => {
    const response = await api.get('/tickets/zoho-agents')
    return response.data
  },

  /** Transfer history for a ticket. origin = 'zoho' | 'internal', ref = zoho id or internal id. */
  getTicketTransfers: async (origin: 'zoho' | 'internal', ref: string | number): Promise<TicketTransfer[]> => {
    const response = await api.get('/tickets/transfers', { params: { origin, ref: String(ref) } })
    return response.data
  },

  /** Reassigns a Zoho ticket to another Zoho Desk agent (by Zoho agent id). */
  transferZohoTicket: async (zohoId: string, toAgentId: string, reason: string): Promise<void> => {
    await api.post(`/tickets/${zohoId}/transfer`, { to_agent_id: toAgentId, reason })
  },

  /** Reassigns an internal alert ticket to another support agent. */
  transferInternalTicket: async (id: number, toUserId: number, reason: string): Promise<Ticket> => {
    const response = await api.post(`/tickets/internal/${id}/transfer`, { to_user_id: toUserId, reason })
    return response.data
  },

  /** Fetches a single internal alert ticket (for its detail page). */
  getInternalTicket: async (id: number): Promise<Ticket> => {
    const response = await api.get(`/tickets/internal/${id}`)
    return response.data
  },

  /** Appends a follow-up note to an internal alert ticket. */
  addInternalNote: async (id: number, content: string): Promise<TicketMessage> => {
    const response = await api.post(`/tickets/internal/${id}/notes`, { content })
    return response.data
  },

  /** Fetches the work-hour rejections report for a given month/year. */
  getRejectionReport: async (month: number, year: number): Promise<{ items: Ticket[]; total: number; month: number; year: number }> => {
    const response = await api.get('/tickets/internal/report', { params: { month, year } })
    return response.data
  },

  getTicket: async (zohoId: string): Promise<TicketDetail> => {
    const response = await api.get(`/tickets/${zohoId}`)
    return response.data
  },

  updateTicket: async (zohoId: string, data: Partial<Ticket>): Promise<Ticket> => {
    const response = await api.put(`/tickets/${zohoId}`, data)
    return response.data
  },

  sendMessage: async (zohoId: string, content: string, channel: 'whatsapp' | 'email' | 'note', templateId?: string): Promise<TicketMessage> => {
    const response = await api.post(`/tickets/${zohoId}/messages`, { content, channel, template_id: templateId })
    return response.data
  },

  getWhatsAppTicket: async (id: number): Promise<Ticket> => {
    const response = await api.get(`/tickets/wa/${id}`)
    return response.data
  },

  sendWhatsAppReply: async (id: number, content: string): Promise<TicketMessage> => {
    const response = await api.post(`/tickets/wa/${id}/messages`, { content })
    return response.data
  },

  whatsAppAction: async (id: number, action: 'claim' | 'resolve' | 'reopen'): Promise<Ticket> => {
    const response = await api.patch(`/tickets/wa/${id}`, { action })
    return response.data
  },

  // Traspasa la conversación a otro agente. Queda en la bitácora de traspasos y
  // le llega un aviso al nuevo responsable.
  assignWhatsAppTicket: async (id: number, assigneeId: number, reason?: string): Promise<Ticket> => {
    const response = await api.patch(`/tickets/wa/${id}`, {
      action: 'assign',
      assignee_id: assigneeId,
      reason: reason || '',
    })
    return response.data
  },

  getWaChats: async (): Promise<WhatsAppChatTicket[]> => {
    const response = await api.get('/tickets/wa')
    const tickets: Ticket[] = response.data ?? []
    return tickets.map(t => ({
      zoho_id: String(t.id),
      contact_name: t.contact?.name || t.title || t.contact?.phone || 'Sin nombre',
      contact_phone: t.contact?.phone || t.professional_phone || '',
      subject: t.title || '',
      status: t.status,
      assignee_id: t.assigned_to ? String(t.assigned_to) : '',
      modified_time: t.updated_at || t.created_at,
    }))
  },

  getWaChatMessages: async (id: string): Promise<WhatsAppMessageDTO[]> => {
    const response = await api.get(`/tickets/wa/${id}`)
    const contactName = response.data?.contact?.name || 'Contacto'
    const msgs: TicketMessage[] = response.data?.messages ?? []
    return msgs.map(m => ({
      id: String(m.id),
      content: m.content,
      direction: m.sender_type === 'agent' ? 'outgoing' : 'incoming',
      author_name: m.sender_type === 'agent' ? 'Agente' : contactName,
      author_type: m.sender_type,
      created_time: m.created_at,
      delivery_status: m.delivery_status,
    }))
  },

  sendWaChatMessage: async (id: string, content: string): Promise<WhatsAppMessageDTO> => {
    const response = await api.post(`/tickets/wa/${id}/messages`, { content })
    const m = response.data
    return {
      id: String(m.id),
      content: m.content,
      direction: 'outgoing',
      author_name: 'Agente',
      author_type: 'agent',
      created_time: m.created_at,
      delivery_status: m.delivery_status,
    }
  },

  // ¿Este teléfono ya tiene conversación de WhatsApp con nosotros? Lo consulta
  // la ficha de empresa para decidir entre llevar a la bandeja o abrir wa.me:
  // escribir en frío desde la línea oficial está bloqueado a propósito.
  lookupWaChat: async (phone: string): Promise<WaChatLookup> => {
    const { data } = await api.get<WaChatLookup>('/tickets/wa/lookup', { params: { phone } })
    return data
  },

  assignWaChat: async (id: string): Promise<void> => {
    await api.patch(`/tickets/wa/${id}`, { action: 'claim' })
  },

  // Estado de la sesión WAHA (Conectado / Escanear QR / Desconectado).
  getWahaStatus: async (): Promise<WahaStatus> => {
    const response = await api.get('/tickets/waha/status')
    return response.data
  },

  // Fuerza el (re)arranque de la sesión en WAHA y devuelve el estado/QR fresco.
  forceWahaConnection: async (): Promise<WahaStatus> => {
    const response = await api.post('/tickets/waha/start')
    return response.data
  },

  // Fuerza la traída del historial reciente desde WAHA. Es la salida cuando el
  // webhook no está entregando y la bandeja se queda atrás.
  syncWahaHistory: async (): Promise<{ imported: number }> => {
    const response = await api.post('/tickets/waha/sync')
    return response.data
  },

  // Inventario de conversaciones guardadas por sesión, incluidas las huérfanas
  // (números ya desvinculados que no se ven en la bandeja pero siguen guardados).
  getWahaSessions: async (): Promise<WahaSessionInfo[]> => {
    const response = await api.get('/tickets/waha/sessions')
    return response.data ?? []
  },

  // Borrado DEFINITIVO de las conversaciones de una sesión. No se puede deshacer.
  purgeWahaSession: async (session: string): Promise<WahaPurgeCounts> => {
    const response = await api.delete(`/tickets/waha/sessions/${encodeURIComponent(session)}`)
    return response.data
  },

  getWhatsAppTemplates: async (departmentId?: string): Promise<{ id: string; title: string; message: string; displayMessage: string; status: string; language?: string }[]> => {
    const params = departmentId ? { departmentId } : {}
    const response = await api.get('/chats/templates', { params })
    return response.data
  },


  getMyWhatsAppChats: async (modifiedSince?: string): Promise<WhatsAppChatTicket[]> => {
    const params = modifiedSince ? { modifiedSince } : {}
    const response = await api.get('/chats/me', { params })
    return response.data
  },

  getUnassignedWhatsAppChats: async (modifiedSince?: string): Promise<WhatsAppChatTicket[]> => {
    const params = modifiedSince ? { modifiedSince } : {}
    const response = await api.get('/chats/unassigned', { params })
    return response.data
  },

  getChatMessages: async (ticketId: string): Promise<WhatsAppMessageDTO[]> => {
    const response = await api.get(`/chats/${ticketId}/messages`)
    return response.data
  },

  assignChat: async (ticketId: string): Promise<{ message: string; ticket_id: string; zoho_agent_id: string }> => {
    const response = await api.patch(`/chats/${ticketId}/assign`)
    return response.data
  },

  sendWhatsAppMessage: async (ticketId: string, content: string, templateId?: string): Promise<WhatsAppMessageDTO> => {
    const response = await api.post(`/chats/${ticketId}/send`, { content, template_id: templateId })
    return response.data
  },

  getContacts: async (): Promise<Contact[]> => {
    const response = await api.get('/tickets/contacts')
    return response.data
  },

  updateContact: async (id: number, data: { name?: string; company_name?: string; parent_contact_id?: number | null }): Promise<Contact> => {
    const response = await api.put(`/tickets/contacts/${id}`, data)
    return response.data
  },
}
