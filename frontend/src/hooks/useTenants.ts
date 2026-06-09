import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
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

const TENANTS_KEY = ['tenants'] as const

/**
 * Tenants list backed by React Query. Benefits over the previous manual
 * useState/useEffect version:
 *  - Caching + dedup: revisiting the page within staleTime renders instantly
 *    while a background refetch keeps it fresh (no spinner flash).
 *  - Mutations auto-invalidate the list — no hand-rolled refresh() chains.
 *  - Suspend/activate update optimistically (badge flips instantly) and roll
 *    back automatically if the request fails.
 */
export function useTenants(): UseTenantsReturn {
  const qc = useQueryClient()

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: TENANTS_KEY,
    queryFn: async () => {
      const res = await adminService.getTenants()
      return Array.isArray(res) ? (res as Tenant[]) : []
    },
  })

  const createMut = useMutation({
    mutationFn: (input: CreateTenantInput) => adminService.createTenant(input),
    onSuccess: () => qc.invalidateQueries({ queryKey: TENANTS_KEY }),
  })

  const setActiveMut = useMutation({
    mutationFn: ({ id, active }: { id: number; active: boolean }) =>
      active ? adminService.activateTenant(id) : adminService.suspendTenant(id),
    // Optimistic update: flip the badge immediately, remember the snapshot.
    onMutate: async ({ id, active }) => {
      await qc.cancelQueries({ queryKey: TENANTS_KEY })
      const prev = qc.getQueryData<Tenant[]>(TENANTS_KEY)
      qc.setQueryData<Tenant[]>(TENANTS_KEY, (old) =>
        (old ?? []).map((t) => (t.id === id ? { ...t, is_active: active } : t)),
      )
      return { prev }
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.prev) qc.setQueryData(TENANTS_KEY, ctx.prev)
    },
    onSettled: () => qc.invalidateQueries({ queryKey: TENANTS_KEY }),
  })

  return {
    tenants: data ?? [],
    isLoading,
    error: error ? 'No se pudieron cargar las empresas' : null,
    refresh: async () => {
      await refetch()
    },
    createTenant: async (input) => {
      await createMut.mutateAsync(input)
    },
    suspendTenant: async (id) => {
      await setActiveMut.mutateAsync({ id, active: false })
    },
    activateTenant: async (id) => {
      await setActiveMut.mutateAsync({ id, active: true })
    },
  }
}
