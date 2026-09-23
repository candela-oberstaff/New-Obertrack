import api from './client';

export interface SurveyQuestion {
  id?: number;
  text: string;
  type: 'text' | 'rating' | 'choice' | 'checkbox' | 'dropdown' | 'linear_scale' | 'grid' | 'checkbox_grid';
  options?: string; // JSON string of choices
  is_required: boolean;
  order_index: number;
  /**
   * Clave de respuesta de un cuestionario de inducción. Solo la recibe el
   * superadmin: el backend la borra para quien responde. Vacío = no puntúa.
   */
  correct_answer?: string;
  /** Ponderación de la pregunta dentro del puntaje. */
  weight?: number;
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
  /** 'feedback' (opinión) o 'induction' (calificado, decide el acceso). */
  kind?: 'feedback' | 'induction';
  /** Mínimo aprobatorio en porcentaje. Solo aplica a las de tipo induction. */
  passing_score?: number;
}

/**
 * Vista de la encuesta que se responde SIN sesión, con el enlace que la empresa
 * recibió por correo. Es un DTO propio del backend: no trae respuestas de otros
 * ni la clave de un cuestionario calificado.
 */
export interface PublicSurvey {
  survey_id: number;
  title: string;
  description: string;
  recipient_name: string;
  questions: SurveyQuestion[];
  /** Ya hay algo respondido — normalmente el clic que dio en el propio correo. */
  already_answered: boolean;
}

/** Una respuesta con la forma que espera el backend. */
export interface SurveyAnswerPayload {
  question_id: number;
  text_value?: string;
  number_value?: number;
}

/**
 * Da forma a lo contestado para enviarlo. Vive aquí y no en una pantalla porque
 * lo usan las dos que responden encuestas —la interna y la pública—, y el
 * criterio de qué va como número y qué como texto tiene que ser el mismo: si se
 * separan, los resultados de una misma encuesta salen mezclados.
 */
export function buildAnswerPayload(
  questions: SurveyQuestion[],
  answers: Record<number, any>,
): SurveyAnswerPayload[] {
  return Object.entries(answers).map(([qId, value]) => {
    const questionId = parseInt(qId);
    const q = questions.find(sq => sq.id === questionId);
    if (q?.type === 'rating' || q?.type === 'linear_scale') {
      return { question_id: questionId, number_value: Number(value) };
    }
    // Arrays y objetos (checkbox, grid, checkbox_grid) → JSON serializado
    if (Array.isArray(value) || (typeof value === 'object' && value !== null)) {
      return { question_id: questionId, text_value: JSON.stringify(value) };
    }
    return { question_id: questionId, text_value: String(value) };
  });
}

/**
 * Encuesta sin sesión. El token del enlace es la credencial, así que estas dos
 * llamadas no dependen de la cookie de sesión (el backend solo las acepta para
 * cuentas empresa).
 */
export const publicSurveyService = {
  get: async (token: string): Promise<PublicSurvey> => {
    const response = await api.get(`/survey-link/${token}`);
    return response.data;
  },

  submit: async (token: string, answers: SurveyAnswerPayload[]) => {
    const response = await api.post(`/survey-link/${token}/responses`, { answers });
    return response.data;
  },
};

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
  /**
   * Envía la encuesta a otros destinatarios sin tocar la lista guardada.
   * recipientIds es un array plano de IDs de usuario.
   */
  sendSurveyToRecipients: async (id: number, recipientIds: number[]) => {
    const response = await api.post(`/surveys/${id}/send`, {
      recipient_list: JSON.stringify(recipientIds),
    });
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
