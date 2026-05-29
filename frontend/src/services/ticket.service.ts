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
  description?: string;
  stage: 'new' | 'in_progress' | 'waiting' | 'closed';
  status: string;
  assigned_to?: number;
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

export const ticketService = {
  getTickets: async (): Promise<Ticket[]> => {
    const response = await api.get('/tickets');
    return response.data;
  },

  /**
   * Fetch a single ticket by its Zoho Desk string ID.
   * The backend accepts the zoho_id directly as the :id param.
   */
  getTicket: async (zohoId: string): Promise<TicketDetail> => {
    const response = await api.get(`/tickets/${zohoId}`);
    return response.data;
  },

  updateTicket: async (zohoId: string, data: Partial<Ticket>): Promise<Ticket> => {
    const response = await api.put(`/tickets/${zohoId}`, data);
    return response.data;
  },

  sendMessage: async (zohoId: string, content: string, channel: 'whatsapp' | 'email' | 'note'): Promise<TicketMessage> => {
    const response = await api.post(`/tickets/${zohoId}/messages`, { content, channel });
    return response.data;
  },

  getContacts: async (): Promise<Contact[]> => {
    const response = await api.get('/tickets/contacts');
    return response.data;
  },

  updateContact: async (id: number, data: { name?: string; company_name?: string; parent_contact_id?: number | null }): Promise<Contact> => {
    const response = await api.put(`/tickets/contacts/${id}`, data);
    return response.data;
  },

  getWahaStatus: async (): Promise<{ status: string; qr?: { image: string } }> => {
    const response = await api.get('/tickets/waha/status');
    return response.data;
  },
};
