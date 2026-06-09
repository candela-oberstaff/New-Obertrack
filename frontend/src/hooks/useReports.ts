import { useState, useEffect, useMemo, useCallback } from 'react'
import { userService, workHourService, taskService, adminService } from '../services/api'
import type { User, WorkHour, Task } from '../types'

export function useReports(user: User | null) {
  const isSuperadmin = !!user?.is_superadmin

  const [employees, setEmployees] = useState<{ id: number; name: string }[]>([])
  const [selectedEmployee, setSelectedEmployee] = useState<number | ''>('')
  const [companies, setCompanies] = useState<{ id: number; company_name: string }[]>([])
  const [selectedCompanyId, setSelectedCompanyIdState] = useState<number | null>(() => {
    const stored = localStorage.getItem('preferred_company_id')
    return stored ? Number(stored) : null
  })
  const [workHours, setWorkHours] = useState<WorkHour[]>([])
  const [tasks, setTasks] = useState<Task[]>([])
  const [month, setMonth] = useState(new Date().toISOString().slice(0, 7))
  const [isLoading, setIsLoading] = useState(false)
  const [reportType, setReportType] = useState<'hours' | 'tasks'>('hours')

  const setSelectedCompanyId = useCallback((id: number | null) => {
    setSelectedCompanyIdState(id)
    setSelectedEmployee('')
    if (id) {
      localStorage.setItem('preferred_company_id', String(id))
    } else {
      localStorage.removeItem('preferred_company_id')
    }
  }, [])

  const getMonthRange = useCallback((monthStr: string) => {
    const [year, m] = monthStr.split('-').map(Number)
    const startDate = `${year}-${String(m).padStart(2, '0')}-01`
    const lastDay = new Date(year, m, 0).getDate()
    const endDate = `${year}-${String(m).padStart(2, '0')}-${String(lastDay).padStart(2, '0')}`
    return [startDate, endDate]
  }, [])

  // Superadmin: load the company list for the company selector.
  useEffect(() => {
    if (!isSuperadmin) return
    let active = true
    adminService.getTenants()
      .then((res: any) => {
        if (!active) return
        setCompanies((res || []).map((t: any) => ({
          id: t.id,
          company_name: t.company_name || t.owner_name || `Empresa ${t.id}`,
        })))
      })
      .catch((error) => console.error('Error fetching companies:', error))
    return () => { active = false }
  }, [isSuperadmin])

  const fetchEmployees = useCallback(async () => {
    if (!user?.is_superadmin && !user?.is_manager && user?.user_type !== 'empleador') return
    try {
      // Superadmin: employees are scoped to the selected company; without one, none.
      if (isSuperadmin) {
        if (!selectedCompanyId) {
          setEmployees([])
          return
        }
        const data = await adminService.getTenantEmployees(selectedCompanyId)
        setEmployees((data || []).map((e: any) => ({ id: e.id, name: e.name || e.email })))
        return
      }
      const data = await userService.getEmployees()
      setEmployees(data || [])
    } catch (error) {
      console.error('Error fetching employees:', error)
    }
  }, [user, isSuperadmin, selectedCompanyId])

  const fetchData = useCallback(async () => {
    // Superadmin must pick a company first; until then show nothing.
    if (isSuperadmin && !selectedCompanyId) {
      setWorkHours([])
      setTasks([])
      return
    }
    setIsLoading(true)
    try {
      const [startDate, endDate] = getMonthRange(month)
      let userIdFilter: string | undefined
      if (selectedEmployee) userIdFilter = String(selectedEmployee)
      else if (user?.user_type === 'profesional') userIdFilter = String(user.id)

      const companyId = isSuperadmin && selectedCompanyId ? selectedCompanyId : undefined

      const [hoursRes, tasksRes] = await Promise.allSettled([
        workHourService.getAll({ user_id: userIdFilter, start_date: startDate, end_date: endDate, company_id: companyId }),
        taskService.getAll({ assignee_id: userIdFilter, start_date: startDate, end_date: endDate, company_id: companyId })
      ])

      if (hoursRes.status === 'fulfilled') setWorkHours(hoursRes.value?.data || [])
      if (tasksRes.status === 'fulfilled') setTasks(tasksRes.value?.data || [])
    } catch (error) {
      console.error('Error fetching data:', error)
    } finally {
      setIsLoading(false)
    }
  }, [selectedEmployee, month, user, getMonthRange, isSuperadmin, selectedCompanyId])

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
    isSuperadmin,
    companies, selectedCompanyId, setSelectedCompanyId,
    employees, selectedEmployee, setSelectedEmployee,
    workHours, tasks, month, setMonth,
    isLoading, reportType, setReportType,
    hoursStats, dailyData, priorityData, fetchData
  }
}
