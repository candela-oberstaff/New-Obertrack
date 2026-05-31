import { useState, useEffect } from 'react'
import { taskService } from '../../services/api'
import type { Task, User, TaskAttachment } from '../../types'
import { ColumnType } from './types'
import styles from '../../pages/Tasks.module.css'
import { X } from 'lucide-react'
import { TaskDetailEditForm } from './components/TaskDetailEditForm'
import { TaskDetailView } from './components/TaskDetailView'

interface TaskDetailPanelProps {
  task: Task | null
  users: User[]
  onClose: () => void
  onUpdate: (id: number, data: Partial<Task>) => Promise<void>
  onDelete: (id: number) => Promise<void>
  columns: ColumnType[]
}

export function TaskDetailPanel({ task, users, onClose, onUpdate, onDelete, columns }: TaskDetailPanelProps) {
  const [isEditing, setIsEditing] = useState(false)
  const [isUpdating, setIsUpdating] = useState(false)
  const [isDeleting, setIsDeleting] = useState(false)
  const [taskComments, setTaskComments] = useState<any[]>([])
  const [attachments, setAttachments] = useState<TaskAttachment[]>([])
  const [isLoadingComments, setIsLoadingComments] = useState(false)

  const refreshTask = async () => {
    if (!task) return
    setIsLoadingComments(true)
    try {
      const updated = await taskService.getById(task.id)
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
      setIsEditing(false)
    } finally {
      setIsUpdating(false)
    }
  }

  const handleStatusChange = async (newStatus: string) => {
    await onUpdate(task.id, { status: newStatus as Task['status'] })
  }

  const handleAttachmentAdded = (newAttachment: TaskAttachment) => {
    setAttachments(prev => [...prev, newAttachment])
  }

  const handleAttachmentDeleted = (deletedId: number) => {
    setAttachments(prev => prev.filter(a => a.id !== deletedId))
  }

  const handleDeleteTask = async () => {
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
        <button className={styles['close-btn']} onClick={onClose}><X size={20} /></button>
      </div>

      <div className={styles['panel-content']}>
        {isEditing ? (
          <TaskDetailEditForm
            task={task}
            users={users}
            columns={columns}
            onCancel={() => setIsEditing(false)}
            onSave={handleSave}
            isUpdating={isUpdating}
            styles={styles}
          />
        ) : (
          <TaskDetailView
            task={task}
            columns={columns}
            attachments={attachments}
            comments={taskComments}
            isLoadingComments={isLoadingComments}
            isDeleting={isDeleting}
            styles={styles}
            onStatusChange={handleStatusChange}
            onEdit={() => setIsEditing(true)}
            onDelete={handleDeleteTask}
            refreshTask={refreshTask}
            onAttachmentAdded={handleAttachmentAdded}
            onAttachmentDeleted={handleAttachmentDeleted}
          />
        )}
      </div>
    </div>
  )
}
