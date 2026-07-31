import api from './client';

export interface EmailTemplate {
  id?: number;
  title: string;
  subject: string;
  content: string; // JSON string of blocks
  type: string;
  is_active?: boolean;
  created_at?: string;
}

export interface EmailCampaign {
  id?: number;
  template_id?: number;
  title: string;
  subject: string;
  status: string;
  recipients?: number;
  open_rate?: number;
  click_rate?: number;
  created_at?: string;
  sent_at?: string;
  scheduled_at?: string;
  recipient_list?: string;
  template?: EmailTemplate;
}

/** Variable de personalización ({{nombre}}, {{empresa}}, ...). El catálogo lo
 *  define el backend en utils/email_vars.go — no lo dupliques aquí. */
export interface EmailVariable {
  key: string;
  label: string;
  description: string;
  example: string;
  fallback: string;
  group: string;
}

export interface QuickEmailPayload {
  to_email: string;
  to_name?: string;
  subject: string;
  html_content: string;
}

export interface BulkEmailPayload {
  /** Contactos sueltos que no son usuarios: solo se personaliza nombre y correo. */
  recipients?: Array<{ name: string; email: string }>;
  /**
   * Formato híbrido JSON ({userIds, groupIds, expressContacts}). El backend lo
   * resuelve contra la base de datos, así que habilita TODAS las variables.
   */
  recipient_list?: string;
  subject: string;
  html_content: string;
}

/**
 * Prueba de un correo antes de enviarlo de verdad. Solo llega a la dirección de
 * quien la pide; el backend ignora cualquier otro destinatario.
 *
 * `blocks` y `template_id` se componen con el MISMO renderizador del envío
 * real, así que el resultado es fiel; `html_content` es para el texto suelto.
 */
export interface TestEmailPayload {
  subject: string;
  template_id?: number;
  blocks?: string;
  html_content?: string;
  /** Usuario cuyos datos resuelven las variables. Sin él, valores de ejemplo. */
  as_user_id?: number;
  /**
   * Dónde llega la prueba. Vacío = tu propio correo. Va a UNA sola dirección
   * por petición y siempre con [PRUEBA] en el asunto.
   */
  to_email?: string;
}

export const emailService = {
  sendTestEmail: async (payload: TestEmailPayload): Promise<{ to: string; viewed_as: string }> => {
    const response = await api.post('/email/test-send', payload);
    return response.data;
  },

  // Variables de personalización
  getVariables: async (): Promise<EmailVariable[]> => {
    const response = await api.get('/email/variables');
    return response.data?.variables || [];
  },

  // Templates
  getTemplates: async (): Promise<EmailTemplate[]> => {
    const response = await api.get('/email/templates');
    return response.data || [];
  },
  createTemplate: async (template: Partial<EmailTemplate>) => {
    const response = await api.post('/email/templates', template);
    return response.data;
  },
  updateTemplate: async (id: number, template: Partial<EmailTemplate>) => {
    const response = await api.put(`/email/templates/${id}`, template);
    return response.data;
  },
  deleteTemplate: async (id: number) => {
    const response = await api.delete(`/email/templates/${id}`);
    return response.data;
  },

  // Campaigns
  getCampaigns: async (): Promise<EmailCampaign[]> => {
    const response = await api.get('/email/campaigns');
    return response.data || [];
  },
  createCampaign: async (campaign: Partial<EmailCampaign>) => {
    const response = await api.post('/email/campaigns', campaign);
    return response.data;
  },
  updateCampaign: async (id: number, campaign: Partial<EmailCampaign>) => {
    const response = await api.put(`/email/campaigns/${id}`, campaign);
    return response.data;
  },
  sendCampaign: async (id: number) => {
    const response = await api.post(`/email/campaigns/${id}/send`, {});
    return response.data;
  },
  sendCampaignToRecipients: async (id: number, recipientList: string) => {
    const response = await api.post(`/email/campaigns/${id}/send`, { recipient_list: recipientList });
    return response.data;
  },
  deleteCampaign: async (id: number) => {
    const response = await api.delete(`/email/campaigns/${id}`);
    return response.data;
  },
  getAvailableRecipients: async () => {
    const response = await api.get('/users?limit=1000');
    return response.data;
  },
  getCampaignEvents: async (id: number) => {
    const response = await api.get(`/email/campaigns/${id}/events`);
    return response.data;
  },

  // Quick send (ad-hoc, no campaign needed)
  sendQuickEmail: async (payload: QuickEmailPayload) => {
    const response = await api.post('/email/quick-send', payload);
    return response.data;
  },
  sendQuickEmailBulk: async (payload: BulkEmailPayload) => {
    const response = await api.post('/email/quick-send-bulk', payload);
    return response.data;
  },
  sendTemplate: async (id: number, recipientList: string) => {
    const response = await api.post(`/email/templates/${id}/send`, { recipient_list: recipientList });
    return response.data;
  },
};
