import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { adminService } from '../services/api'
import type { FollowUpInfo } from './useAdmin'

/**
 * Bitácora de gestión de customer success: estado vigente por profesional.
 *
 * Comparte clave de consulta con el panel de administración a propósito, para
 * que la ficha de una empresa y el panel enseñen la misma gestión sin pedirla
 * dos veces ni quedar desincronizados al anotar desde cualquiera de los dos.
 */
export function useFollowUps(kind: 'inactivity' | 'absence', enabled = true) {
  const qc = useQueryClient()

  const q = useQuery({
    queryKey: ['admin', 'follow-ups', kind],
    queryFn: async () => {
      const items: FollowUpInfo[] = await adminService.getFollowUps(kind)
      return Object.fromEntries(items.map(i => [i.user_id, i])) as Record<number, FollowUpInfo>
    },
    enabled,
  })

  const mut = useMutation({
    mutationFn: (payload: { user_id: number; status: string }) =>
      adminService.createFollowUp({ ...payload, kind }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['admin', 'follow-ups', kind] }) },
  })

  return {
    followUps: q.data ?? {},
    setFollowUp: (userId: number, status: string) => { mut.mutate({ user_id: userId, status }) },
  }
}
