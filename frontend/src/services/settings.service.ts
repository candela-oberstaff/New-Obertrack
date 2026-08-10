import api from './client'

export type ReportFrequency = 'daily' | 'weekly' | 'monthly'

export interface ReportSchedule {
  id: number
  enabled: boolean
  frequency: ReportFrequency
  hour: number
  minute: number
  timezone: string
  weekday: number
  day_of_month: number
  updated_by?: number
  updated_at: string
}

export interface ReportRun {
  id: number
  tenant_id: number
  period_key: string
  frequency: string
  recipient_email: string
  recipient_name: string
  status: 'sent' | 'failed'
  error?: string
  created_at: string
}

export interface RunNowResult {
  sent: number
  skipped: number
  failed: number
}

export const settingsService = {
  getReportSchedule: async () => {
    const { data } = await api.get<ReportSchedule>('/admin/settings/report-schedule')
    return data
  },
  updateReportSchedule: async (payload: Partial<ReportSchedule>) => {
    const { data } = await api.put<ReportSchedule>('/admin/settings/report-schedule', payload)
    return data
  },
  /** Corrida manual: ignora la hora programada pero respeta la deduplicación. */
  runReportNow: async () => {
    const { data } = await api.post<RunNowResult>('/admin/settings/report-schedule/run-now')
    return data
  },
  /** Bitácora paginada: página 1-based; devuelve también el total. */
  getReportRuns: async (page = 1, limit = 10) => {
    const { data } = await api.get<{ runs: ReportRun[]; total: number; page: number; limit: number }>(
      '/admin/settings/report-schedule/runs',
      { params: { page, limit } },
    )
    return data
  },
}
