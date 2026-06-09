import { useState, useCallback } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
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
  type: string
  description: string
  details?: string
  user: string
  created_at: string
  timestamp?: string
}

interface AbsenceReportItem {
  id: number
  user_id: number
  user: string
  company: string
  work_date: string
  hours_worked: number
  absence_hours: number
  absence_reason: string
  approved: boolean
  rejected: boolean
  created_at: string
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
  recentActivity: ActivityItem[]
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

function normalizeRecentActivity(items: any[] = []): ActivityItem[] {
  return items.map((item, index) => ({
    ...item,
    id: item.id ?? index,
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
      const response = await adminService.getUsers()
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

  const activityQ = useQuery({
    queryKey: ['admin', 'recent-activity'],
    queryFn: async () => normalizeRecentActivity(await adminService.getRecentActivity()),
  })

  const absenceQ = useQuery({
    queryKey: ['admin', 'absence-report'],
    queryFn: async () => (await adminService.getAbsenceReport()) || null,
  })

  const invalidateUsers = () => qc.invalidateQueries({ queryKey: ['admin', 'users'] })

  const createMut = useMutation({ mutationFn: (data: any) => adminService.createUser(data), onSuccess: invalidateUsers })
  const updateMut = useMutation({ mutationFn: ({ id, data }: { id: number; data: any }) => adminService.updateUser(id, data), onSuccess: invalidateUsers })
  const deleteMut = useMutation({ mutationFn: (id: number) => adminService.deleteUser(id), onSuccess: invalidateUsers })

  const users: User[] = usersQ.data ?? []

  return {
    stats: statsQ.data ?? null,
    users,
    companies: companiesQ.data ?? [],
    inactiveUsers: inactiveQ.data ?? [],
    recentActivity: activityQ.data ?? [],
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
