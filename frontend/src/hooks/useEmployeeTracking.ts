import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { adminService } from '../services/api'
import type { EmployeeTracking } from '../types'

interface UseEmployeeTrackingReturn {
  tracking: EmployeeTracking | null
  isLoading: boolean
  error: string | null
  refresh: () => Promise<void>
  toggleStatus: () => Promise<void>
  resetPassword: (newPassword: string) => Promise<void>
}

export function useEmployeeTracking(id: number): UseEmployeeTrackingReturn {
  const qc = useQueryClient()
  const queryKey = ['employee-tracking', id]

  const { data, isLoading, error, refetch } = useQuery({
    queryKey,
    queryFn: async () => {
      const res = await adminService.getEmployeeTracking(id)
      return {
        ...res,
        work_hours: Array.isArray(res?.work_hours) ? res.work_hours : [],
        tasks: Array.isArray(res?.tasks) ? res.tasks : [],
      } as EmployeeTracking
    },
    enabled: !!id,
  })

  const toggleMut = useMutation({
    mutationFn: (nextActive: boolean) => adminService.updateUser(id, { is_active: nextActive }),
    onSuccess: () => qc.invalidateQueries({ queryKey }),
  })

  return {
    tracking: data ?? null,
    isLoading,
    error: error ? 'No se pudo cargar el empleado' : null,
    refresh: async () => { await refetch() },
    toggleStatus: async () => {
      if (!data) return
      await toggleMut.mutateAsync(!data.user.is_active)
    },
    resetPassword: async (newPassword: string) => {
      await adminService.resetPassword(id, newPassword)
    },
  }
}
