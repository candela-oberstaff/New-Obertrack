import { useQuery, useMutation, useQueryClient, keepPreviousData } from '@tanstack/react-query'
import { adminService } from '../services/api'

/** Categorías del expediente, en el mismo orden en que se ofrecen como filtro. */
export const ACTIVITY_CATEGORIES = [
  { value: '', label: 'Todo' },
  { value: 'lifecycle', label: 'Ciclo de vida' },
  { value: 'staff', label: 'Plantilla' },
  { value: 'work', label: 'Jornadas' },
  { value: 'management', label: 'Gestión CS' },
  { value: 'note', label: 'Notas' },
] as const

export interface TenantActivity {
  type: string
  category: string
  user: string
  user_id: number
  company: string
  details: string
  timestamp: string
  /** Solo las entradas de company_events lo traen; 0 en las derivadas. */
  event_id: number
  /** Solo en notas: fijada en la cabecera y fecha de la última corrección. */
  pinned?: boolean
  edited_at?: string
}

export interface TenantActivityPerson {
  user_id: number
  name: string
}

interface UseTenantActivityReturn {
  activity: TenantActivity[]
  total: number
  /** Movimientos por categoría ('' = total), con el filtro de persona aplicado. */
  counts: Record<string, number>
  /** Personas que aparecen en el expediente, para el desplegable de filtro. */
  people: TenantActivityPerson[]
  /** Notas fijadas, que van arriba y fuera de la cronología. */
  pinnedNotes: TenantActivity[]
  isLoading: boolean
  /** Hay una página nueva en camino (se sigue mostrando la anterior). */
  isFetching: boolean
  error: string | null
  addNote: (detail: string) => Promise<void>
  updateNote: (eventId: number, detail: string) => Promise<void>
  setNotePinned: (eventId: number, pinned: boolean) => Promise<void>
  deleteNote: (eventId: number) => Promise<void>
}

/**
 * Una página del expediente de la empresa. La paginación y los filtros son de
 * servidor: el expediente de una empresa con recorrido son miles de líneas y
 * traerlas todas para enseñar veinte no tiene sentido.
 */
export function useTenantActivity(
  tenantId: number,
  category: string,
  userId: number,
  page: number,
  pageSize = 20,
): UseTenantActivityReturn {
  const qc = useQueryClient()
  const queryKey = ['tenant-activity', tenantId, category, userId, page, pageSize]

  const { data, isLoading, isFetching, error } = useQuery({
    queryKey,
    queryFn: () => adminService.getTenantActivity(tenantId, { category, user_id: userId, page, limit: pageSize }),
    enabled: !!tenantId,
    // Al cambiar de página o de filtro se mantiene lo anterior en pantalla
    // hasta que llega lo nuevo, en vez de parpadear a vacío.
    placeholderData: keepPreviousData,
  })

  // La lista de personas no depende de la página ni del filtro: se pide una vez
  // por empresa y se reutiliza mientras se navega el expediente.
  const peopleQ = useQuery({
    queryKey: ['tenant-activity-people', tenantId],
    queryFn: () => adminService.getTenantActivityPeople(tenantId),
    enabled: !!tenantId,
  })

  // Las fijadas son pocas y siempre visibles: van aparte de la paginación para
  // que no dependan de en qué página estés ni de qué filtro tengas puesto.
  const pinnedQ = useQuery({
    queryKey: ['tenant-activity-pinned', tenantId],
    queryFn: () => adminService.getTenantPinnedNotes(tenantId),
    enabled: !!tenantId,
  })

  // Tras tocar una nota queda obsoleto todo el expediente de esta empresa: sus
  // páginas, sus contadores, las fijadas, y la lista de personas (quien firma
  // su primera nota pasa a aparecer en el filtro).
  const invalidate = () =>
    qc.invalidateQueries({
      predicate: q =>
        typeof q.queryKey[0] === 'string' &&
        (q.queryKey[0] as string).startsWith('tenant-activity') &&
        q.queryKey[1] === tenantId,
    })

  const addMut = useMutation({
    mutationFn: (detail: string) => adminService.addTenantNote(tenantId, detail),
    onSuccess: invalidate,
  })

  const updateMut = useMutation({
    mutationFn: ({ eventId, detail }: { eventId: number; detail: string }) =>
      adminService.updateTenantNote(tenantId, eventId, detail),
    onSuccess: invalidate,
  })

  const pinMut = useMutation({
    mutationFn: ({ eventId, pinned }: { eventId: number; pinned: boolean }) =>
      adminService.setTenantNotePinned(tenantId, eventId, pinned),
    onSuccess: invalidate,
  })

  const deleteMut = useMutation({
    mutationFn: (eventId: number) => adminService.deleteTenantNote(tenantId, eventId),
    onSuccess: invalidate,
  })

  return {
    activity: (data?.data ?? []) as TenantActivity[],
    total: data?.total ?? 0,
    counts: (data?.counts ?? {}) as Record<string, number>,
    people: peopleQ.data ?? [],
    pinnedNotes: (pinnedQ.data ?? []) as TenantActivity[],
    isLoading,
    isFetching,
    error: error ? 'No se pudo cargar el expediente' : null,
    addNote: async (detail) => { await addMut.mutateAsync(detail) },
    updateNote: async (eventId, detail) => { await updateMut.mutateAsync({ eventId, detail }) },
    setNotePinned: async (eventId, pinned) => { await pinMut.mutateAsync({ eventId, pinned }) },
    deleteNote: async (eventId) => { await deleteMut.mutateAsync(eventId) },
  }
}
