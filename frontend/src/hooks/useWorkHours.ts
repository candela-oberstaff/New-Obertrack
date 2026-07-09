import { useState, useCallback, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { workHourService } from '../services/api'
import { canEditModule } from '../lib/permissions'
import type { WorkHour } from '../types'

interface WorkHoursSummary {
  total_hours: number
  approved_hours: number
  pending_hours: number
  rejected_hours: number
}

interface WorkHourFormData {
  work_date: string
  work_type: 'complete' | 'absence' | 'recover'
  activities: string
  absence_reason: string
  absence_hours: number
  comments: string
}

interface UseWorkHoursReturn {
  // Data
  workHours: WorkHour[]
  summary: WorkHoursSummary
  pendingHours: WorkHour[]
  isLoading: boolean

  // Calendar state
  selectedDate: string | null
  setSelectedDate: (date: string | null) => void
  currentMonth: number
  currentYear: number
  setCurrentMonth: (month: number) => void
  setCurrentYear: (year: number) => void

  // Form state
  formData: WorkHourFormData
  setFormData: (data: WorkHourFormData) => void
  resetForm: () => void

  // Actions
  fetchData: () => Promise<void>
  createWorkHour: (data: Omit<WorkHourFormData, 'absence_reason' | 'absence_hours'> & { id?: number; absence_reason?: string; absence_hours?: number }) => Promise<void>
  approveWorkHours: (ids: number[]) => Promise<void>
  approveSingle: (id: number) => Promise<void>
  rejectSingle: (id: number, reason: string) => Promise<void>

  // Computed
  filteredHours: WorkHour[]
  pendingForSelectedDate: WorkHour[]
  weekHours: number
  todayWork: WorkHour | undefined
  canApprove: boolean
  /** false cuando el rol del usuario deja Horas en solo lectura. */
  canEditHours: boolean
}

const JORNADA_COMPLETA = 8

interface UseWorkHoursOptions {
  // Superadmin scope: the company (tenant) and optional employee currently selected.
  companyId?: number | null
  employeeId?: number | null
}

const EMPTY_SUMMARY: WorkHoursSummary = { total_hours: 0, approved_hours: 0, pending_hours: 0, rejected_hours: 0 }

export function useWorkHours(user: any, options: UseWorkHoursOptions = {}): UseWorkHoursReturn {
  const { companyId = null, employeeId = null } = options
  const [selectedDate, setSelectedDate] = useState<string | null>(null)
  const [currentMonth, setCurrentMonth] = useState(new Date().getMonth())
  const [currentYear, setCurrentYear] = useState(new Date().getFullYear())

  const [formData, setFormData] = useState<WorkHourFormData>({
    work_date: new Date().toISOString().split('T')[0],
    work_type: 'complete',
    activities: '',
    absence_reason: '',
    absence_hours: 0,
    comments: '',
  })

  const isEmployer = user?.user_type === 'empleador'
  const isSuperadmin = user?.is_superadmin
  const isManager = user?.is_manager
  const canSeePending = isEmployer || isSuperadmin || isManager

  // Refetch al navegar de mes: los cálculos de calendario/recuperación/ausencias
  // del mes visible necesitan los registros de ESE mes, no solo la primera página.
  const queryKey = ['work-hours', companyId, employeeId, user?.id, currentMonth, currentYear]

  const { data, isLoading, refetch } = useQuery({
    queryKey,
    // Superadmin must pick a company first; otherwise nothing is fetched.
    enabled: !!user && !(isSuperadmin && !companyId),
    queryFn: async () => {
      const scope = {
        ...(companyId ? { company_id: companyId } : {}),
        ...(employeeId ? { user_id: String(employeeId) } : {}),
      }
      // Ventana de datos = unión del mes visible y la semana actual, así tanto
      // el calendario/lista del mes como las tarjetas "Hoy"/"Esta semana" tienen
      // todos sus registros (antes el límite de 10 truncaba silenciosamente).
      const fmt = (d: Date) =>
        `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
      const monthStart = new Date(currentYear, currentMonth, 1)
      const monthEnd = new Date(currentYear, currentMonth + 1, 0)
      const now = new Date()
      const weekStart = new Date(now)
      weekStart.setDate(now.getDate() - now.getDay())
      weekStart.setHours(0, 0, 0, 0)
      const rangeStart = monthStart < weekStart ? monthStart : weekStart
      const rangeEnd = monthEnd > now ? monthEnd : now
      const dateScope = { start_date: fmt(rangeStart), end_date: fmt(rangeEnd), limit: 1000 }

      const [hoursRes, summaryRes] = await Promise.all([
        workHourService.getAll({ ...scope, ...dateScope }),
        workHourService.getSummary({ ...scope }),
      ])
      if (canSeePending) {
        const pendingRes = await workHourService.getPending({ ...scope })
        const byID = new Map<number, WorkHour>()
        ;(hoursRes.data || []).forEach((wh) => byID.set(wh.id, wh))
        ;(pendingRes || []).forEach((wh) => byID.set(wh.id, wh))
        return { workHours: Array.from(byID.values()), pendingHours: pendingRes || [], summary: summaryRes }
      }
      return { workHours: hoursRes.data || [], pendingHours: [] as WorkHour[], summary: summaryRes }
    },
  })

  const workHours = data?.workHours ?? []
  const pendingHours = data?.pendingHours ?? []
  const summary = data?.summary ?? EMPTY_SUMMARY

  const fetchData = useCallback(async () => { await refetch() }, [refetch])

  const createWorkHour = useCallback(async (data: Omit<WorkHourFormData, 'absence_reason' | 'absence_hours' | 'comments'> & { id?: number; absence_reason?: string; absence_hours?: number; comments?: string }) => {
    const hoursWorked = data.work_type === 'recover'
      ? (data as any).hours_worked || JORNADA_COMPLETA
      : data.work_type === 'absence'
      ? Math.max(0, JORNADA_COMPLETA - (data.absence_hours || 0))
      : JORNADA_COMPLETA

    const payload = {
      work_date: data.work_date,
      work_type: data.work_type,
      activities: data.activities || undefined,
      hours_worked: hoursWorked,
      absence_reason: data.work_type === 'absence' ? data.absence_reason : undefined,
      absence_hours: data.work_type === 'absence' ? data.absence_hours : undefined,
      comments: data.work_type === 'absence' ? (data.comments || '') : undefined,
    }

    if (data.id) {
      await workHourService.update(data.id, payload)
    } else {
      await workHourService.create(payload)
    }
    fetchData()
  }, [fetchData])

  const approveWorkHours = useCallback(async (ids: number[]) => {
    await workHourService.approve(ids)
    fetchData()
  }, [fetchData])

  const approveSingle = useCallback(async (id: number) => {
    await workHourService.approve([id])
    fetchData()
  }, [fetchData])

  const rejectSingle = useCallback(async (id: number, reason: string) => {
    await workHourService.reject([id], reason)
    fetchData()
  }, [fetchData])

  const resetForm = useCallback(() => {
    setFormData({
      work_date: new Date().toISOString().split('T')[0],
      work_type: 'complete',
      activities: '',
      absence_reason: '',
      absence_hours: 0,
      comments: '',
    })
  }, [])

  // Registros del mes visible (la ventana traída puede incluir la semana actual
  // de otro mes; la lista "Mis registros" se acota al mes que se está viendo).
  const monthHours = useMemo(() => {
    const ym = `${currentYear}-${String(currentMonth + 1).padStart(2, '0')}`
    return workHours.filter(wh => wh.work_date.split('T')[0].startsWith(ym))
  }, [workHours, currentMonth, currentYear])

  const filteredHours = useMemo(() => {
    if (!selectedDate) return monthHours
    return workHours.filter(wh => wh.work_date.split('T')[0] === selectedDate)
  }, [workHours, monthHours, selectedDate])

  const pendingForSelectedDate = useMemo(() => {
    if (selectedDate) {
      return filteredHours.filter(wh => !wh.approved && !wh.rejected && wh.hours_worked > 0)
    }
    return workHours.filter(wh => !wh.approved && !wh.rejected && wh.hours_worked > 0)
  }, [filteredHours, selectedDate, workHours])

  const weekHours = useMemo(() => {
    // Inicio/fin de la semana actual en hora LOCAL (evita el corrimiento de día
    // de toISOString/UTC) y acotada a [inicio, fin] para no sumar otras semanas.
    const fmt = (d: Date) =>
      `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
    const now = new Date()
    const startOfWeek = new Date(now)
    startOfWeek.setDate(now.getDate() - now.getDay())
    startOfWeek.setHours(0, 0, 0, 0)
    const endOfWeek = new Date(startOfWeek)
    endOfWeek.setDate(startOfWeek.getDate() + 6)
    const startStr = fmt(startOfWeek)
    const endStr = fmt(endOfWeek)

    return workHours
      .filter(wh => {
        const ds = wh.work_date.split('T')[0]
        return ds >= startStr && ds <= endStr
      })
      .reduce((sum, wh) => sum + wh.hours_worked, 0)
  }, [workHours])

  const todayWork = useMemo(() => {
    const today = new Date().toISOString().split('T')[0]
    return workHours.find(wh => wh.work_date.split('T')[0] === today)
  }, [workHours])

  // Permiso del rol sobre el módulo Horas: con "Ver" se ocultan las acciones
  // de escritura (el backend igual las bloquea con 403).
  const canEditHours = canEditModule(user, 'hours')
  const canApprove = !!(user?.is_superadmin || user?.is_manager || user?.user_type === 'empleador') && canEditHours

  return {
    workHours,
    summary,
    pendingHours,
    isLoading,
    selectedDate,
    setSelectedDate,
    currentMonth,
    currentYear,
    setCurrentMonth,
    setCurrentYear,
    formData,
    setFormData,
    resetForm,
    fetchData,
    createWorkHour,
    approveWorkHours,
    approveSingle,
    rejectSingle,
    filteredHours,
    pendingForSelectedDate,
    weekHours,
    todayWork,
    canApprove,
    canEditHours,
  }
}
