import api from './client'
import type { Tutorial, CreateTutorialInput, UpdateTutorialInput } from '../types'

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
  recordView: async (id: number) => {
    await api.post(`/tutorials/${id}/view`)
  },
  getMyViews: async () => {
    const { data } = await api.get<ViewedIdsResponse>('/tutorials/views')
    return data.data || []
  },
}
