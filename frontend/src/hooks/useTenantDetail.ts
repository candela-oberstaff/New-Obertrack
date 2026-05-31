import { useState, useCallback, useEffect } from 'react'
import { adminService } from '../services/api'
import type { Tenant, EmployeeSummary } from '../types'

interface TenantActivity {
  type: string
  user: string
  company: string
  details: string
  timestamp: string
}

interface UseTenantDetailReturn {
  tenant: Tenant | null
  employees: EmployeeSummary[]
  activity: TenantActivity[]
  isLoading: boolean
  error: string | null
  refresh: () => Promise<void>
  suspendTenant: () => Promise<void>
  activateTenant: () => Promise<void>
  toggleEmployeeStatus: (employee: EmployeeSummary) => Promise<void>
  resetEmployeePassword: (id: number, newPassword: string) => Promise<void>
}

export function useTenantDetail(id: number): UseTenantDetailReturn {
  const [tenant, setTenant] = useState<Tenant | null>(null)
  const [employees, setEmployees] = useState<EmployeeSummary[]>([])
  const [activity, setActivity] = useState<TenantActivity[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    try {
      const [tenantData, employeesData, activityData] = await Promise.allSettled([
        adminService.getTenant(id),
        adminService.getTenantEmployees(id),
        adminService.getTenantActivity(id),
      ])

      if (tenantData.status === 'fulfilled') {
        setTenant(tenantData.value)
        setError(null)
      } else {
        setError('No se pudo cargar la empresa')
      }
      setEmployees(employeesData.status === 'fulfilled' && Array.isArray(employeesData.value) ? employeesData.value : [])
      setActivity(activityData.status === 'fulfilled' && Array.isArray(activityData.value) ? activityData.value : [])
    } catch {
      setError('No se pudo cargar la empresa')
    }
  }, [id])

  const suspendTenant = useCallback(async () => {
    await adminService.suspendTenant(id)
    await refresh()
  }, [id, refresh])

  const activateTenant = useCallback(async () => {
    await adminService.activateTenant(id)
    await refresh()
  }, [id, refresh])

  const toggleEmployeeStatus = useCallback(async (employee: EmployeeSummary) => {
    await adminService.updateUser(employee.id, { is_active: !employee.is_active })
    await refresh()
  }, [refresh])

  const resetEmployeePassword = useCallback(async (employeeId: number, newPassword: string) => {
    await adminService.resetPassword(employeeId, newPassword)
  }, [])

  useEffect(() => {
    const load = async () => {
      setIsLoading(true)
      await refresh()
      setIsLoading(false)
    }
    load()
  }, [refresh])

  return { tenant, employees, activity, isLoading, error, refresh, suspendTenant, activateTenant, toggleEmployeeStatus, resetEmployeePassword }
}
