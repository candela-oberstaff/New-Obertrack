import { useState, useCallback, useEffect } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { boardService } from '../../../services/api'
import type { Board, CreateBoardInput } from '../../../types'

interface UseBoardsReturn {
  boards: Board[]
  selectedBoard: Board | null
  setSelectedBoard: React.Dispatch<React.SetStateAction<Board | null>>
  publicBoards: Board[]
  isLoading: boolean
  isCreatingBoard: boolean
  createBoard: (data: CreateBoardInput) => Promise<Board | null>
  deleteBoard: (boardId: number) => Promise<void>
  joinBoard: (boardId: number) => Promise<boolean>
  fetchBoards: () => Promise<Board[]>
  fetchPublicBoards: () => Promise<void>
  updateBoardMembers: (boardId: number, memberIds: number[]) => Promise<void>
  reorderPhases: (boardId: number, phaseIds: number[]) => Promise<void>
  addPhase: (boardId: number, phase: { name: string; color?: string }) => Promise<void>
  removePhase: (boardId: number, phaseId: number) => Promise<void>
  newBoardData: CreateBoardInput
  setNewBoardData: React.Dispatch<React.SetStateAction<CreateBoardInput>>
}

const DEFAULT_PHASES = [
  { name: 'Por hacer', color: '#6b7280' },
  { name: 'En proceso', color: 'var(--primary)' },
  { name: 'Finalizado', color: '#22c55e' },
]

interface UseBoardsOptions {
  // For superadmin: the company (tenant) currently selected. Boards are only
  // fetched once a company is chosen, so tenants never get mixed in the view.
  companyId?: number | null
  // When true, the caller must provide a companyId before any board is fetched.
  requireCompany?: boolean
  // When false, no board is auto-selected after fetching: the caller lands on a
  // board picker instead of jumping straight into the first board.
  autoSelectFirst?: boolean
}

