import api from './client'
import type { Task, PaginatedResponse, CreateTaskInput, TaskStatusHistoryEntry, TaskGateRequirement } from '../types'

export const taskService = {
  getAll: async (params?: { 
    status?: string; 
    priority?: string; 
    page?: number; 
    limit?: number; 
    assignee_id?: string;
    board_id?: number;
    start_date?: string;
    end_date?: string;
    company_id?: number;
  }) => {
    const { data } = await api.get<PaginatedResponse<Task>>('/tasks', { params })
    return data
  },
  getById: async (id: number) => {
    const { data } = await api.get<Task>(`/tasks/${id}`)
    return data
  },
  // Per-board task counts grouped by status: { [boardId]: { [status]: count } }.
  // Aggregated server-side (no full task list download).
  getBoardStatusCounts: async (companyId?: number | null) => {
    const params = companyId ? { company_id: companyId } : undefined
    const { data } = await api.get<Record<number, Record<string, number>>>('/tasks/status-counts', { params })
    return data
  },
  // Bitácora de movimientos entre columnas, de la más reciente a la más antigua.
  // Vive en su propio endpoint y no dentro de getById porque sólo la mira quien
  // abre el historial, y una tarjeta muy movida acumula bastantes filas.
  getHistory: async (id: number) => {
    const { data } = await api.get<TaskStatusHistoryEntry[]>(`/tasks/${id}/history`)
    return data
  },
  create: async (taskData: CreateTaskInput) => {
    const { data } = await api.post<Task>('/tasks', taskData)
    return data
  },
  // `gate` lleva el formulario cuando la columna destino tiene una puerta. Se envía
  // aparte de los campos de la tarea porque no es parte de ella: es lo que se aportó
  // para poder moverla, y el servidor lo guarda en el historial, no en la tarjeta.
  update: async (id: number, taskData: Partial<Task>, gate?: Record<string, unknown>) => {
    const body = gate ? { ...taskData, gate } : taskData
    const { data } = await api.put<Task>(`/tasks/${id}`, body)
    return data
  },
  delete: async (id: number) => {
    await api.delete(`/tasks/${id}`)
  },
  // Persiste el orden manual de las tarjetas de una columna (board + status):
  // cada tarea recibe order = índice en ordered_ids.
  reorder: async (boardId: number, status: string, orderedIds: number[]) => {
    await api.put('/tasks/reorder', { board_id: boardId, status, ordered_ids: orderedIds })
  },
  toggleCompletion: async (id: number) => {
    const { data } = await api.post<Task>(`/tasks/${id}/toggle-completion`)
    return data
  },
  addComment: async (id: number, content: string) => {
    const { data } = await api.post(`/tasks/${id}/comments`, { content })
    return data
  },
  addAttachment: async (id: number, file: File) => {
    const formData = new FormData()
    formData.append('file', file)
    const { data } = await api.post(`/tasks/${id}/attachments`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
    return data
  },
  deleteAttachment: async (taskId: number, attachmentId: number) => {
    await api.delete(`/tasks/${taskId}/attachments/${attachmentId}`)
  },
}

/**
 * Extrae el requisito de puerta de un error de axios, o null si el error era otra
 * cosa. Se comprueba el 422 Y la forma del cuerpo: un 422 de otro origen no debe
 * acabar abriendo un modal vacío.
 */
export function gateFromError(err: unknown): TaskGateRequirement | null {
  const res = (err as { response?: { status?: number; data?: { gate?: TaskGateRequirement } } })?.response
  if (res?.status !== 422) return null
  const gate = res.data?.gate
  if (!gate || !Array.isArray(gate.form?.fields)) return null
  return gate
}
