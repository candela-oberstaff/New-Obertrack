import { useState, useCallback, useEffect } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { taskService } from '../../../services/api'
import type { Task, CreateTaskInput } from '../../../types'

interface UseTasksOptions {
  boardId?: number | null
  showAllTasks?: boolean
  // For superadmin: scopes the task query to a single company (tenant).
  companyId?: number | null
}

interface UseTasksReturn {
  tasks: Task[]
  selectedTask: Task | null
  setSelectedTask: (task: Task | null) => void
  isLoading: boolean
  createTask: (data: CreateTaskInput) => Promise<Task>
  updateTask: (id: number, data: Partial<Task>) => Promise<void>
  deleteTask: (id: number) => Promise<void>
  fetchTasks: () => Promise<void>
  getTasksByStatus: (status: string) => Task[]
}

export function useTasks({ boardId, showAllTasks, companyId }: UseTasksOptions = {}): UseTasksReturn {
  const qc = useQueryClient()
  const [selectedTask, setSelectedTask] = useState<Task | null>(null)

  const queryKey = ['tasks', boardId ?? null, !!showAllTasks, companyId ?? null]

  const { data: tasks = [], isLoading, refetch } = useQuery({
    queryKey,
    queryFn: async () => {
      const params: Record<string, unknown> = { limit: 1000 }
      if (!showAllTasks && boardId) params.board_id = boardId
      if (companyId) params.company_id = companyId
      const tasksRes = await taskService.getAll(params)
      let fetched = tasksRes.data || []
      if (!showAllTasks && boardId) fetched = fetched.filter((t: any) => t.board_id === boardId)
      return fetched as Task[]
    },
  })

  const fetchTasks = useCallback(async () => { await refetch() }, [refetch])

  // External signal (e.g. a task assigned elsewhere) → refresh.
  useEffect(() => {
    const handler = () => { refetch() }
    window.addEventListener('task-assigned', handler)
    return () => window.removeEventListener('task-assigned', handler)
  }, [refetch])

  const createTask = useCallback(async (data: CreateTaskInput): Promise<Task> => {
    const newTask = await taskService.create(data)
    await qc.invalidateQueries({ queryKey })
    return newTask
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [qc, boardId, showAllTasks, companyId])

  const updateTask = useCallback(async (id: number, data: Partial<Task>) => {
    const previous = qc.getQueryData<Task[]>(queryKey)
    // Optimistic update (skip assignees — they come as objects not IDs).
    qc.setQueryData<Task[]>(queryKey, (old) =>
      (old ?? []).map((t) => {
        if (t.id === id) {
          const { assignees, ...rest } = data
          return { ...t, ...rest }
        }
        return t
      }),
    )
    try {
      await taskService.update(id, data)
      await qc.invalidateQueries({ queryKey })
      if (selectedTask && selectedTask.id === id) {
        setSelectedTask(await taskService.getById(id))
      }
    } catch (error) {
      console.error('Error updating task:', error)
      if (previous) qc.setQueryData(queryKey, previous)
      throw error
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [qc, boardId, showAllTasks, companyId, selectedTask])

  const deleteTask = useCallback(async (id: number) => {
    await taskService.delete(id)
    await qc.invalidateQueries({ queryKey })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [qc, boardId, showAllTasks, companyId])

  const getTasksByStatus = useCallback((status: string) => {
    return tasks.filter((task) => task.status === status)
  }, [tasks])

  return {
    tasks,
    selectedTask,
    setSelectedTask,
    isLoading,
    createTask,
    updateTask,
    deleteTask,
    fetchTasks,
    getTasksByStatus,
  }
}
