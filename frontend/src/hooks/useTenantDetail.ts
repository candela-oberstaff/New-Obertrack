import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { adminService } from '../services/api'
import type { Tenant, EmployeeSummary } from '../types'

interface UseTenantDetailReturn {
  tenant: Tenant | null
  employees: EmployeeSummary[]
  isLoading: boolean
  error: string | null
  refresh: () => Promise<void>
  suspendTenant: () => Promise<void>
  activateTenant: () => Promise<void>
  toggleEmployeeStatus: (employee: EmployeeSummary) => Promise<void>
  resetEmployeePassword: (id: number, newPassword: string) => Promise<void>
}

export function useTenantDetail(id: number): UseTenantDetailReturn {
  const qc = useQueryClient()
  const queryKey = ['tenant-detail', id]

  const { data, isLoading, error, refetch } = useQuery({
    queryKey,
    // El expediente NO viaja aquí: se pagina y se filtra aparte (useTenantActivity),
    // así cambiar de página no vuelve a pedir la empresa ni su plantilla.
    queryFn: async () => {
      const [tenantData, employeesData] = await Promise.allSettled([
        adminService.getTenant(id),
        adminService.getTenantEmployees(id),
      ])
      if (tenantData.status !== 'fulfilled') {
        throw new Error('No se pudo cargar la empresa')
      }
      return {
        tenant: (tenantData.value as Tenant) ?? null,
        employees: employeesData.status === 'fulfilled' && Array.isArray(employeesData.value) ? employeesData.value : [],
      }
    },
    enabled: !!id,
  })

  const invalidate = () => qc.invalidateQueries({ queryKey })

  const setActiveMut = useMutation({
    mutationFn: (active: boolean) => (active ? adminService.activateTenant(id) : adminService.suspendTenant(id)),
    onSuccess: invalidate,
  })

  const toggleEmployeeMut = useMutation({
    mutationFn: (employee: EmployeeSummary) => adminService.updateUser(employee.id, { is_active: !employee.is_active }),
    onSuccess: invalidate,
  })

  return {
    tenant: data?.tenant ?? null,
    employees: data?.employees ?? [],
    isLoading,
    error: error ? 'No se pudo cargar la empresa' : null,
    refresh: async () => { await refetch() },
    suspendTenant: async () => { await setActiveMut.mutateAsync(false) },
    activateTenant: async () => { await setActiveMut.mutateAsync(true) },
    toggleEmployeeStatus: async (employee) => { await toggleEmployeeMut.mutateAsync(employee) },
    resetEmployeePassword: async (employeeId, newPassword) => { await adminService.resetPassword(employeeId, newPassword) },
  }
}
