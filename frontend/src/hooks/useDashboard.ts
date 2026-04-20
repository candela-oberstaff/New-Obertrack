import { useState, useCallback, useEffect, useMemo } from 'react'
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

export function useDashboard(user: any): UseDashboardReturn {
  const [tasks, setTasks] = useState<Task[]>([])
  const [workHours, setWorkHours] = useState<WorkHour[]>([])
  const [employees, setEmployees] = useState<User[]>([])
  const [summary, setSummary] = useState<DashboardSummary>({ total_hours: 0, approved_hours: 0, pending_hours: 0 })
  const [isLoading, setIsLoading] = useState(true)

  const fetchData = useCallback(async () => {
    try {
      const [tasksData, workHoursData, summaryData] = await Promise.allSettled([
        taskService.getAll({ limit: 10 }),
        workHourService.getAll({ limit: 10 }),
        workHourService.getSummary(),
      ])

      if (tasksData.status === 'fulfilled') {
        setTasks(tasksData.value?.data || [])
      }
      if (workHoursData.status === 'fulfilled') {
        setWorkHours(workHoursData.value?.data || [])
      }
      if (summaryData.status === 'fulfilled') {
        setSummary(summaryData.value || { total_hours: 0, approved_hours: 0, pending_hours: 0 })
      }

      if (user?.is_superadmin || user?.is_manager || user?.user_type === 'empleador') {
        const employeesData = await userService.getEmployees()
        setEmployees(employeesData || [])
      }
    } catch (error) {
      console.error('Error fetching dashboard data:', error)
    } finally {
      setIsLoading(false)
    }
  }, [user])

  useEffect(() => {
    fetchData()
  }, [fetchData])

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