export function useBoards({ companyId = null, requireCompany = false, autoSelectFirst = true }: UseBoardsOptions = {}): UseBoardsReturn {
  const qc = useQueryClient()
  // Cache key for the boards list. Only the GET is routed through the cache; all
  // the selection logic below is unchanged.
  const boardsQueryKey = ['boards', companyId] as const
  const [boards, setBoards] = useState<Board[]>([])
  const [selectedBoard, setSelectedBoardState] = useState<Board | null>(null)
  const [publicBoards, setPublicBoards] = useState<Board[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [isCreatingBoard, setIsCreatingBoard] = useState(false)
  const [newBoardData, setNewBoardData] = useState<CreateBoardInput>({
    name: '',
    description: '',
    color: 'var(--primary)',
    member_ids: [],
    phases: DEFAULT_PHASES,
  })

  const setSelectedBoard = useCallback((value: Board | null | ((prev: Board | null) => Board | null)) => {
    setSelectedBoardState((current) => {
      const next = typeof value === 'function' ? (value as any)(current) : value
      if (next) {
        localStorage.setItem('preferred_board_id', String(next.id))
      } else {
        localStorage.removeItem('preferred_board_id')
      }
      return next
    })
  }, [])

  const fetchBoards = useCallback(async () => {
    // Superadmin without a selected company: do not fetch anything to avoid
    // mixing boards from different tenants.
    if (requireCompany && !companyId) {
      setBoards([])
      setSelectedBoardState(null)
      return []
    }
    setIsLoading(true)
    try {
      // Read through the React Query cache: revisiting the page returns the
      // cached list instantly (within staleTime) while staying fresh after
      // mutations, which invalidate this key.
      const boardsRes = await qc.fetchQuery({
        queryKey: boardsQueryKey,
        queryFn: () => boardService.getAll(companyId),
      })
      // Deduplicate by ID to avoid React key warnings and duplicates in rendering
      const unique = (boardsRes || []).filter(
        (b: Board, idx: number, arr: Board[]) => arr.findIndex((x: Board) => x.id === b.id) === idx
      )
      setBoards(unique)

      setSelectedBoardState((current) => {
        if (current) {
          const stillExists = unique.find((b: Board) => b.id === current.id)
          if (stillExists) return stillExists
        }

        // When auto-select is disabled (e.g. superadmin entering a company), land
        // on the board picker instead of jumping into a board.
        if (!autoSelectFirst) {
          return null
        }

        const preferredId = localStorage.getItem('preferred_board_id')
        if (preferredId) {
          const found = unique.find((b: Board) => b.id === Number(preferredId))
          if (found) return found
        }

        if (unique.length > 0) {
          return unique[0]
        }

        return null
      })

      return unique
    } catch (error) {
      console.error('Error fetching boards:', error)
      return []
    } finally {
      setIsLoading(false)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [companyId, requireCompany, autoSelectFirst, qc])

  // Marks the cached boards list stale so the next fetchBoards() refetches.
  const invalidateBoards = useCallback(
    () => qc.invalidateQueries({ queryKey: boardsQueryKey }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [qc, companyId],
  )

  const fetchPublicBoards = useCallback(async () => {
    try {
      const res = await boardService.getPublicBoards(companyId)
      setPublicBoards(res || [])
    } catch (error) {
      console.error('Error fetching public boards:', error)
    }
  }, [companyId])

  useEffect(() => {
    fetchBoards()
  }, [fetchBoards])

  const createBoard = useCallback(async (data: CreateBoardInput): Promise<Board | null> => {
    if (!data.name?.trim()) return null
    setIsCreatingBoard(true)
    try {
      const newBoard = await boardService.create(data, companyId)
      setNewBoardData({
        name: '',
        description: '',
        color: 'var(--primary)',
        member_ids: [],
        phases: DEFAULT_PHASES,
      })
      await invalidateBoards()
      const boardsRes = await fetchBoards()
      const found = boardsRes.find((b: Board) => b.id === newBoard.id)
      if (found) setSelectedBoard(found)
      return newBoard
    } catch (error) {
      console.error('Error creating board:', error)
      return null
    } finally {
      setIsCreatingBoard(false)
    }
  }, [fetchBoards, companyId, invalidateBoards])

  const deleteBoard = useCallback(async (boardId: number) => {
    await boardService.delete(boardId)
    await invalidateBoards()
    await fetchBoards()
    if (selectedBoard?.id === boardId) {
      setSelectedBoard(null)
    }
  }, [fetchBoards, selectedBoard, invalidateBoards])

  const joinBoard = useCallback(async (boardId: number): Promise<boolean> => {
    try {
      await boardService.join(boardId)
      await invalidateBoards()
      const boardsRes = await fetchBoards()
      const found = boardsRes.find((b: Board) => b.id === boardId)
      if (found) setSelectedBoard(found)
      return true
    } catch (error: any) {
      if (error?.response?.status === 409) {
        await invalidateBoards()
        const boardsRes = await fetchBoards()
        const found = boardsRes.find((b: Board) => b.id === boardId)
        if (found) setSelectedBoard(found)
        return true
      }
      console.error('Error joining board:', error)
      return false
    }
  }, [fetchBoards, invalidateBoards])

  const updateBoardMembers = useCallback(async (boardId: number, memberIds: number[]) => {
    await boardService.update(boardId, { member_ids: memberIds })
    await invalidateBoards()
    await fetchBoards()
  }, [fetchBoards, invalidateBoards])

  const reorderPhases = useCallback(async (boardId: number, phaseIds: number[]) => {
    await boardService.reorderPhases(boardId, phaseIds)
    await invalidateBoards()
    await fetchBoards()
  }, [fetchBoards, invalidateBoards])

  const addPhase = useCallback(async (boardId: number, phase: { name: string; color?: string }) => {
    const updated = await boardService.addPhase(boardId, phase)
    setBoards((prev) => prev.map((b) => (b.id === boardId ? updated : b)))
    setSelectedBoardState((current) => (current?.id === boardId ? updated : current))
    invalidateBoards()
  }, [invalidateBoards])

  const removePhase = useCallback(async (boardId: number, phaseId: number) => {
    const updated = await boardService.removePhase(boardId, phaseId)
    setBoards((prev) => prev.map((b) => (b.id === boardId ? updated : b)))
    setSelectedBoardState((current) => (current?.id === boardId ? updated : current))
    invalidateBoards()
  }, [invalidateBoards])

  return {
    boards,
    selectedBoard,
    setSelectedBoard,
    publicBoards,
    isLoading,
    isCreatingBoard,
    createBoard,
    deleteBoard,
    joinBoard,
    fetchBoards,
    fetchPublicBoards,
    updateBoardMembers,
    reorderPhases,
    addPhase,
    removePhase,
    newBoardData,
    setNewBoardData,
  }
}
