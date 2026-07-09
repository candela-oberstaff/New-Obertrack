import api from './client'
import type { Board, BoardInvitation, CreateBoardInput } from '../types'

export const boardService = {
  getAll: async (companyId?: number | null) => {
    const params = companyId ? { company_id: companyId } : undefined
    const { data } = await api.get<Board[]>('/boards', { params })
    return data
  },
  getById: async (id: number) => {
    const { data } = await api.get<Board>(`/boards/${id}`)
    return data
  },
  create: async (boardData: CreateBoardInput, companyId?: number | null) => {
    const params = companyId ? { company_id: companyId } : undefined
    const { data } = await api.post<Board>('/boards', boardData, { params })
    return data
  },
  update: async (id: number, boardData: CreateBoardInput) => {
    const { data } = await api.put<Board>(`/boards/${id}`, boardData)
    return data
  },
  delete: async (id: number) => {
    await api.delete(`/boards/${id}`)
  },
  addPhase: async (boardId: number, phase: { name: string; color?: string }) => {
    const { data } = await api.post<Board>(`/boards/${boardId}/phases`, phase)
    return data
  },
  removePhase: async (boardId: number, phaseId: number) => {
    const { data } = await api.delete<Board>(`/boards/${boardId}/phases/${phaseId}`)
    return data
  },
  reorderPhases: async (boardId: number, phaseIds: number[]) => {
    const { data } = await api.put<Board>(`/boards/${boardId}/phases/reorder`, { phase_ids: phaseIds })
    return data
  },
  getPublicBoards: async (companyId?: number | null) => {
    const params = companyId ? { company_id: companyId } : undefined
    const { data } = await api.get<Board[]>('/boards/public', { params })
    return data
  },
  invite: async (boardId: number, userIds: number[]) => {
    const { data } = await api.post<{ invited: number; invitations: BoardInvitation[] }>(
      `/boards/${boardId}/invite`,
      { user_ids: userIds },
    )
    return data
  },
  requestJoin: async (boardId: number) => {
    const { data } = await api.post<BoardInvitation>(`/boards/${boardId}/request`)
    return data
  },
  getMyInvitations: async () => {
    const { data } = await api.get<BoardInvitation[]>('/board-invitations/mine')
    return data
  },
  getBoardRequests: async (boardId: number) => {
    const { data } = await api.get<BoardInvitation[]>(`/boards/${boardId}/requests`)
    return data
  },
  getBoardInvitations: async (boardId: number) => {
    const { data } = await api.get<BoardInvitation[]>(`/boards/${boardId}/invitations`)
    return data
  },
  acceptInvitation: async (invId: number) => {
    const { data } = await api.post<BoardInvitation>(`/board-invitations/${invId}/accept`)
    return data
  },
  rejectInvitation: async (invId: number) => {
    const { data } = await api.post<BoardInvitation>(`/board-invitations/${invId}/reject`)
    return data
  },
  cancelInvitation: async (invId: number) => {
    await api.delete(`/board-invitations/${invId}`)
  },
  removeMember: async (boardId: number, userId: number) => {
    const { data } = await api.delete<Board>(`/boards/${boardId}/members/${userId}`)
    return data
  },
  leaveBoard: async (boardId: number) => {
    await api.post(`/boards/${boardId}/leave`)
  },
}
