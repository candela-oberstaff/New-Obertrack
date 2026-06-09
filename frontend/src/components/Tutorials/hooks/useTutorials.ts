import { useCallback } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { tutorialService } from '../../../services/api'
import type { Tutorial, CreateTutorialInput, UpdateTutorialInput } from '../../../types'

interface UseTutorialsReturn {
  tutorials: Tutorial[]
  setTutorials: React.Dispatch<React.SetStateAction<Tutorial[]>>
  viewedIds: Set<number>
  isLoading: boolean
  fetchTutorials: () => Promise<void>
  createTutorial: (data: CreateTutorialInput) => Promise<Tutorial>
  updateTutorial: (id: number, data: UpdateTutorialInput) => Promise<void>
  deleteTutorial: (id: number) => Promise<void>
  reorderTutorials: (ids: number[]) => Promise<void>
  recordView: (id: number) => Promise<void>
}

const TUTORIALS_KEY = ['tutorials']
const VIEWS_KEY = ['tutorial-views']

export function useTutorials(): UseTutorialsReturn {
  const qc = useQueryClient()

  const { data: tutorials, isLoading } = useQuery({
    queryKey: TUTORIALS_KEY,
    queryFn: async () => (await tutorialService.getAll()) || [],
  })

  const { data: viewed } = useQuery({
    queryKey: VIEWS_KEY,
    queryFn: async () => {
      try { return (await tutorialService.getMyViews()) || [] } catch { return [] }
    },
  })

  const viewedIds = new Set<number>(viewed ?? [])

  // Shim: lets callers (drag-reorder, inline edits) mutate the cached list
  // directly, preserving the previous useState-based API.
  const setTutorials = useCallback<React.Dispatch<React.SetStateAction<Tutorial[]>>>((value) => {
    qc.setQueryData<Tutorial[]>(TUTORIALS_KEY, (old) =>
      typeof value === 'function' ? (value as (p: Tutorial[]) => Tutorial[])(old ?? []) : value,
    )
  }, [qc])

  const fetchTutorials = useCallback(async () => {
    await qc.invalidateQueries({ queryKey: TUTORIALS_KEY })
  }, [qc])

  const createTutorial = useCallback(async (data: CreateTutorialInput): Promise<Tutorial> => {
    const created = await tutorialService.create(data)
    await qc.invalidateQueries({ queryKey: TUTORIALS_KEY })
    return created
  }, [qc])

  const updateTutorial = useCallback(async (id: number, data: UpdateTutorialInput) => {
    const previous = qc.getQueryData<Tutorial[]>(TUTORIALS_KEY)
    try {
      setTutorials(current => current.map(t => (t.id === id ? { ...t, ...data } : t)))
      await tutorialService.update(id, data)
      await qc.invalidateQueries({ queryKey: TUTORIALS_KEY })
    } catch (error) {
      console.error('Error updating tutorial:', error)
      if (previous) qc.setQueryData(TUTORIALS_KEY, previous)
      throw error
    }
  }, [qc, setTutorials])

  const deleteTutorial = useCallback(async (id: number) => {
    const previous = qc.getQueryData<Tutorial[]>(TUTORIALS_KEY)
    try {
      setTutorials(current => current.filter(t => t.id !== id))
      await tutorialService.delete(id)
    } catch (error) {
      console.error('Error deleting tutorial:', error)
      if (previous) qc.setQueryData(TUTORIALS_KEY, previous)
      throw error
    }
  }, [qc, setTutorials])

  const reorderTutorials = useCallback(async (ids: number[]) => {
    const previous = qc.getQueryData<Tutorial[]>(TUTORIALS_KEY)
    try {
      const map = new Map((previous ?? []).map(t => [t.id, t]))
      const reordered = ids.map(id => map.get(id)).filter((t): t is Tutorial => !!t)
      setTutorials(reordered)
      await tutorialService.reorder(ids)
    } catch (error) {
      console.error('Error reordering tutorials:', error)
      if (previous) qc.setQueryData(TUTORIALS_KEY, previous)
      throw error
    }
  }, [qc, setTutorials])

  const recordView = useCallback(async (id: number) => {
    if (viewedIds.has(id)) return
    qc.setQueryData<number[]>(VIEWS_KEY, (old) => Array.from(new Set([...(old ?? []), id])))
    try {
      await tutorialService.recordView(id)
    } catch (error) {
      console.error('Error recording view:', error)
    }
  }, [qc, viewedIds])

  return {
    tutorials: tutorials ?? [],
    setTutorials,
    viewedIds,
    isLoading,
    fetchTutorials,
    createTutorial,
    updateTutorial,
    deleteTutorial,
    reorderTutorials,
    recordView,
  }
}
