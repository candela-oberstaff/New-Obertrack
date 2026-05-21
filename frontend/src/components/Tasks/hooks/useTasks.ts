import { useState, useCallback, useEffect } from 'react'
import { taskService } from '../../../services/api'
import type { Task, CreateTaskInput } from '../../../types'

interface UseTasksOptions {
  boardId?: number | null
  showAllTasks?: boolean
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

export function useTasks({ boardId, showAllTasks }: UseTasksOptions = {}): UseTasksReturn {
  const [tasks, setTasks] = useState<Task[]>([])
  const [selectedTask, setSelectedTask] = useState<Task | null>(null)
  const [isLoading] = useState(false)

  const fetchTasks = useCallback(async () => {
    try {
      const params: Record<string, unknown> = { limit: 1000 }
      if (!showAllTasks && boardId) {
        params.board_id = boardId
      }
      const tasksRes = await taskService.getAll(params)
      let fetchedTasks = tasksRes.data || []
      if (!showAllTasks && boardId) {
        fetchedTasks = fetchedTasks.filter((t: any) => t.board_id === boardId)
      }
      setTasks(fetchedTasks)
    } catch (error) {
      console.error('Error fetching tasks:', error)
    }
  }, [boardId, showAllTasks])

  useEffect(() => {
    fetchTasks()
  }, [fetchTasks])

  useEffect(() => {
    setTasks([])
  }, [boardId, showAllTasks])

  useEffect(() => {
    const handleTaskAssigned = () => {
      fetchTasks()
    }
    window.addEventListener('task-assigned', handleTaskAssigned)
    return () => window.removeEventListener('task-assigned', handleTaskAssigned)
  }, [fetchTasks])

  const createTask = useCallback(async (data: CreateTaskInput): Promise<Task> => {
    const newTask = await taskService.create(data)
    fetchTasks()
    return newTask
  }, [fetchTasks])

  const updateTask = useCallback(async (id: number, data: Partial<Task>) => {
    const previousTasks = [...tasks]
    try {
      setTasks(currentTasks =>
        currentTasks.map(t => {
          if (t.id === id) {
            // No actualizar asignados de forma optimista si solo vienen IDs
            const { assignees, ...rest } = data
            return { ...t, ...rest }
          }
          return t
        })
      )

      await taskService.update(id, data)
      // We still fetch to ensure full sync (e.g., computed fields, timestamps)
      fetchTasks()

      if (selectedTask && selectedTask.id === id) {
        const updated = await taskService.getById(id)
        setSelectedTask(updated)
      }
    } catch (error) {
      console.error('Error updating task:', error)
      // Rollback on failure
      setTasks(previousTasks)
    }
  }, [fetchTasks, selectedTask, tasks])

  const deleteTask = useCallback(async (id: number) => {
    await taskService.delete(id)
    fetchTasks()
  }, [fetchTasks])

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
