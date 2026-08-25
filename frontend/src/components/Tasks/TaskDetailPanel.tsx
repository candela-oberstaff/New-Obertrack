import { useState, useEffect } from 'react'
import { taskService } from '../../services/api'
import type { Task, User, TaskAttachment } from '../../types'
import { ColumnType } from './types'
import styles from '../../pages/Tasks.module.css'
import { X } from 'lucide-react'
import { TaskDetailEditForm } from './components/TaskDetailEditForm'
import { TaskDetailView } from './components/TaskDetailView'
import { TaskAttachmentsSection } from './components/TaskAttachmentsSection'
import { TaskCommentsSection } from './components/TaskCommentsSection'
import { useCloseGuard } from '../ui/useCloseGuard'
import { useConfirm } from '../ui/ConfirmProvider'
import { Pencil, Trash2 } from 'lucide-react'

interface TaskDetailPanelProps {
  task: Task | null
  users: User[]
  onClose: () => void
  onUpdate: (id: number, data: Partial<Task>) => Promise<void>
  onDelete: (id: number) => Promise<void>
  columns: ColumnType[]
}

export function TaskDetailPanel({ task, users, onClose, onUpdate, onDelete, columns }: TaskDetailPanelProps) {
  const confirm = useConfirm()
  const [isEditing, setIsEditing] = useState(false)
  const [isUpdating, setIsUpdating] = useState(false)
  const [isDeleting, setIsDeleting] = useState(false)
  const [fullTask, setFullTask] = useState<Task>(task!)
  const [taskComments, setTaskComments] = useState<any[]>([])
  const [attachments, setAttachments] = useState<TaskAttachment[]>([])
  const [isLoadingComments, setIsLoadingComments] = useState(false)

  // El panel se cierra solo con la X, pero hacerlo en modo edición tira el
  // formulario. Se pregunta mientras se está editando: el detalle de qué campos
  // cambiaron vive dentro del formulario y no llega hasta aquí.
  const requestClose = useCloseGuard(() => isEditing, onClose, {
    title: '¿Salir sin guardar la tarea?',
    message: 'Estás editando la tarea. Si cierras ahora se perderán los cambios.',
    confirmLabel: 'Salir sin guardar',
  })

  const refreshTask = async () => {
    if (!task) return
    setIsLoadingComments(true)
    try {
      const updated = await taskService.getById(task.id)
      setFullTask(updated)
      setTaskComments(updated.comments || [])
      setAttachments((updated as any).attachments || [])
    } catch (error) {
      console.error('Error refreshing task:', error)
    } finally {
      setIsLoadingComments(false)
    }
  }

  useEffect(() => {
    if (task) {
      setFullTask(task)
      setTaskComments(task.comments || [])
      setAttachments((task as any).attachments || [])
      setIsEditing(false)
      refreshTask()
    }
  }, [task?.id])

  if (!task) return null

  const handleSave = async (formData: any) => {
    setIsUpdating(true)
    try {
      await onUpdate(task.id, formData)
      onClose()
    } finally {
      setIsUpdating(false)
    }
  }

  const handleStatusChange = async (newStatus: string) => {
    const doneCol = columns.find(c => c.id === 'finalizado' || c.title.toLowerCase().includes('finaliz'))
    const doneId = doneCol ? doneCol.id : 'finalizado'
    if (newStatus === doneId) {
      onClose()
    }
    await onUpdate(task.id, { status: newStatus as Task['status'] })
  }

  const handleAttachmentAdded = (newAttachment: TaskAttachment) => {
    setAttachments(prev => [...prev, newAttachment])
  }

  const handleAttachmentDeleted = (deletedId: number) => {
    setAttachments(prev => prev.filter(a => a.id !== deletedId))
  }

  const handleDeleteTask = async () => {
    const ok = await confirm({
      title: 'Eliminar tarea',
      message: '¿Seguro que deseas eliminar esta tarea? Esta acción no se puede deshacer.',
      confirmLabel: 'Eliminar',
      variant: 'danger',
    })
    if (!ok) return

    setIsDeleting(true)
    try {
      await onDelete(task.id)
      onClose()
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <div className={styles['task-detail-panel']}>
      <div className={styles['panel-header']}>
        <h2>{isEditing ? 'Editar Tarea' : 'Detalles de la tarea'}</h2>
        <button className={styles['close-btn']} onClick={requestClose}><X size={20} /></button>
      </div>

      <div className={styles['panel-content']}>
        {isEditing ? (
          <TaskDetailEditForm
            task={fullTask}
            users={users}
            columns={columns}
            onCancel={() => setIsEditing(false)}
            onSave={handleSave}
            isUpdating={isUpdating}
            styles={styles}
          >
            <TaskAttachmentsSection
              taskId={fullTask.id}
              attachments={attachments}
              onAttachmentAdded={handleAttachmentAdded}
              onAttachmentDeleted={handleAttachmentDeleted}
              styles={styles}
            />
            <TaskCommentsSection
              taskId={fullTask.id}
              comments={taskComments}
              isLoadingComments={isLoadingComments}
              refreshTask={refreshTask}
              styles={styles}
            />
          </TaskDetailEditForm>
        ) : (
          <TaskDetailView
            task={fullTask}
            columns={columns}
            styles={styles}
            onStatusChange={handleStatusChange}
          >
            <TaskAttachmentsSection
              taskId={fullTask.id}
              attachments={attachments}
              onAttachmentAdded={handleAttachmentAdded}
              onAttachmentDeleted={handleAttachmentDeleted}
              styles={styles}
            />
            <TaskCommentsSection
              taskId={fullTask.id}
              comments={taskComments}
              isLoadingComments={isLoadingComments}
              refreshTask={refreshTask}
              styles={styles}
            />
          </TaskDetailView>
        )}

        {!isEditing && (
          <div className={styles['panel-actions']}>
            <button className={styles['btn-edit']} onClick={() => setIsEditing(true)}>
              <Pencil size={16} /> Editar
            </button>
            <button className={styles['btn-delete']} onClick={handleDeleteTask} disabled={isDeleting}>
              {isDeleting ? 'Eliminando...' : <><Trash2 size={16} /> Eliminar</>}
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
