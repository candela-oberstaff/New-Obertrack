import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { adminService } from '../services/api'
import type { EmployeeTracking } from '../types'

interface UseEmployeeTrackingReturn {
  tracking: EmployeeTracking | null
  isLoading: boolean
  error: string | null
  refresh: () => Promise<void>
  toggleStatus: () => Promise<void>
  toggleReplacement: () => Promise<void>
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
    onSuccess: () => {
      qc.invalidateQueries({ queryKey })
      qc.invalidateQueries({ queryKey: ['tenant-detail'] })
      qc.invalidateQueries({ queryKey: ['tenant-employees'] })
      qc.invalidateQueries({ queryKey: ['admin-tenants'] })
    },
  })

  const toggleReplacementMut = useMutation({
    mutationFn: (nextVal: boolean) => adminService.updateUser(id, { is_replacement: nextVal }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey })
      qc.invalidateQueries({ queryKey: ['tenant-detail'] })
      qc.invalidateQueries({ queryKey: ['tenant-employees'] })
      qc.invalidateQueries({ queryKey: ['admin-tenants'] })
    },
  })

  return {
    tracking: data ?? null,
    isLoading,
    error: error ? 'No se pudo cargar el empleado' : null,
    refresh: async () => { await refetch() },
    toggleStatus: async () => {
      if (!data?.user) return
      await toggleMut.mutateAsync(!data.user.is_active)
    },
    toggleReplacement: async () => {
      if (!data?.user) return
      await toggleReplacementMut.mutateAsync(!data.user.is_replacement)
    },
    resetPassword: async (newPassword: string) => {
      await adminService.resetPassword(id, newPassword)
    },
  }
}
