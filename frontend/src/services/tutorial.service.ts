import api from './client'
import type {
  Tutorial,
  CreateTutorialInput,
  UpdateTutorialInput,
  TutorialMetrics,
  TutorialViewSource,
  TutorialTarget,
  TutorialAudience,
  TutorialAudienceOptions,
  TutorialAudiencePreview,
} from '../types'

interface TutorialListResponse {
  data: Tutorial[]
}

interface ViewedIdsResponse {
  data: number[]
}

export const tutorialService = {
  getAll: async () => {
    const { data } = await api.get<TutorialListResponse>('/tutorials')
    return data.data
  },
  getById: async (id: number) => {
    const { data } = await api.get<Tutorial>(`/tutorials/${id}`)
    return data
  },
  create: async (payload: CreateTutorialInput) => {
    const { data } = await api.post<Tutorial>('/tutorials', payload)
    return data
  },
  update: async (id: number, payload: UpdateTutorialInput) => {
    const { data } = await api.put<Tutorial>(`/tutorials/${id}`, payload)
    return data
  },
  delete: async (id: number) => {
    await api.delete(`/tutorials/${id}`)
  },
  reorder: async (ids: number[]) => {
    await api.post('/tutorials/reorder', { ids })
  },
  /** Registra la vista. El origen alimenta las métricas de la novedad. */
  recordView: async (id: number, source: TutorialViewSource = 'seccion', acknowledged = false) => {
    await api.post(`/tutorials/${id}/view`, { source, acknowledged })
  },

  /** Anota que se pulsó el botón de acción de la novedad. */
  recordClick: async (id: number) => {
    await api.post(`/tutorials/${id}/click`, {})
  },

  /** Vuelve a avisar a quienes no la han visto y reabre la ventana del aviso. */
  remindPending: async (id: number) => {
    const { data } = await api.post<{ reminded: number }>(`/tutorials/${id}/remind`, {})
    return data.reminded
  },

  /** Empresas, paises y grupos elegibles como publico. Solo superadmin. */
  getAudienceOptions: async () => {
    const { data } = await api.get<TutorialAudienceOptions>('/tutorials/audience-options')
    return data
  },

  /** A cuanta gente llegaria la novedad con ese publico, sin publicar nada. */
  previewAudience: async (audience: TutorialAudience, target: TutorialTarget) => {
    const { data } = await api.post<TutorialAudiencePreview>('/tutorials/audience-preview', { audience, target })
    return data
  },

  /** Desempeño de una novedad. Solo superadmin. */
  getMetrics: async (id: number) => {
    const { data } = await api.get<TutorialMetrics>(`/tutorials/${id}/metrics`)
    return data
  },
  /**
   * Novedades anunciadas que este usuario todavía no ha visto. Es lo que se
   * muestra en la ventana emergente al entrar. Devuelve [] si la cuenta no
   * tiene acceso al módulo, para que el aviso nunca rompa el arranque.
   */
  getPending: async () => {
    try {
      const { data } = await api.get<TutorialListResponse>('/tutorials/pending')
      return data.data || []
    } catch {
      return []
    }
  },
  getMyViews: async () => {
    const { data } = await api.get<ViewedIdsResponse>('/tutorials/views')
    return data.data || []
  },
}
