import api from './client';

export interface SurveyQuestion {
  id?: number;
  text: string;
  type: 'text' | 'rating' | 'choice' | 'checkbox' | 'dropdown' | 'linear_scale' | 'grid' | 'checkbox_grid';
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
    const response = await api.get('/surveys');
    return response.data;
  },

  createSurvey: async (survey: Partial<Survey>) => {
    const response = await api.post('/surveys', survey);
    return response.data;
  },

  getSurvey: async (id: number) => {
    const response = await api.get(`/surveys/${id}`);
    return response.data;
  },

  sendSurvey: async (id: number) => {
    const response = await api.post(`/surveys/${id}/send`, {});
    return response.data;
  },

  submitResponse: async (id: number, answers: any[]) => {
    const response = await api.post(`/surveys/${id}/responses`, { answers });
    return response.data;
  },

  updateSurvey: async (id: number, survey: Partial<Survey>) => {
    const response = await api.put(`/surveys/${id}`, survey);
    return response.data;
  },

  deleteSurvey: async (id: number) => {
    const response = await api.delete(`/surveys/${id}`);
    return response.data;
  },
};
