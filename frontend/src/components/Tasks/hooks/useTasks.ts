import { useState, useCallback, useEffect } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { taskService } from '../../../services/api'
import { gateFromError } from '../../../services/task.service'
import type { Task, CreateTaskInput, TaskGateRequirement } from '../../../types'

interface UseTasksOptions {
  boardId?: number | null
  showAllTasks?: boolean
  // For superadmin: scopes the task query to a single company (tenant).
  companyId?: number | null
}

/**
 * Un movimiento detenido por una puerta de fase: qué pide el servidor y cómo
 * reintentarlo una vez el usuario lo rellene.
 *
 * El reintento se guarda como cierre en vez de reconstruirlo desde el modal porque
 * cada operación (mover con reorden, cambiar estado desde la ficha) tiene su propia
 * continuación, y el modal no tiene por qué conocerlas.
 */
export interface PendingGate {
  requirement: TaskGateRequirement
  retry: (form: Record<string, unknown>) => Promise<void>
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
  /** Movimiento detenido por una puerta, o null. Lo consume el modal. */
  pendingGate: PendingGate | null
  /** Reintenta el movimiento con el formulario relleno. */
  submitGate: (form: Record<string, unknown>) => Promise<void>
  /** Descarta el movimiento. Lo optimista ya se revirtió al fallar. */
  cancelGate: () => void
}

