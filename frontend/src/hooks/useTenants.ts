import { useState, useCallback, useEffect } from 'react'
import { adminService } from '../services/api'
import type { Tenant } from '../types'

interface CreateTenantInput {
  company_name: string
  user_id?: number
  name?: string
  email?: string
  password?: string
}

interface UseTenantsReturn {
  tenants: Tenant[]
  isLoading: boolean
  error: string | null
  refresh: () => Promise<void>
  createTenant: (data: CreateTenantInput) => Promise<void>
  suspendTenant: (id: number) => Promise<void>
  activateTenant: (id: number) => Promise<void>
}

export function useTenants(): UseTenantsReturn {
  const [tenants, setTenants] = useState<Tenant[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    try {
      const data = await adminService.getTenants()
      setTenants(Array.isArray(data) ? data : [])
      setError(null)
    } catch {
      setError('No se pudieron cargar las empresas')
    }
  }, [])

  const createTenant = useCallback(async (data: CreateTenantInput) => {
    await adminService.createTenant(data)
    await refresh()
  }, [refresh])

  const suspendTenant = useCallback(async (id: number) => {
    await adminService.suspendTenant(id)
    await refresh()
  }, [refresh])

  const activateTenant = useCallback(async (id: number) => {
    await adminService.activateTenant(id)
    await refresh()
  }, [refresh])

  useEffect(() => {
    const load = async () => {
      setIsLoading(true)
      await refresh()
      setIsLoading(false)
    }
    load()
  }, [refresh])

  return { tenants, isLoading, error, refresh, createTenant, suspendTenant, activateTenant }
}
