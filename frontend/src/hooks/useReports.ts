import { useState, useEffect, useMemo, useCallback } from 'react'
import { userService, workHourService, taskService } from '../services/api'
import type { User, WorkHour, Task } from '../types'

export function useReports(user: User | null) {
  const [employees, setEmployees] = useState<{ id: number; name: string }[]>([])
  const [selectedEmployee, setSelectedEmployee] = useState<number | ''>('')
  const [workHours, setWorkHours] = useState<WorkHour[]>([])
  const [tasks, setTasks] = useState<Task[]>([])
  const [month, setMonth] = useState(new Date().toISOString().slice(0, 7))
  const [isLoading, setIsLoading] = useState(false)
  const [reportType, setReportType] = useState<'hours' | 'tasks'>('hours')

  const getMonthRange = useCallback((monthStr: string) => {
    const [year, m] = monthStr.split('-').map(Number)
    const startDate = `${year}-${String(m).padStart(2, '0')}-01`
    const lastDay = new Date(year, m, 0).getDate()
    const endDate = `${year}-${String(m).padStart(2, '0')}-${String(lastDay).padStart(2, '0')}`
    return [startDate, endDate]
  }, [])

  const fetchEmployees = useCallback(async () => {
    if (!user?.is_superadmin && !user?.is_manager && user?.user_type !== 'empleador') return
    try {
      const data = await userService.getEmployees()
      setEmployees(data || [])
    } catch (error) {
      console.error('Error fetching employees:', error)
    }
  }, [user])

  const fetchData = useCallback(async () => {
    setIsLoading(true)
    try {
      const [startDate, endDate] = getMonthRange(month)
      let userIdFilter: string | undefined
      if (selectedEmployee) userIdFilter = String(selectedEmployee)
      else if (user?.user_type === 'profesional') userIdFilter = String(user.id)

      const [hoursRes, tasksRes] = await Promise.allSettled([
        workHourService.getAll({ user_id: userIdFilter, start_date: startDate, end_date: endDate }),
        taskService.getAll({ assignee_id: userIdFilter, start_date: startDate, end_date: endDate })
      ])

      if (hoursRes.status === 'fulfilled') setWorkHours(hoursRes.value?.data || [])
      if (tasksRes.status === 'fulfilled') setTasks(tasksRes.value?.data || [])
    } catch (error) {
      console.error('Error fetching data:', error)
    } finally {
      setIsLoading(false)
    }
  }, [selectedEmployee, month, user, getMonthRange])

  useEffect(() => { fetchEmployees() }, [fetchEmployees])
  useEffect(() => {
    if (selectedEmployee || user?.user_type === 'profesional' || user?.user_type === 'empleador' || user?.is_superadmin) {
      fetchData()
    }
  }, [fetchData, selectedEmployee, month, user])

  const hoursStats = useMemo(() => {
    const total = workHours.reduce((sum, wh) => sum + wh.hours_worked, 0)
    const approved = workHours.filter(wh => wh.approved).reduce((sum, wh) => sum + wh.hours_worked, 0)
    const rejected = workHours.filter(wh => wh.rejected).reduce((sum, wh) => sum + wh.hours_worked, 0)
    const pending = Math.max(0, total - approved - rejected)
    const daysWorked = new Set(workHours.map(wh => wh.work_date.split('T')[0])).size
    const targetHours = 160
    return { total, approved, pending, rejected, daysWorked, targetHours, progress: Math.min((total / targetHours) * 100, 100) }
  }, [workHours])

  const dailyData = useMemo(() => {
    const [year, m] = month.split('-').map(Number)
    const daysInMonth = new Date(year, m, 0).getDate()
    const days = []
    const isProf = user?.user_type === 'profesional'
    for (let day = 1; day <= daysInMonth; day++) {
      const dateStr = `${month}-${String(day).padStart(2, '0')}`
      const dayHours = workHours
        .filter(wh => wh.work_date.split('T')[0] === dateStr)
        .reduce((sum, wh) => sum + wh.hours_worked, 0)
      days.push({ day, value: isProf ? (dayHours > 0 ? 1 : 0) : dayHours, target: isProf ? 1 : 8 })
    }
    return days
  }, [workHours, month, user])

  const priorityData = useMemo(() => {
    const counts: Record<string, number> = { urgent: 0, high: 0, medium: 0, low: 0 }
    const LABELS: Record<string, string> = { urgent: 'Urgente', high: 'Alta', medium: 'Media', low: 'Baja' }
    tasks.forEach(t => { if (counts[t.priority] !== undefined) counts[t.priority]++ })
    return Object.entries(counts).filter(([_, v]) => v > 0).map(([k, v]) => ({ name: LABELS[k] || k, value: v }))
  }, [tasks])

  return {
    employees, selectedEmployee, setSelectedEmployee,
    workHours, tasks, month, setMonth,
    isLoading, reportType, setReportType,
    hoursStats, dailyData, priorityData, fetchData
  }
}
