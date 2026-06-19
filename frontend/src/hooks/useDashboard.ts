import { useCallback, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { taskService, workHourService, userService } from '../services/api'
import type { Task, WorkHour, User } from '../types'

interface DashboardSummary {
  total_hours: number
  approved_hours: number
  pending_hours: number
}

interface WeekDayData {
  day: string
  hours: number
  target: number
}

interface UseDashboardReturn {
  // Data
  tasks: Task[]
  workHours: WorkHour[]
  employees: User[]
  summary: DashboardSummary
  isLoading: boolean

  // Computed
  weekData: WeekDayData[]
  maxHours: number
  pendingTasks: Task[]
  completedTasks: Task[]

  // Actions
  fetchData: () => Promise<void>
  refreshData: () => Promise<void>

  // Helpers
  getPriorityColor: (priority: string) => string
  getStatusLabel: (status: string) => string
}

const DAYS_ES = ['Domingo', 'Lunes', 'Martes', 'Miércoles', 'Jueves', 'Viernes', 'Sábado']

const EMPTY_SUMMARY: DashboardSummary = { total_hours: 0, approved_hours: 0, pending_hours: 0 }

export function useDashboard(user: any): UseDashboardReturn {
  const canSeeEmployees = !!(user?.is_superadmin || user?.is_manager || user?.user_type === 'empleador')

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['dashboard', user?.id, canSeeEmployees],
    queryFn: async () => {
      const [tasksData, workHoursData, summaryData] = await Promise.allSettled([
        taskService.getAll({ limit: 10, assignee_id: String(user.id) }),
        workHourService.getAll({ limit: 10 }),
        workHourService.getSummary(),
      ])
      let employees: User[] = []
      if (canSeeEmployees) {
        try { employees = (await userService.getEmployees()) || [] } catch { /* non-fatal */ }
      }
      return {
        tasks: tasksData.status === 'fulfilled' ? (tasksData.value?.data || []) : [],
        workHours: workHoursData.status === 'fulfilled' ? (workHoursData.value?.data || []) : [],
        summary: summaryData.status === 'fulfilled' ? (summaryData.value || EMPTY_SUMMARY) : EMPTY_SUMMARY,
        employees,
      }
    },
    enabled: !!user,
  })

  const tasks = data?.tasks ?? []
  const workHours = data?.workHours ?? []
  const employees = data?.employees ?? []
  const summary = data?.summary ?? EMPTY_SUMMARY

  const fetchData = useCallback(async () => { await refetch() }, [refetch])

  const today = useMemo(() => new Date(), [])

  const weekData = useMemo((): WeekDayData[] => {
    const days: WeekDayData[] = []
    const startOfWeek = new Date(today)
    startOfWeek.setDate(today.getDate() - today.getDay())
    startOfWeek.setHours(0, 0, 0, 0)

    for (let i = 0; i < 7; i++) {
      const date = new Date(startOfWeek)
      date.setDate(startOfWeek.getDate() + i)
      const dateStr = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
      const dayHours = workHours
        .filter(wh => wh.work_date.split('T')[0] === dateStr)
        .reduce((sum, wh) => sum + wh.hours_worked, 0)
      days.push({
        day: DAYS_ES[i].slice(0, 3),
        hours: dayHours,
        target: 8,
      })
    }
    return days
  }, [workHours, today])

  const maxHours = useMemo(() => {
    return Math.max(...weekData.map(d => Math.max(d.hours, d.target)), 8)
  }, [weekData])

  const pendingTasks = useMemo(() => tasks.filter(t => t.status !== 'finalizado'), [tasks])
  const completedTasks = useMemo(() => tasks.filter(t => t.status === 'finalizado'), [tasks])

  const getPriorityColor = useCallback((priority: string): string => {
    const colors: Record<string, string> = {
      urgent: '#ef4444',
      high: '#f97316',
      medium: '#eab308',
      low: '#22c55e',
    }
    return colors[priority] || '#6b7280'
  }, [])

  const getStatusLabel = useCallback((status: string): string => {
    const labels: Record<string, string> = {
      por_hacer: 'Por hacer',
      en_proceso: 'En proceso',
      finalized: 'Finalizado',
    }
    return labels[status] || status
  }, [])

  return {
    tasks,
    workHours,
    employees,
    summary,
    isLoading,
    weekData,
    maxHours,
    pendingTasks,
    completedTasks,
    fetchData,
    refreshData: fetchData,
    getPriorityColor,
    getStatusLabel,
  }
}
