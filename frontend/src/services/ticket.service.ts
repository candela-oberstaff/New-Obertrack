import api from './api';

export interface Contact {
  id: number;
  name: string;
  phone: string;
  email: string;
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
  contact_id: number;
  contact?: Contact;
  title: string;
  stage: 'new' | 'in_progress' | 'waiting' | 'closed';
  status: 'open' | 'closed';
  assigned_to?: number;
  messages?: TicketMessage[];
  created_at: string;
  updated_at: string;
}

export const ticketService = {
  getTickets: async (): Promise<Ticket[]> => {
    const response = await api.get('/tickets');
    return response.data;
  },

  getTicket: async (id: number): Promise<Ticket> => {
    const response = await api.get(`/tickets/${id}`);
    return response.data;
  },

  updateTicket: async (id: number, data: Partial<Ticket>): Promise<Ticket> => {
    const response = await api.put(`/tickets/${id}`, data);
    return response.data;
  },

  sendMessage: async (id: number, content: string, channel: 'whatsapp' | 'email' | 'note'): Promise<TicketMessage> => {
    const response = await api.post(`/tickets/${id}/messages`, { content, channel });
    return response.data;
  },

  simulateWahaMessage: async (phone: string, body: string): Promise<any> => {
    const response = await api.post('/webhooks/waha', {
      event: 'message',
      session: 'default',
      payload: {
        id: 'false_' + phone + '@c.us_3A' + Date.now(),
        from: phone + '@c.us',
        to: 'me',
        body: body,
        type: 'chat',
        fromMe: false,
        timestamp: Math.floor(Date.now() / 1000)
      }
    });
    return response.data;
  },

  getWahaStatus: async (): Promise<{ status: string; qr?: { image: string } }> => {
    const response = await api.get('/tickets/waha/status');
    return response.data;
  }
};
