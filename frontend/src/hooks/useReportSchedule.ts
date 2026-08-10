import { useCallback, useState } from 'react'
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { settingsService } from '../services/api'
import type { ReportSchedule } from '../services/api'

const SCHEDULE_KEY = ['settings', 'report-schedule'] as const
const RUNS_KEY = ['settings', 'report-runs'] as const

const RUNS_PAGE_SIZE = 10

export function useReportSchedule() {
  const qc = useQueryClient()

  const scheduleQ = useQuery({
    queryKey: SCHEDULE_KEY,
    queryFn: () => settingsService.getReportSchedule(),
  })

  // Bitácora paginada. keepPreviousData evita el parpadeo al cambiar de página.
  const [runsPage, setRunsPage] = useState(1)
  const runsQ = useQuery({
    queryKey: [...RUNS_KEY, runsPage],
    queryFn: () => settingsService.getReportRuns(runsPage, RUNS_PAGE_SIZE),
    placeholderData: keepPreviousData,
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
    runs: runsQ.data?.runs ?? [],
    runsTotal: runsQ.data?.total ?? 0,
    runsPage,
    setRunsPage,
    runsPageSize: RUNS_PAGE_SIZE,
    isLoading: scheduleQ.isLoading,
    save: saveMut.mutateAsync,
    isSaving: saveMut.isPending,
    runNow: runNowMut.mutateAsync,
    isRunning: runNowMut.isPending,
  }
}
