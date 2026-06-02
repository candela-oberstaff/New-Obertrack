import { useState, useCallback, useEffect } from 'react'
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

export function useTutorials(): UseTutorialsReturn {
  const [tutorials, setTutorials] = useState<Tutorial[]>([])
  const [viewedIds, setViewedIds] = useState<Set<number>>(new Set())
  const [isLoading, setIsLoading] = useState(true)

  const fetchTutorials = useCallback(async () => {
    try {
      const [data, viewed] = await Promise.all([
        tutorialService.getAll(),
        tutorialService.getMyViews().catch(() => []),
      ])
      setTutorials(data || [])
      setViewedIds(new Set(viewed))
    } catch (error) {
      console.error('Error fetching tutorials:', error)
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchTutorials()
  }, [fetchTutorials])

  const createTutorial = useCallback(async (data: CreateTutorialInput): Promise<Tutorial> => {
    const created = await tutorialService.create(data)
    await fetchTutorials()
    return created
  }, [fetchTutorials])

  const updateTutorial = useCallback(async (id: number, data: UpdateTutorialInput) => {
    const previous = [...tutorials]
    try {
      setTutorials(current =>
        current.map(t => (t.id === id ? { ...t, ...data } : t))
      )
      await tutorialService.update(id, data)
      await fetchTutorials()
    } catch (error) {
      console.error('Error updating tutorial:', error)
      setTutorials(previous)
      throw error
    }
  }, [fetchTutorials, tutorials])

  const deleteTutorial = useCallback(async (id: number) => {
    const previous = [...tutorials]
    try {
      setTutorials(current => current.filter(t => t.id !== id))
      await tutorialService.delete(id)
    } catch (error) {
      console.error('Error deleting tutorial:', error)
      setTutorials(previous)
      throw error
    }
  }, [tutorials])

  const reorderTutorials = useCallback(async (ids: number[]) => {
    const previous = [...tutorials]
    try {
      const map = new Map(previous.map(t => [t.id, t]))
      const reordered = ids.map(id => map.get(id)).filter((t): t is Tutorial => !!t)
      setTutorials(reordered)
      await tutorialService.reorder(ids)
    } catch (error) {
      console.error('Error reordering tutorials:', error)
      setTutorials(previous)
      throw error
    }
  }, [tutorials])

  const recordView = useCallback(async (id: number) => {
    if (viewedIds.has(id)) return
    setViewedIds(current => new Set(current).add(id))
    try {
      await tutorialService.recordView(id)
    } catch (error) {
      console.error('Error recording view:', error)
    }
  }, [viewedIds])

  return {
    tutorials,
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
