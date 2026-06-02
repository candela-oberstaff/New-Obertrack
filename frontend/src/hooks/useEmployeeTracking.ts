import { useState, useCallback, useEffect } from 'react'
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
  const [tracking, setTracking] = useState<EmployeeTracking | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    try {
      const data = await adminService.getEmployeeTracking(id)
      setTracking({
        ...data,
        work_hours: Array.isArray(data?.work_hours) ? data.work_hours : [],
        tasks: Array.isArray(data?.tasks) ? data.tasks : [],
      })
      setError(null)
    } catch {
      setError('No se pudo cargar el empleado')
    }
  }, [id])

  const toggleStatus = useCallback(async () => {
    if (!tracking) return
    await adminService.updateUser(id, { is_active: !tracking.user.is_active })
    await refresh()
  }, [id, tracking, refresh])

  const resetPassword = useCallback(async (newPassword: string) => {
    await adminService.resetPassword(id, newPassword)
  }, [id])

  useEffect(() => {
    const load = async () => {
      setIsLoading(true)
      await refresh()
      setIsLoading(false)
    }
    load()
  }, [refresh])

  return { tracking, isLoading, error, refresh, toggleStatus, resetPassword }
}
