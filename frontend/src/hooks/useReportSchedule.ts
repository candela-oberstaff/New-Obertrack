import { useCallback } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { settingsService } from '../services/api'
import type { ReportSchedule } from '../services/api'

const SCHEDULE_KEY = ['settings', 'report-schedule'] as const
const RUNS_KEY = ['settings', 'report-runs'] as const

export function useReportSchedule() {
  const qc = useQueryClient()

  const scheduleQ = useQuery({
    queryKey: SCHEDULE_KEY,
    queryFn: () => settingsService.getReportSchedule(),
  })

  const runsQ = useQuery({
    queryKey: RUNS_KEY,
    queryFn: () => settingsService.getReportRuns(50),
  })

  const invalidateAll = useCallback(async () => {
    await Promise.all([
      qc.invalidateQueries({ queryKey: SCHEDULE_KEY }),
      qc.invalidateQueries({ queryKey: RUNS_KEY }),
    ])
  }, [qc])

  const saveMut = useMutation({
    mutationFn: (payload: Partial<ReportSchedule>) => settingsService.updateReportSchedule(payload),
    onSuccess: () => qc.invalidateQueries({ queryKey: SCHEDULE_KEY }),
  })

  // Tras una corrida manual cambia la bitácora, así que se refresca también.
  const runNowMut = useMutation({
    mutationFn: () => settingsService.runReportNow(),
    onSuccess: invalidateAll,
  })

  return {
    schedule: scheduleQ.data,
    runs: runsQ.data ?? [],
    isLoading: scheduleQ.isLoading,
    save: saveMut.mutateAsync,
    isSaving: saveMut.isPending,
    runNow: runNowMut.mutateAsync,
    isRunning: runNowMut.isPending,
  }
}
