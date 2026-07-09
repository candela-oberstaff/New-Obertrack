import { useCallback } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { boardService } from '../../../services/api'
import type { BoardInvitation } from '../../../types'

interface UseBoardInvitationsOptions {
  boardId?: number | null
  canManage?: boolean
  companyId?: number | null
}

interface UseBoardInvitationsReturn {
  myInvitations: BoardInvitation[]
  boardRequests: BoardInvitation[]
  boardInvitations: BoardInvitation[]
  isLoading: boolean
  accept: (invId: number) => Promise<void>
  reject: (invId: number) => Promise<void>
  cancel: (invId: number) => Promise<void>
  refetchAll: () => Promise<void>
}

export function useBoardInvitations({
  boardId = null,
  canManage = false,
  companyId = null,
}: UseBoardInvitationsOptions = {}): UseBoardInvitationsReturn {
  const qc = useQueryClient()

  const myQ = useQuery({
    queryKey: ['board-invitations', 'mine'],
    queryFn: () => boardService.getMyInvitations(),
  })

  const requestsQ = useQuery({
    queryKey: ['board-requests', boardId],
    queryFn: () => boardService.getBoardRequests(boardId as number),
    enabled: !!boardId && canManage,
  })

  const invitesQ = useQuery({
    queryKey: ['board-sent-invitations', boardId],
    queryFn: () => boardService.getBoardInvitations(boardId as number),
    enabled: !!boardId && canManage,
  })

  const refetchAll = useCallback(async () => {
    await Promise.all([
      qc.invalidateQueries({ queryKey: ['board-invitations', 'mine'] }),
      qc.invalidateQueries({ queryKey: ['board-requests', boardId] }),
      qc.invalidateQueries({ queryKey: ['board-sent-invitations', boardId] }),
      qc.invalidateQueries({ queryKey: ['boards', companyId] }),
    ])
  }, [qc, boardId, companyId])

  const accept = useCallback(async (invId: number) => {
    await boardService.acceptInvitation(invId)
    await refetchAll()
  }, [refetchAll])

  const reject = useCallback(async (invId: number) => {
    await boardService.rejectInvitation(invId)
    await refetchAll()
  }, [refetchAll])

  const cancel = useCallback(async (invId: number) => {
    await boardService.cancelInvitation(invId)
    await refetchAll()
  }, [refetchAll])

  return {
    myInvitations: myQ.data ?? [],
    boardRequests: requestsQ.data ?? [],
    boardInvitations: invitesQ.data ?? [],
    isLoading: myQ.isLoading || requestsQ.isLoading,
    accept,
    reject,
    cancel,
    refetchAll,
  }
}
