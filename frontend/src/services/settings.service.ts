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

/** Un tipo de correo del sistema, con su interruptor (Configuración → Correos). */
export interface EmailType {
  key: string
  name: string
  description: string
  trigger: string
  recipient: string
  category: 'automatic' | 'event' | 'manual'
  essential: boolean
  /** Si viene, su encendido se gobierna en otra sección (no lleva toggle propio). */
  managed_elsewhere?: string
  enabled: boolean
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
  // --- Correos del sistema ---
  getEmailTypes: async () => {
    const { data } = await api.get<EmailType[]>('/admin/settings/emails')
    return data
  },
  setEmailEnabled: async (key: string, enabled: boolean) => {
    const { data } = await api.put<{ key: string; enabled: boolean }>(`/admin/settings/emails/${key}`, { enabled })
    return data
  },
  /** Envía una muestra del correo (a la sesión si no se indica destinatario). */
  sendEmailTest: async (key: string, email?: string) => {
    const { data } = await api.post<{ message: string; email: string }>(`/admin/settings/emails/${key}/test`, { email })
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
