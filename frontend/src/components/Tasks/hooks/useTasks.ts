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
  moveTask: (id: number, boardId: number, newStatus: string, orderedIds: number[]) => Promise<void>
  reorderColumn: (boardId: number, status: string, orderedIds: number[]) => Promise<void>
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
      // 200 es el tope por tablero. En producción con limit=1000 se descargaban
      // hasta 34 MB por apertura de tablero (tasks?limit=1000 → 34,781 kB).
      // Con 200 el payload cae ~5x manteniendo capacidad para tableros grandes.
      // Si un tablero supera 200 tareas se emite un console.warn (ver abajo).
      const params: Record<string, unknown> = { limit: 200 }
      if (!showAllTasks && boardId) params.board_id = boardId
      if (companyId) params.company_id = companyId
      const tasksRes = await taskService.getAll(params)
      let fetched = tasksRes.data || []
      if (tasksRes.total > fetched.length) {
        console.warn(`[useTasks] El tablero tiene ${tasksRes.total} tareas pero solo se cargaron ${fetched.length}; las restantes no se muestran.`)
      }
      if (!showAllTasks && boardId) fetched = fetched.filter((t: any) => t.board_id === boardId)
      return fetched as Task[]
    },
    enabled: !!showAllTasks || !!boardId,
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

  // Protocolo optimista compartido por updateTask/moveTask/reorderColumn:
  // snapshot → aplicar al cache → persistir → invalidate. Si persist falla a
  // medias (p. ej. cambió el status pero falló el reorden), el snapshot local
  // podría no reflejar lo que el servidor sí guardó, así que tras revertir
  // TAMBIÉN se invalida para resincronizar con la verdad del servidor.
  const runOptimistic = useCallback(async (apply: (old: Task[]) => Task[], persist: () => Promise<void>) => {
    const previous = qc.getQueryData<Task[]>(queryKey)
    qc.setQueryData<Task[]>(queryKey, (old) => apply(old ?? []))
    try {
      await persist()
      await qc.invalidateQueries({ queryKey })
    } catch (error) {
      console.error('Error persisting task change:', error)
      if (previous) qc.setQueryData(queryKey, previous)
      await qc.invalidateQueries({ queryKey })
      throw error
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [qc, boardId, showAllTasks, companyId])

  const refreshSelectedTask = useCallback(async (id: number) => {
    if (selectedTask && selectedTask.id === id) {
      setSelectedTask(await taskService.getById(id))
    }
  }, [selectedTask])

  const updateTask = useCallback(async (id: number, data: Partial<Task>) => {
    await runOptimistic(
      // Optimistic update (skip assignees — they come as objects not IDs).
      (old) => old.map((t) => {
        if (t.id === id) {
          const { assignees, ...rest } = data
          return { ...t, ...rest }
        }
        return t
      }),
      async () => {
        await taskService.update(id, data)
        await refreshSelectedTask(id)
      },
    )
  }, [runOptimistic, refreshSelectedTask])

  // Mueve una tarjeta a otra columna dejándola en una posición concreta:
  // actualiza status + order de forma optimista, persiste el status y después
  // el orden de la columna destino (el endpoint de reorden filtra por status,
  // así que debe correr tras el update).
  const moveTask = useCallback(async (id: number, boardId: number, newStatus: string, orderedIds: number[]) => {
    await runOptimistic(
      (old) => old.map((t) => {
        const idx = orderedIds.indexOf(t.id)
        if (t.id === id) return { ...t, status: newStatus, order: idx >= 0 ? idx : t.order }
        return idx >= 0 ? { ...t, order: idx } : t
      }),
      async () => {
        await taskService.update(id, { status: newStatus })
        try {
          await taskService.reorder(boardId, newStatus, orderedIds)
        } catch (reorderError) {
          console.warn('No se pudo persistir la posición exacta (la tarea sí cambió de columna):', reorderError)
        }
        await refreshSelectedTask(id)
      },
    )
  }, [runOptimistic, refreshSelectedTask])

  // Reordena las tarjetas dentro de una columna (orden manual persistido).
  const reorderColumn = useCallback(async (boardId: number, status: string, orderedIds: number[]) => {
    await runOptimistic(
      (old) => old.map((t) => {
        const idx = orderedIds.indexOf(t.id)
        return idx >= 0 ? { ...t, order: idx } : t
      }),
      () => taskService.reorder(boardId, status, orderedIds),
    )
  }, [runOptimistic])

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
    moveTask,
    reorderColumn,
    deleteTask,
    fetchTasks,
    getTasksByStatus,
  }
}
