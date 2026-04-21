import { useState, useCallback, useEffect } from 'react'
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
  newBoardData: CreateBoardInput
  setNewBoardData: React.Dispatch<React.SetStateAction<CreateBoardInput>>
}

const DEFAULT_PHASES = [
  { name: 'Por hacer', color: '#6b7280' },
  { name: 'En proceso', color: 'var(--primary)' },
  { name: 'Finalizado', color: '#22c55e' },
]

export function useBoards(): UseBoardsReturn {
  const [boards, setBoards] = useState<Board[]>([])
  const [selectedBoard, setSelectedBoard] = useState<Board | null>(null)
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

  const fetchBoards = useCallback(async () => {
    setIsLoading(true)
    try {
      const boardsRes = await boardService.getAll()
      // Deduplicate by ID to avoid React key warnings and duplicates in rendering
      const unique = (boardsRes || []).filter(
        (b: Board, idx: number, arr: Board[]) => arr.findIndex((x: Board) => x.id === b.id) === idx
      )
      setBoards(unique)
      return unique
    } catch (error) {
      console.error('Error fetching boards:', error)
      return []
    } finally {
      setIsLoading(false)
    }
  }, [])

  const fetchPublicBoards = useCallback(async () => {
    try {
      const res = await boardService.getPublicBoards()
      setPublicBoards(res || [])
    } catch (error) {
      console.error('Error fetching public boards:', error)
    }
  }, [])

  useEffect(() => {
    fetchBoards()
  }, [fetchBoards])

  const createBoard = useCallback(async (data: CreateBoardInput): Promise<Board | null> => {
    if (!data.name?.trim()) return null
    setIsCreatingBoard(true)
    try {
      const newBoard = await boardService.create(data)
      setNewBoardData({
        name: '',
        description: '',
        color: 'var(--primary)',
        member_ids: [],
        phases: DEFAULT_PHASES,
      })
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
  }, [fetchBoards])

  const deleteBoard = useCallback(async (boardId: number) => {
    await boardService.delete(boardId)
    await fetchBoards()
    if (selectedBoard?.id === boardId) {
      setSelectedBoard(null)
    }
  }, [fetchBoards, selectedBoard])

  const joinBoard = useCallback(async (boardId: number): Promise<boolean> => {
    try {
      await boardService.join(boardId)
      await fetchBoards()
      return true
    } catch (error: any) {
      console.error('Error joining board:', error)
      return false
    }
  }, [fetchBoards])

  const updateBoardMembers = useCallback(async (boardId: number, memberIds: number[]) => {
    await boardService.update(boardId, { member_ids: memberIds })
    await fetchBoards()
  }, [fetchBoards])

  const reorderPhases = useCallback(async (boardId: number, phaseIds: number[]) => {
    await boardService.reorderPhases(boardId, phaseIds)
    await fetchBoards()
  }, [fetchBoards])

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
    newBoardData,
    setNewBoardData,
  }
}
