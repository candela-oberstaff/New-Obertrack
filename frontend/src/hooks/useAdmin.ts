import { useState, useCallback } from 'react'
import { useQuery, useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { adminService } from '../services/api'
import type { User } from '../types'

interface DashboardStats {
  totalUsers: number
  activeUsers: number
  totalTasks: number
  totalBoards: number
  totalCompanies: number
  totalProfessionals: number
  activeToday: number
  inactiveWarning: number
  pendingHours: number
}

interface ActivityItem {
  id: number
  /** Identidad estable para listar: el id de origen se repite entre tipos
   *  (la tarea 5 y el tablero 5 son filas distintas del mismo feed). */
  uid: string
  type: string
  description: string
  details?: string
  user: string
  created_at: string
  timestamp?: string
}

export interface AbsenceReportItem {
  id: number
  user_id: number
  user: string
  email: string
  phone_number?: string
  avatar?: string
  tenant_id: number
  company: string
  work_date: string
  hours_worked: number
  absence_hours: number
  absence_reason: string
  approved: boolean
  rejected: boolean
  created_at: string
}

export interface FollowUpInfo {
  user_id: number
  status: 'contacted' | 'justified' | 'escalated'
  note: string
  by_name: string
  created_at: string
}

export interface SeniorityItem {
  user_id: number
  name: string
  email: string
  avatar?: string
  job_title?: string
  company: string
  tenant_id: number
  started_at: string
  days_employed: number
}

export interface TeamInactivityItem {
  id: number
  name: string
  email: string
  avatar?: string
  job_title?: string
  phone_number?: string
  company: string
  tenant_id: number
  last_active: string
  days_inactive: number
}

interface AbsenceReasonCount {
  reason: string
  count: number
}

interface AbsenceReport {
  total_absences: number
  absence_hours: number
  pending_review: number
  approved: number
  rejected: number
  reasons: AbsenceReasonCount[]
  items: AbsenceReportItem[]
}

interface UseAdminReturn {
  stats: DashboardStats | null
  users: User[]
  companies: any[]
  inactiveUsers: User[]
  /** Actividad de equipo: profesionales con 1+ días sin registrar horas. */
  teamInactivity: TeamInactivityItem[]
  teamInactivityLoading: boolean
  /** Métricas CS: ranking de antigüedad y resumen por empresa. */
  seniority: SeniorityItem[]
  tenants: any[]
  /** Bitácora de gestión CS: estado vigente por profesional (user_id → info). */
  followUps: { inactivity: Record<number, FollowUpInfo>; absence: Record<number, FollowUpInfo> }
  setFollowUp: (userId: number, kind: 'inactivity' | 'absence', status: string) => Promise<void>
  recentActivity: ActivityItem[]
  /** Feed navegado de página en página (flechas), no acumulado. */
  activityPage: ActivityItem[]
  activityPageNumber: number
  hasPrevActivity: boolean
  /** Quedan eventos más antiguos, ya en caché o por pedir al servidor. */
  hasMoreActivity: boolean
  loadingMoreActivity: boolean
  prevActivityPage: () => void
  nextActivityPage: () => void
  absenceReport: AbsenceReport | null
  isLoading: boolean

  activeTab: string
  setActiveTab: (tab: string) => void

  fetchDashboard: () => Promise<void>
  fetchUsers: () => Promise<void>
  fetchCompanies: () => Promise<void>
  fetchInactiveUsers: () => Promise<void>
  fetchRecentActivity: () => Promise<void>
  fetchAbsenceReport: () => Promise<void>

  createUser: (data: any) => Promise<void>
  updateUser: (id: number, data: any) => Promise<void>
  deleteUser: (id: number) => Promise<void>
  toggleUserStatus: (id: number) => Promise<void>
  resetUserPassword: (id: number, newPassword: string) => Promise<void>
  promoteToManager: (id: number) => Promise<void>
}

/** Eventos por tanda del feed de actividad (el backend acepta hasta 100). */
const ACTIVITY_PAGE_SIZE = 25

/** Posición desde la que seguir leyendo el feed hacia atrás. */
interface ActivityCursor {
  before: string
  before_type: string
  before_id: number
}

function normalizeRecentActivity(items: any[] = []): ActivityItem[] {
  return items.map((item, index) => ({
    ...item,
    id: item.id ?? index,
    uid: item.id != null ? `${item.type ?? 'evento'}-${item.id}` : `${item.timestamp ?? 'sin-fecha'}-${index}`,
    description: item.description || item.details || 'Sin descripcion',
    created_at: item.created_at || item.timestamp || '',
  }))
}

export function useAdmin(): UseAdminReturn {
  const qc = useQueryClient()
  const [activeTab, setActiveTab] = useState('dashboard')

  const statsQ = useQuery({
    queryKey: ['admin', 'dashboard'],
    queryFn: async (): Promise<DashboardStats | null> => {
      const data = await adminService.getDashboard()
      if (!data) return null
      return {
        totalUsers: data.total_users || 0,
        activeUsers: data.active_users || 0,
        totalTasks: data.total_tasks || 0,
        totalBoards: data.total_boards || 0,
        totalCompanies: data.total_companies || 0,
        totalProfessionals: data.total_professionals || 0,
        activeToday: data.active_today || 0,
        inactiveWarning: data.inactive_warning || 0,
        pendingHours: data.pending_hours || 0,
      }
    },
  })

  const usersQ = useQuery({
    queryKey: ['admin', 'users'],
    queryFn: async () => {
      const response = await adminService.getUsers({ limit: 1000 })
      return response?.data || (Array.isArray(response) ? response : [])
    },
  })

  const companiesQ = useQuery({
    queryKey: ['admin', 'companies'],
    queryFn: async () => (await adminService.getCompanies()) || [],
  })

  const inactiveQ = useQuery({
    queryKey: ['admin', 'inactive-users'],
    queryFn: async () => (await adminService.getInactiveUsers()) || [],
  })

  // Actividad de equipo: umbral de 1 día (amarillo 1, rojo 2+).
  const teamInactivityQ = useQuery({
    queryKey: ['admin', 'team-inactivity'],
    queryFn: async (): Promise<TeamInactivityItem[]> => (await adminService.getInactiveUsers(1)) || [],
  })

  // Métricas de customer success (dashboard).
  const seniorityQ = useQuery({
    queryKey: ['admin', 'seniority'],
    queryFn: async (): Promise<SeniorityItem[]> => (await adminService.getSeniority()) || [],
  })

  const tenantsQ = useQuery({
    queryKey: ['admin', 'tenants-summary'],
    queryFn: async () => (await adminService.getTenants()) || [],
  })

  // Bitácora de gestión CS (estado vigente por profesional).
  const toFollowUpMap = (items: FollowUpInfo[]) =>
    Object.fromEntries(items.map(i => [i.user_id, i])) as Record<number, FollowUpInfo>

  const followUpsInactivityQ = useQuery({
    queryKey: ['admin', 'follow-ups', 'inactivity'],
    queryFn: async () => toFollowUpMap(await adminService.getFollowUps('inactivity')),
  })
  const followUpsAbsenceQ = useQuery({
    queryKey: ['admin', 'follow-ups', 'absence'],
    queryFn: async () => toFollowUpMap(await adminService.getFollowUps('absence')),
  })

  const followUpMut = useMutation({
    mutationFn: (payload: { user_id: number; kind: 'inactivity' | 'absence'; status: string }) =>
      adminService.createFollowUp(payload),
    onSuccess: (_data, payload) => {
      qc.invalidateQueries({ queryKey: ['admin', 'follow-ups', payload.kind] })
    },
  })

  // Feed global por tandas. El cursor viaja con la fecha tal cual la devolvió el
  // backend (sin pasar por Date, que redondea y se saltaría eventos del mismo
  // milisegundo) más el tipo e id del último evento entregado.
  const activityQ = useInfiniteQuery({
    queryKey: ['admin', 'recent-activity'],
    initialPageParam: null as ActivityCursor | null,
    queryFn: async ({ pageParam }) =>
      normalizeRecentActivity(
        await adminService.getRecentActivity({ limit: ACTIVITY_PAGE_SIZE, ...(pageParam ?? {}) }),
      ),
    getNextPageParam: (lastPage): ActivityCursor | undefined => {
      // Una página incompleta significa que ya no queda nada más atrás.
      if (lastPage.length < ACTIVITY_PAGE_SIZE) return undefined
      const last = lastPage[lastPage.length - 1]
      const before = last.timestamp || last.created_at
      if (!before) return undefined
      return { before, before_type: last.type, before_id: last.id }
    },
  })

  // Navegación del feed: las tandas ya traídas se quedan en caché, así que
  // volver atrás es instantáneo y solo se pide al servidor al pisar terreno
  // nuevo. No hay "de N páginas" porque el cursor no sabe cuántas quedan:
  // contarlas obligaría a recorrer el histórico entero en cada carga.
  const [activityPageIndex, setActivityPageIndex] = useState(0)
  const activityPages = activityQ.data?.pages ?? []

  const nextActivityPage = useCallback(async () => {
    if (activityPageIndex < activityPages.length - 1) {
      setActivityPageIndex(i => i + 1)
      return
    }
    const res = await activityQ.fetchNextPage()
    const pages = res.data?.pages ?? []
    // Solo se avanza si la tanda nueva trae algo: si el feed se acabó justo en
    // un múltiplo del tamaño de página, la última llamada vuelve vacía.
    if (pages.length > activityPages.length && pages[pages.length - 1].length > 0) {
      setActivityPageIndex(pages.length - 1)
    }
  }, [activityPageIndex, activityPages.length, activityQ.fetchNextPage])

  const absenceQ = useQuery({
    queryKey: ['admin', 'absence-report'],
    queryFn: async () => (await adminService.getAbsenceReport()) || null,
  })

  const invalidateUsers = () => qc.invalidateQueries({ queryKey: ['admin', 'users'] })
  const invalidateAfterUserChange = () => {
    qc.invalidateQueries({ queryKey: ['admin', 'users'] })
    qc.invalidateQueries({ queryKey: ['admin', 'dashboard'] })
    qc.invalidateQueries({ queryKey: ['admin', 'companies'] })
    qc.invalidateQueries({ queryKey: ['admin', 'tenants-summary'] })
  }

  const createMut = useMutation({ mutationFn: (data: any) => adminService.createUser(data), onSuccess: invalidateAfterUserChange })
  const updateMut = useMutation({ mutationFn: ({ id, data }: { id: number; data: any }) => adminService.updateUser(id, data), onSuccess: invalidateAfterUserChange })
  const deleteMut = useMutation({ mutationFn: (id: number) => adminService.deleteUser(id), onSuccess: invalidateAfterUserChange })

  const users: User[] = usersQ.data ?? []

  return {
    stats: statsQ.data ?? null,
    users,
    companies: companiesQ.data ?? [],
    inactiveUsers: inactiveQ.data ?? [],
    teamInactivity: teamInactivityQ.data ?? [],
    teamInactivityLoading: teamInactivityQ.isLoading,
    seniority: seniorityQ.data ?? [],
    tenants: tenantsQ.data ?? [],
    followUps: {
      inactivity: followUpsInactivityQ.data ?? {},
      absence: followUpsAbsenceQ.data ?? {},
    },
    setFollowUp: async (userId, kind, status) => {
      await followUpMut.mutateAsync({ user_id: userId, kind, status })
    },
    recentActivity: activityPages.flat(),
    activityPage: activityPages[activityPageIndex] ?? [],
    activityPageNumber: activityPageIndex + 1,
    hasPrevActivity: activityPageIndex > 0,
    hasMoreActivity: activityPageIndex < activityPages.length - 1 || activityQ.hasNextPage,
    loadingMoreActivity: activityQ.isFetchingNextPage,
    prevActivityPage: useCallback(() => { setActivityPageIndex(i => Math.max(0, i - 1)) }, []),
    nextActivityPage,
    absenceReport: absenceQ.data ?? null,
    // Page-level skeleton waits for the main data sets, mirroring the old behaviour.
    isLoading: statsQ.isLoading || usersQ.isLoading || companiesQ.isLoading || inactiveQ.isLoading || absenceQ.isLoading,

    activeTab,
    setActiveTab,

    fetchDashboard: useCallback(async () => { await qc.invalidateQueries({ queryKey: ['admin', 'dashboard'] }) }, [qc]),
    fetchUsers: useCallback(async () => { await invalidateUsers() }, [qc]),
    fetchCompanies: useCallback(async () => { await qc.invalidateQueries({ queryKey: ['admin', 'companies'] }) }, [qc]),
    fetchInactiveUsers: useCallback(async () => { await qc.invalidateQueries({ queryKey: ['admin', 'inactive-users'] }) }, [qc]),
    fetchRecentActivity: useCallback(async () => { await qc.invalidateQueries({ queryKey: ['admin', 'recent-activity'] }) }, [qc]),
    fetchAbsenceReport: useCallback(async () => { await qc.invalidateQueries({ queryKey: ['admin', 'absence-report'] }) }, [qc]),

    createUser: async (data) => { await createMut.mutateAsync(data) },
    updateUser: async (id, data) => { await updateMut.mutateAsync({ id, data }) },
    deleteUser: async (id) => { await deleteMut.mutateAsync(id) },
    toggleUserStatus: async (id) => {
      const user = users.find(u => u.id === id)
      if (user) await updateMut.mutateAsync({ id, data: { is_active: !user.is_active } })
    },
    resetUserPassword: async (id, newPassword) => { await adminService.resetPassword(id, newPassword) },
    promoteToManager: async (id) => { await updateMut.mutateAsync({ id, data: { is_manager: true } }) },
  }
}
