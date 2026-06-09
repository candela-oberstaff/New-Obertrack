import { useState, useEffect, useMemo, useCallback } from 'react'
import { useQuery } from '@tanstack/react-query'
import { userService, workHourService, taskService, adminService } from '../services/api'
import { useNotification } from '../context/NotificationContext'
import type { User, WorkHour, Task } from '../types'

function getMonthRange(monthStr: string) {
  const [year, m] = monthStr.split('-').map(Number)
  const startDate = `${year}-${String(m).padStart(2, '0')}-01`
  const lastDay = new Date(year, m, 0).getDate()
  const endDate = `${year}-${String(m).padStart(2, '0')}-${String(lastDay).padStart(2, '0')}`
  return [startDate, endDate]
}

export function useReports(user: User | null) {
  const { error: showError } = useNotification()
  const isSuperadmin = !!user?.is_superadmin

  const [selectedEmployee, setSelectedEmployee] = useState<number | ''>('')
  const [selectedCompanyId, setSelectedCompanyIdState] = useState<number | null>(() => {
    const stored = localStorage.getItem('preferred_company_id')
    return stored ? Number(stored) : null
  })
  const [month, setMonth] = useState(new Date().toISOString().slice(0, 7))
  const [reportType, setReportType] = useState<'hours' | 'tasks'>('hours')

  const setSelectedCompanyId = useCallback((id: number | null) => {
    setSelectedCompanyIdState(id)
    setSelectedEmployee('')
    if (id) localStorage.setItem('preferred_company_id', String(id))
    else localStorage.removeItem('preferred_company_id')
  }, [])

  // Company selector (superadmin only)
  const { data: companies = [] } = useQuery({
    queryKey: ['report-companies'],
    enabled: isSuperadmin,
    queryFn: async (): Promise<{ id: number; company_name: string }[]> => {
      const res: any = await adminService.getTenants()
      return (res || []).map((t: any) => ({ id: t.id, company_name: t.company_name || t.owner_name || `Empresa ${t.id}` }))
    },
  })

  // Employees: superadmin → scoped to selected company; others → their team.
  const canSeeEmployees = isSuperadmin || user?.is_manager || user?.user_type === 'empleador'
  const { data: employees = [] } = useQuery({
    queryKey: ['report-employees', isSuperadmin, selectedCompanyId, user?.id],
    enabled: !!canSeeEmployees && (!isSuperadmin || !!selectedCompanyId),
    queryFn: async (): Promise<{ id: number; name: string }[]> => {
      if (isSuperadmin) {
        const data: any = await adminService.getTenantEmployees(selectedCompanyId as number)
        return (data || []).map((e: any) => ({ id: e.id, name: e.name || e.email }))
      }
      const data: any = await userService.getEmployees()
      return (data || []).map((e: any) => ({ id: e.id, name: e.name || e.email }))
    },
  })

  const [startDate, endDate] = getMonthRange(month)
  const userIdFilter = selectedEmployee
    ? String(selectedEmployee)
    : user?.user_type === 'profesional' ? String(user.id) : undefined
  const companyId = isSuperadmin && selectedCompanyId ? selectedCompanyId : undefined

  const reportQ = useQuery({
    queryKey: ['report-data', month, userIdFilter ?? null, companyId ?? null],
    // Superadmin must pick a company first; everyone else loads once user is known.
    enabled: !!user && (!isSuperadmin || !!selectedCompanyId),
    queryFn: async () => {
      const [hoursRes, tasksRes] = await Promise.allSettled([
        workHourService.getAll({ user_id: userIdFilter, start_date: startDate, end_date: endDate, company_id: companyId }),
        taskService.getAll({ assignee_id: userIdFilter, start_date: startDate, end_date: endDate, company_id: companyId }),
      ])
      if (hoursRes.status === 'rejected' && tasksRes.status === 'rejected') {
        throw new Error('No se pudieron cargar los datos del reporte.')
      }
      return {
        workHours: hoursRes.status === 'fulfilled' ? (hoursRes.value?.data || []) : [],
        tasks: tasksRes.status === 'fulfilled' ? (tasksRes.value?.data || []) : [],
      }
    },
  })

  useEffect(() => {
    if (reportQ.error) showError('No se pudieron cargar los datos del reporte.')
  }, [reportQ.error, showError])

  const workHours: WorkHour[] = reportQ.data?.workHours ?? []
  const tasks: Task[] = reportQ.data?.tasks ?? []

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
    isLoading: reportQ.isLoading, reportType, setReportType,
    hoursStats, dailyData, priorityData,
    fetchData: async () => { await reportQ.refetch() },
  }
}