export function useTasks({ boardId, showAllTasks, companyId }: UseTasksOptions = {}): UseTasksReturn {
  const qc = useQueryClient()
  const [selectedTask, setSelectedTask] = useState<Task | null>(null)
  const [pendingGate, setPendingGate] = useState<PendingGate | null>(null)

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

  // Señal externa (una tarea asignada desde otro sitio, o una automatización que
  // acaba de tocar el tablero) → refrescar. Los dos eventos hacen lo mismo y se
  // escuchan por separado para que se lea de dónde viene cada refresco.
  useEffect(() => {
    const handler = () => { refetch() }
    window.addEventListener('task-assigned', handler)
    window.addEventListener('tasks-changed', handler)
    return () => {
      window.removeEventListener('task-assigned', handler)
      window.removeEventListener('tasks-changed', handler)
    }
  }, [refetch])

  const createTask = useCallback(async (data: CreateTaskInput): Promise<Task> => {
    const newTask = await taskService.create(data)
    await qc.invalidateQueries({ queryKey })
    return newTask
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [qc, boardId, showAllTasks, companyId])

  // Protocolo optimista compartido por updateTask/moveTask/reorderColumn:
  // cancelar queries en vuelo → snapshot → aplicar al cache → persistir → invalidate.
  // Cancelar es clave: sin esto un re-fetch en curso puede volver con datos viejos
  // del servidor y pisar el estado optimista correcto (race condition).
  // Si persist falla, se revierte el snapshot Y se invalida para resincronizar.
  const runOptimistic = useCallback(async (apply: (old: Task[]) => Task[], persist: () => Promise<void>) => {
    // Cancelar cualquier re-fetch pendiente para evitar que respuestas viejas del
    // servidor sobreescriban el estado optimista que estamos a punto de aplicar.
    await qc.cancelQueries({ queryKey })
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

  // Envuelve una operación que puede chocar contra una puerta. Si el servidor la
  // detiene, guarda el reintento y NO relanza: el movimiento ya quedó revertido por
  // runOptimistic, y lo que procede es preguntar, no mostrar un error.
  //
  // Cualquier otro fallo se relanza tal cual: una puerta es una condición de negocio,
  // no una excusa para tragarse un error de red.
  const withGate = useCallback(async (run: (form?: Record<string, unknown>) => Promise<void>) => {
    try {
      await run()
      setPendingGate(null)
    } catch (err) {
      const requirement = gateFromError(err)
      if (!requirement) throw err
      setPendingGate({ requirement, retry: (form) => run(form) })
    }
  }, [])

  const submitGate = useCallback(async (form: Record<string, unknown>) => {
    if (!pendingGate) return
    try {
      await pendingGate.retry(form)
      setPendingGate(null)
      // Cruzar una puerta puede tener consecuencia: la regla que la puso decide y
      // mueve la tarjeta segundos después, ya en el worker. El tablero que estás
      // mirando no se entera de ese movimiento —lo hizo el servidor, no tú—, así que
      // se vuelve a preguntar un par de veces. Sin esto, "la tarjeta se mueve sola"
      // sólo era cierto si recargabas, que es justo lo que la promesa evita.
      window.setTimeout(() => { refetch() }, 1500)
      window.setTimeout(() => { refetch() }, 4000)
    } catch (err) {
      // Un segundo rechazo trae los errores por campo: se refresca el requisito y el
      // modal sigue abierto, con lo escrito y las correcciones señaladas.
      const requirement = gateFromError(err)
      if (!requirement) throw err
      setPendingGate({ requirement, retry: pendingGate.retry })
    }
  }, [pendingGate, refetch])

  const cancelGate = useCallback(() => setPendingGate(null), [])

  const refreshSelectedTask = useCallback(async (id: number) => {
    try {
      const updated = await taskService.getById(id)
      setSelectedTask((prev) => {
        if (prev && prev.id === id) {
          return updated
        }
        return prev
      })
    } catch (err) {
      console.error('Error refreshing selected task:', err)
    }
  }, [])

  const updateTask = useCallback(async (id: number, data: Partial<Task>) => {
    // withGate envuelve TODO el ciclo optimista, no sólo la llamada: si la puerta
    // detiene el movimiento, runOptimistic ya revirtió la tarjeta y lo que queda es
    // preguntar. El reintento vuelve a entrar por aquí con el formulario relleno.
    await withGate((gateForm) => runOptimistic(
      // Optimistic update (skip assignees — they come as objects not IDs).
      (old) => old.map((t) => {
        if (t.id === id) {
          const { assignees, ...rest } = data
          let completed = t.completed
          if (data.status === 'finalizado') {
            completed = true
          } else if (data.status !== undefined) {
            completed = false
          }
          if (data.completed !== undefined) {
            completed = data.completed
          }
          return { ...t, ...rest, completed }
        }
        return t
      }),
      async () => {
        await taskService.update(id, data, gateForm)
        await refreshSelectedTask(id)
      },
    ))
  }, [withGate, runOptimistic, refreshSelectedTask])

  // Mueve una tarjeta a otra columna dejándola en una posición concreta:
  // actualiza status + order de forma optimista, persiste el status y después
  // el orden de la columna destino (el endpoint de reorden filtra por status,
  // así que debe correr tras el update).
  const moveTask = useCallback(async (id: number, boardId: number, newStatus: string, orderedIds: number[]) => {
    await withGate((gateForm) => runOptimistic(
      (old) => old.map((t) => {
        const idx = orderedIds.indexOf(t.id)
        if (t.id === id) {
          const completed = newStatus === 'finalizado'
          return { ...t, status: newStatus, completed, order: idx >= 0 ? idx : t.order }
        }
        return idx >= 0 ? { ...t, order: idx } : t
      }),
      async () => {
        // Si la puerta rechaza, esto lanza y el reordenamiento de abajo NO llega a
        // correr: sería colocar en una columna donde la tarjeta no entró.
        await taskService.update(id, { status: newStatus }, gateForm)
        try {
          await taskService.reorder(boardId, newStatus, orderedIds)
        } catch (reorderError) {
          console.warn('No se pudo persistir la posición exacta (la tarea sí cambió de columna):', reorderError)
        }
        await refreshSelectedTask(id)
      },
    ))
  }, [withGate, runOptimistic, refreshSelectedTask])

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
    pendingGate,
    submitGate,
    cancelGate,
  }
}
