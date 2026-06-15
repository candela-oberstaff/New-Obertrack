import api from './client';

export interface EmailTemplate {
  id?: number;
  title: string;
  subject: string;
  content: string; // JSON string of blocks
  type: string;
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

export const emailService = {
  // Templates
  getTemplates: async () => {
    const response = await api.get('/email/templates');
    return response.data;
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
  getCampaigns: async () => {
    const response = await api.get('/email/campaigns');
    return response.data;
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
};
