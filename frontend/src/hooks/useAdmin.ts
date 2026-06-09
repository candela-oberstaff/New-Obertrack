import { useState, useCallback, useEffect } from 'react'
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
  // Data
  stats: DashboardStats | null
  users: User[]
  companies: any[]
  inactiveUsers: User[]
  recentActivity: ActivityItem[]
  absenceReport: AbsenceReport | null
  isLoading: boolean

  // Tab state
  activeTab: string
  setActiveTab: (tab: string) => void

  // Actions
  fetchDashboard: () => Promise<void>
  fetchUsers: () => Promise<void>
  fetchCompanies: () => Promise<void>
  fetchInactiveUsers: () => Promise<void>
  fetchRecentActivity: () => Promise<void>
  fetchAbsenceReport: () => Promise<void>

  // User CRUD
  createUser: (data: any) => Promise<void>
  updateUser: (id: number, data: any) => Promise<void>
  deleteUser: (id: number) => Promise<void>
  toggleUserStatus: (id: number) => Promise<void>
  resetUserPassword: (id: number, newPassword: string) => Promise<void>
  promoteToManager: (id: number) => Promise<void>
}

export function useAdmin(): UseAdminReturn {
  const [stats, setStats] = useState<DashboardStats | null>(null)
  const [users, setUsers] = useState<User[]>([])
  const [companies, setCompanies] = useState<any[]>([])
  const [inactiveUsers, setInactiveUsers] = useState<User[]>([])
  const [recentActivity, setRecentActivity] = useState<ActivityItem[]>([])
  const [absenceReport, setAbsenceReport] = useState<AbsenceReport | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [activeTab, setActiveTab] = useState('dashboard')

  const normalizeRecentActivity = (items: any[] = []): ActivityItem[] => {
    return items.map((item, index) => ({
      ...item,
      id: item.id ?? index,
      description: item.description || item.details || 'Sin descripcion',
      created_at: item.created_at || item.timestamp || '',
    }))
  }

  const fetchDashboard = useCallback(async () => {
    try {
      const [statsData, activityData] = await Promise.allSettled([
        adminService.getDashboard(),
        adminService.getRecentActivity(),
      ])

      if (statsData.status === 'fulfilled' && statsData.value) {
        const data = statsData.value;
        setStats({
          totalUsers: data.total_users || 0,
          activeUsers: data.active_users || 0,
          totalTasks: data.total_tasks || 0,
          totalBoards: data.total_boards || 0,
          totalCompanies: data.total_companies || 0,
          totalProfessionals: data.total_professionals || 0,
          activeToday: data.active_today || 0,
          inactiveWarning: data.inactive_warning || 0,
          pendingHours: data.pending_hours || 0,
        })
      }
      if (activityData.status === 'fulfilled') {
        setRecentActivity(normalizeRecentActivity(activityData.value))
      }
    } catch (error) {
      console.error('Error fetching dashboard:', error)
    }
  }, [])

  const fetchUsers = useCallback(async () => {
    try {
      const response = await adminService.getUsers()
      // The backend returns { data: [...], total: ... } for paginated results
      const usersArray = response?.data || (Array.isArray(response) ? response : [])
      setUsers(usersArray)
    } catch (error) {
      console.error('Error fetching users:', error)
    }
  }, [])

  const fetchCompanies = useCallback(async () => {
    try {
      const data = await adminService.getCompanies()
      setCompanies(data || [])
    } catch (error) {
      console.error('Error fetching companies:', error)
    }
  }, [])

  const fetchInactiveUsers = useCallback(async () => {
    try {
      const data = await adminService.getInactiveUsers()
      setInactiveUsers(data || [])
    } catch (error) {
      console.error('Error fetching inactive users:', error)
    }
  }, [])

  const fetchRecentActivity = useCallback(async () => {
    try {
      const data = await adminService.getRecentActivity()
      setRecentActivity(normalizeRecentActivity(data))
    } catch (error) {
      console.error('Error fetching recent activity:', error)
    }
  }, [])

  const fetchAbsenceReport = useCallback(async () => {
    try {
      const data = await adminService.getAbsenceReport()
      setAbsenceReport(data || null)
    } catch (error) {
      console.error('Error fetching absence report:', error)
    }
  }, [])

  const createUser = useCallback(async (data: any) => {
    await adminService.createUser(data)
    fetchUsers()
  }, [fetchUsers])

  const updateUser = useCallback(async (id: number, data: any) => {
    await adminService.updateUser(id, data)
    fetchUsers()
  }, [fetchUsers])

  const deleteUser = useCallback(async (id: number) => {
    await adminService.deleteUser(id)
    fetchUsers()
  }, [fetchUsers])

  const toggleUserStatus = useCallback(async (id: number) => {
    // Using updateUser to toggle status
    const user = users.find(u => u.id === id)
    if (user) {
      await adminService.updateUser(id, { is_active: !user.is_active })
      fetchUsers()
    }
  }, [users, fetchUsers])

  const resetUserPassword = useCallback(async (id: number, newPassword: string) => {
    await adminService.resetPassword(id, newPassword)
  }, [])

  const promoteToManager = useCallback(async (id: number) => {
    await adminService.updateUser(id, { is_manager: true })
    fetchUsers()
  }, [fetchUsers])

  useEffect(() => {
    const loadInitialData = async () => {
      setIsLoading(true)
      await Promise.all([
        fetchDashboard(),
        fetchUsers(),
        fetchCompanies(),
        fetchInactiveUsers(),
        fetchAbsenceReport(),
      ])
      setIsLoading(false)
    }
    loadInitialData()
  }, [fetchDashboard, fetchUsers, fetchCompanies, fetchInactiveUsers, fetchAbsenceReport])

  return {
    stats,
    users,
    companies,
    inactiveUsers,
    recentActivity,
    absenceReport,
    isLoading,
    activeTab,
    setActiveTab,
    fetchDashboard,
    fetchUsers,
    fetchCompanies,
    fetchInactiveUsers,
    fetchRecentActivity,
    fetchAbsenceReport,
    createUser,
    updateUser,
    deleteUser,
    toggleUserStatus,
    resetUserPassword,
    promoteToManager,
  }
}
