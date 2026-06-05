import { useState, useCallback, useEffect, useMemo } from 'react'
import { workHourService } from '../services/api'
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
}

const JORNADA_COMPLETA = 8

export function useWorkHours(user: any): UseWorkHoursReturn {
  const [workHours, setWorkHours] = useState<WorkHour[]>([])
  const [pendingHours, setPendingHours] = useState<WorkHour[]>([])
  const [summary, setSummary] = useState<WorkHoursSummary>({ total_hours: 0, approved_hours: 0, pending_hours: 0, rejected_hours: 0 })
  const [isLoading, setIsLoading] = useState(true)
  const [selectedDate, setSelectedDate] = useState<string | null>(null)
  const [currentMonth, setCurrentMonth] = useState(new Date().getMonth())
  const [currentYear, setCurrentYear] = useState(new Date().getFullYear())

  const [formData, setFormData] = useState<WorkHourFormData>({
    work_date: new Date().toISOString().split('T')[0],
    work_type: 'complete',
    activities: '',
    absence_reason: '',
    absence_hours: 0,
  })

  const fetchData = useCallback(async () => {
    try {
      const isEmployer = user?.user_type === 'empleador'
      const isSuperadmin = user?.is_superadmin

      const [hoursRes, summaryRes] = await Promise.all([
        workHourService.getAll({}),
        workHourService.getSummary(),
      ])
      setWorkHours(hoursRes.data || [])
      setSummary(summaryRes)

      if (isEmployer || isSuperadmin) {
        const pendingRes = await workHourService.getPending()
        setPendingHours(pendingRes || [])
      }
    } catch (error) {
      console.error('Error fetching data:', error)
    } finally {
      setIsLoading(false)
    }
  }, [user])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  const createWorkHour = useCallback(async (data: Omit<WorkHourFormData, 'absence_reason' | 'absence_hours'> & { id?: number; absence_reason?: string; absence_hours?: number }) => {
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
    })
  }, [])

  const filteredHours = useMemo(() => {
    if (!selectedDate) return workHours
    return workHours.filter(wh => wh.work_date.split('T')[0] === selectedDate)
  }, [workHours, selectedDate])

  const pendingForSelectedDate = useMemo(() => {
    if (selectedDate) {
      return filteredHours.filter(wh => !wh.approved && !wh.rejected && wh.hours_worked > 0)
    }
    return workHours.filter(wh => !wh.approved && !wh.rejected && wh.hours_worked > 0)
  }, [filteredHours, selectedDate, workHours])

  const weekHours = useMemo(() => {
    const now = new Date()
    const startOfWeek = new Date(now)
    startOfWeek.setDate(now.getDate() - now.getDay())
    startOfWeek.setHours(0, 0, 0, 0)
    const startStr = startOfWeek.toISOString().split('T')[0]

    return workHours
      .filter(wh => wh.work_date.split('T')[0] >= startStr)
      .reduce((sum, wh) => sum + wh.hours_worked, 0)
  }, [workHours])

  const todayWork = useMemo(() => {
    const today = new Date().toISOString().split('T')[0]
    return workHours.find(wh => wh.work_date.split('T')[0] === today)
  }, [workHours])

  const canApprove = user?.is_superadmin || user?.is_manager || user?.user_type === 'empleador'

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
  }
}
