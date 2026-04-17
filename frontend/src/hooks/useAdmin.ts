import { useState, useCallback, useEffect } from 'react'
import { adminService, boardService } from '../services/api'
import type { User } from '../types'

interface DashboardStats {
  totalUsers: number
  activeUsers: number
  totalTasks: number
  totalBoards: number
}

interface ActivityItem {
  id: number
  type: string
  description: string
  user: string
  created_at: string
}

interface UseAdminReturn {
  // Data
  stats: DashboardStats | null
  users: User[]
  companies: any[]
  inactiveUsers: User[]
  recentActivity: ActivityItem[]
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
  const [isLoading, setIsLoading] = useState(true)
  const [activeTab, setActiveTab] = useState('dashboard')

  const fetchDashboard = useCallback(async () => {
    try {
      const [statsData, activityData, boardsData] = await Promise.allSettled([
        adminService.getDashboard(),
        adminService.getRecentActivity(),
        boardService.getAll()
      ])

      if (statsData.status === 'fulfilled' && statsData.value) {
        const data = statsData.value;
        setStats({
          totalUsers: (data.total_companies || 0) + (data.total_professionals || 0) + (data.total_managers || 0),
          activeUsers: data.active_today || 0,
          totalTasks: data.total_tasks || 0,
          totalBoards: boardsData.status === 'fulfilled' ? (boardsData.value?.length || 0) : 0
        })
      }
      if (activityData.status === 'fulfilled') {
        setRecentActivity(activityData.value || [])
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
      setRecentActivity(data || [])
    } catch (error) {
      console.error('Error fetching recent activity:', error)
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
      ])
      setIsLoading(false)
    }
    loadInitialData()
  }, [fetchDashboard, fetchUsers, fetchCompanies, fetchInactiveUsers])

  return {
    stats,
    users,
    companies,
    inactiveUsers,
    recentActivity,
    isLoading,
    activeTab,
    setActiveTab,
    fetchDashboard,
    fetchUsers,
    fetchCompanies,
    fetchInactiveUsers,
    fetchRecentActivity,
    createUser,
    updateUser,
    deleteUser,
    toggleUserStatus,
    resetUserPassword,
    promoteToManager,
  }
}
