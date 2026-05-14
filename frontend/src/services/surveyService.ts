import axios from 'axios';

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080/api';

export interface SurveyQuestion {
  id?: number;
  text: string;
  type: 'text' | 'rating' | 'choice';
  options?: string; // JSON string of choices
  is_required: boolean;
  order_index: number;
}

export interface Survey {
  id?: number;
  title: string;
  description: string;
  status: 'draft' | 'active' | 'closed';
  send_by_email: boolean;
  send_by_inapp: boolean;
  recipient_list: string; // JSON string
  created_at?: string;
  questions?: SurveyQuestion[];
  responses?: any[];
}

export const surveyService = {
  getSurveys: async () => {
    const response = await axios.get(`${API_URL}/surveys`, {
      headers: { Authorization: `Bearer ${localStorage.getItem('token')}` }
    });
    return response.data;
  },

  createSurvey: async (survey: Partial<Survey>) => {
    const response = await axios.post(`${API_URL}/surveys`, survey, {
      headers: { Authorization: `Bearer ${localStorage.getItem('token')}` }
    });
    return response.data;
  },

  getSurvey: async (id: number) => {
    const response = await axios.get(`${API_URL}/surveys/${id}`, {
      headers: { Authorization: `Bearer ${localStorage.getItem('token')}` }
    });
    return response.data;
  },

  sendSurvey: async (id: number) => {
    const response = await axios.post(`${API_URL}/surveys/${id}/send`, {}, {
      headers: { Authorization: `Bearer ${localStorage.getItem('token')}` }
    });
    return response.data;
  },

  submitResponse: async (id: number, answers: any[]) => {
    const response = await axios.post(`${API_URL}/surveys/${id}/responses`, { answers }, {
      headers: { Authorization: `Bearer ${localStorage.getItem('token')}` }
    });
    return response.data;
  },
  updateSurvey: async (id: number, survey: Partial<Survey>) => {
    const response = await axios.put(`${API_URL}/surveys/${id}`, survey, {
      headers: { Authorization: `Bearer ${localStorage.getItem('token')}` }
    });
    return response.data;
  },
};
