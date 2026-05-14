import axios from 'axios';

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080/api';

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
    const response = await axios.get(`${API_URL}/email/templates`, {
      headers: { Authorization: `Bearer ${localStorage.getItem('token')}` }
    });
    return response.data;
  },
  createTemplate: async (template: Partial<EmailTemplate>) => {
    const response = await axios.post(`${API_URL}/email/templates`, template, {
      headers: { Authorization: `Bearer ${localStorage.getItem('token')}` }
    });
    return response.data;
  },
  updateTemplate: async (id: number, template: Partial<EmailTemplate>) => {
    const response = await axios.put(`${API_URL}/email/templates/${id}`, template, {
      headers: { Authorization: `Bearer ${localStorage.getItem('token')}` }
    });
    return response.data;
  },

  // Campaigns
  getCampaigns: async () => {
    const response = await axios.get(`${API_URL}/email/campaigns`, {
      headers: { Authorization: `Bearer ${localStorage.getItem('token')}` }
    });
    return response.data;
  },
  createCampaign: async (campaign: Partial<EmailCampaign>) => {
    const response = await axios.post(`${API_URL}/email/campaigns`, campaign, {
      headers: { Authorization: `Bearer ${localStorage.getItem('token')}` }
    });
    return response.data;
  },
  updateCampaign: async (id: number, campaign: Partial<EmailCampaign>) => {
    const response = await axios.put(`${API_URL}/email/campaigns/${id}`, campaign, {
      headers: { Authorization: `Bearer ${localStorage.getItem('token')}` }
    });
    return response.data;
  },
  sendCampaign: async (id: number) => {
    const response = await axios.post(`${API_URL}/email/campaigns/${id}/send`, {}, {
      headers: { Authorization: `Bearer ${localStorage.getItem('token')}` }
    });
    return response.data;
  },
  getAvailableRecipients: async () => {
    // Increase limit to ensure we get all potential recipients instead of the default 10
    const response = await axios.get(`${API_URL}/users?limit=1000`, {
      headers: { Authorization: `Bearer ${localStorage.getItem('token')}` }
    });
    return response.data;
  }
};
