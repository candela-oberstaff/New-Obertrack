import { useState, useEffect, useRef } from 'react'
import { taskService } from '../../services/api'
import type { Task, User, TaskStatus, TaskPriority, TaskAttachment } from '../../types'
import { RichTextEditor } from './RichTextEditor'
import { ColumnType } from './types'
import styles from '../../pages/Tasks.module.css'
import { 
  X, 
  Pencil, 
  Trash2, 
  Paperclip, 
  Image as ImageIcon, 
  FileText, 
  BarChart, 
  Music, 
  Check 
} from 'lucide-react'

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
  const [newComment, setNewComment] = useState('')
  const [isSubmittingComment, setIsSubmittingComment] = useState(false)
  const [isUpdating, setIsUpdating] = useState(false)
  const [assigneeSearch, setAssigneeSearch] = useState('')
  const [isDeleting, setIsDeleting] = useState(false)
  const [taskComments, setTaskComments] = useState(task?.comments || [])
  const [attachments, setAttachments] = useState<TaskAttachment[]>([])
  const [isUploadingFile, setIsUploadingFile] = useState(false)
  const [isLoadingComments, setIsLoadingComments] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [formData, setFormData] = useState({
    title: '',
    description: '',
    priority: 'medium',
    status: 'por_hacer',
    start_date: '',
    end_date: '',
    assignees: [] as number[],
  })

  useEffect(() => {
    if (task) {
      setFormData({
        title: task.title,
        description: task.description || '',
        priority: task.priority,
        status: task.status,
        start_date: task.start_date?.split('T')[0] || '',
        end_date: task.end_date?.split('T')[0] || '',
        assignees: task.assignees?.map(a => a.id) || [],
      })
      setTaskComments(task.comments || [])
      setAttachments((task as any).attachments || [])
    }
  }, [task])

  const refreshTask = async () => {
    setIsLoadingComments(true)
    try {
      const updated = await taskService.getById(task!.id)
      setTaskComments(updated.comments || [])
      setAttachments((updated as any).attachments || [])
    } catch (error) {
      console.error('Error refreshing task:', error)
    } finally {
      setIsLoadingComments(false)
    }
  }

  useEffect(() => {
    if (task?.id) {
      refreshTask()
    }
  }, [task?.id])

  if (!task) return null

  const handleAddComment = async () => {
    if (!newComment.trim()) return
    setIsSubmittingComment(true)
    try {
      await taskService.addComment(task.id, newComment)
      setNewComment('')
      await refreshTask()
    } catch (error) {
      console.error('Error adding comment:', error)
    } finally {
      setIsSubmittingComment(false)
    }
  }

  const handleSave = async () => {
    setIsUpdating(true)
    try {
      await onUpdate(task.id, {
        title: formData.title,
        description: formData.description,
        priority: formData.priority as TaskPriority,
        status: formData.status as TaskStatus,
        start_date: formData.start_date || undefined,
        end_date: formData.end_date || undefined,
        assignees: formData.assignees as any,
      })
      setIsEditing(false)
    } finally {
      setIsUpdating(false)
    }
  }

  const handleStatusChange = async (newStatus: string) => {
    await onUpdate(task.id, { status: newStatus as Task['status'] })
  }

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    setIsUploadingFile(true)
    try {
      const attachment = await taskService.addAttachment(task.id, file)
      setAttachments(prev => [...prev, attachment])
    } catch (error) {
      console.error('Error uploading file:', error)
    } finally {
      setIsUploadingFile(false)
      if (fileInputRef.current) fileInputRef.current.value = ''
    }
  }

  const handleDeleteAttachment = async (attachmentId: number) => {
    if (!confirm('¿Eliminar este archivo?')) return
    try {
      await taskService.deleteAttachment(task.id, attachmentId)
      setAttachments(prev => prev.filter(a => a.id !== attachmentId))
    } catch (error) {
      console.error('Error deleting attachment:', error)
    }
  }

  const formatFileSize = (bytes: number) => {
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  }

  const getFileIcon = (mimeType: string) => {
    if (mimeType?.startsWith('image/')) return <ImageIcon size={18} />
    if (mimeType?.includes('pdf')) return <FileText size={18} />
    if (mimeType?.includes('word')) return <FileText size={18} />
    if (mimeType?.includes('excel') || mimeType?.includes('sheet')) return <BarChart size={18} />
    if (mimeType?.startsWith('audio/')) return <Music size={18} />
    return <Paperclip size={18} />
  }

  const getPriorityColor = (priority: string) => {
    const colors: Record<string, string> = {
      urgent: '#ef4444',
      high: '#f97316',
      medium: '#eab308',
      low: '#22c55e',
    }
    return colors[priority] || '#6b7280'
  }
  return (
    <div className={styles['task-detail-panel']}>
      <div className={styles['panel-header']}>
        <h2>{isEditing ? 'Editar Tarea' : 'Detalles de la tarea'}</h2>
        <button className={styles['close-btn']} onClick={onClose}><X size={20} /></button>
      </div>

      <div className={styles['panel-content']}>
        {isEditing ? (
          <div className={styles['edit-form']}>
            <div className={styles['form-group']}>
              <label>Título</label>
              <input
                type="text"
                value={formData.title}
                onChange={(e) => setFormData({ ...formData, title: e.target.value })}
              />
            </div>
            <div className={styles['form-group']}>
              <label>Descripción</label>
              <RichTextEditor
                value={formData.description}
                onChange={(value) => setFormData({ ...formData, description: value })}
              />
            </div>
            <div className={styles['form-row']}>
              <div className={styles['form-group']}>
                <label>Prioridad</label>
                <select
                  value={formData.priority}
                  onChange={(e) => setFormData({ ...formData, priority: e.target.value })}
                >
                  <option value="low">Baja</option>
                  <option value="medium">Media</option>
                  <option value="high">Alta</option>
                  <option value="urgent">Urgente</option>
                </select>
              </div>
              <div className={styles['form-group']}>
                <label>Estado</label>
                <select
                  value={formData.status}
                  onChange={(e) => setFormData({ ...formData, status: e.target.value })}
                >
                  {columns.map(col => (
                    <option key={col.id} value={col.id}>{col.title}</option>
                  ))}
                </select>
              </div>
            </div>
            <div className={styles['form-row']}>
              <div className={styles['form-group']}>
                <label>Fecha inicio</label>
                <input
                  type="date"
                  value={formData.start_date}
                  onChange={(e) => setFormData({ ...formData, start_date: e.target.value })}
                />
              </div>
              <div className={styles['form-group']}>
                <label>Fecha límite</label>
                <input
                  type="date"
                  value={formData.end_date}
                  onChange={(e) => setFormData({ ...formData, end_date: e.target.value })}
                />
              </div>
            </div>
            <div className={styles['form-group']}>
              <label>Asignados</label>
              <div className={styles['assignees-edit-list'] || 'assignees-edit-list'}>
                {formData.assignees.length > 0 && (
                  <div className={styles['assigned-chips'] || 'assigned-chips'}>
                    {formData.assignees.map((id) => {
                      const user = users.find(u => u.id === id)
                      if (!user) return null
                      return (
                        <div key={user.id} className={styles['assignee-chip'] || 'assignee-chip'}>
                          <span>{user.name || user.email}</span>
                          <button
                            type="button"
                            className={styles['remove-assignee'] || 'remove-assignee'}
                            onClick={() => setFormData({
                              ...formData,
                              assignees: formData.assignees.filter(aid => aid !== user.id)
                            })}
                          >
                            <X size={14} />
                          </button>
                        </div>
                      )
                    })}
                  </div>
                )}
                <div className={styles['assignee-dropdown-wrapper'] || 'assignee-dropdown-wrapper'}>
                  <input
                    type="text"
                    className={styles['assignee-search'] || 'assignee-search'}
                    placeholder="Buscar usuario..."
                    value={assigneeSearch}
                    onChange={(e) => setAssigneeSearch(e.target.value)}
                  />
                  <div className={styles['assignee-dropdown'] || 'assignee-dropdown'}>
                    {users
                      .filter(u =>
                        u.name?.toLowerCase().includes(assigneeSearch.toLowerCase()) ||
                        u.email?.toLowerCase().includes(assigneeSearch.toLowerCase())
                      )
                      .sort((a, b) => (a.name || a.email).localeCompare(b.name || b.email))
                      .map(user => {
                        const isAssigned = formData.assignees.includes(user.id)
                        return (
                          <div
                            key={user.id}
                            className={`${styles['assignee-option'] || 'assignee-option'} ${isAssigned ? (styles['assigned'] || 'assigned') : ''}`}
                            onClick={() => {
                              if (isAssigned) {
                                setFormData({
                                  ...formData,
                                  assignees: formData.assignees.filter(aid => aid !== user.id)
                                })
                              } else {
                                setFormData({
                                  ...formData,
                                  assignees: [...formData.assignees, user.id]
                                })
                              }
                            }}
                          >
                            <div className={styles['chip-avatar'] || 'chip-avatar'}>{user.name?.charAt(0).toUpperCase() || '?'}</div>
                            <span className={styles['assignee-name'] || 'assignee-name'}>{user.name || user.email}</span>
                            {isAssigned && <span className={styles['assignee-check'] || 'assignee-check'}><Check size={14} /></span>}
                          </div>
                        )
                      })}
                  </div>
                </div>
              </div>
            </div>
            <div className={styles['form-actions']}>
              <button onClick={() => setIsEditing(false)} disabled={isUpdating}>Cancelar</button>
              <button className={styles['btn-primary']} onClick={handleSave} disabled={isUpdating}>
                {isUpdating ? 'Guardando...' : 'Guardar'}
              </button>
            </div>
          </div>
        ) : (
          <>
            <div className={styles['task-status-bar']}>
              <select
                value={task.status}
                onChange={(e) => handleStatusChange(e.target.value)}
                className={styles['status-select']}
              >
                {columns.map((col) => (
                  <option key={col.id} value={col.id}>{col.title}</option>
                ))}
              </select>
              <span
                className={styles['priority-badge']}
                style={{ backgroundColor: getPriorityColor(task.priority) }}
              >
                {task.priority}
              </span>
            </div>

            <h3 className={styles['task-title']}>{task.title}</h3>

            <div className={styles['task-section']}>
              <h4>Descripción</h4>
              {task.description ? (
                <div className={styles['task-description-html']} dangerouslySetInnerHTML={{ __html: task.description }} />
              ) : (
                <p>Sin descripción</p>
              )}
            </div>

            <div className={styles['task-dates-row']}>
              {task.start_date && (
                <div className={styles['date-item']}>
                  <span className={styles['date-label']}>Inicio</span>
                  <span>{new Date(task.start_date).toLocaleDateString('es-ES', { weekday: 'short', day: 'numeric', month: 'short' })}</span>
                </div>
              )}
              {task.end_date && (
                <div className={styles['date-item']}>
                  <span className={styles['date-label']}>Fin</span>
                  <span>{new Date(task.end_date).toLocaleDateString('es-ES', { weekday: 'short', day: 'numeric', month: 'short' })}</span>
                </div>
              )}
            </div>

            <div className={styles['task-section']}>
              <h4>Asignados</h4>
              <div className={styles['assignees-list']}>
                {task.assignees && task.assignees.length > 0 ? (
                  task.assignees.map((user) => (
                    <div key={user.id} className={styles['assignee-item']}>
                      <span>{user.name}</span>
                    </div>
                  ))
                ) : (
                  <span className={styles['no-data']}>Sin asignar</span>
                )}
              </div>
            </div>

            {/* Attachments Section */}
            <div className={styles['task-section']}>
              <h4>Archivos adjuntos ({attachments.length})</h4>
              {attachments.length > 0 && (
                <div className={styles['attachments-list']}>
                  {attachments.map((att) => (
                    <div key={att.id} className={styles['attachment-item']}>
                      <a 
                        href={att.file_url} 
                        target="_blank" 
                        rel="noopener noreferrer" 
                        className={styles['attachment-link-wrapper']}
                        title={`Ver ${att.file_name}`}
                      />
                      <div className={styles['attachment-icon']}>
                        {getFileIcon(att.mime_type)}
                      </div>
                      <div className={styles['attachment-info']}>
                        <span className={styles['attachment-name']}>{att.file_name}</span>
                        <span className={styles['attachment-meta']}>{formatFileSize(att.file_size)}</span>
                      </div>
                      <div className={styles['attachment-actions']} style={{ position: 'relative', zIndex: 2 }}>
                        <button
                          className={styles['btn-delete-att']}
                          onClick={(e) => {
                            e.preventDefault()
                            e.stopPropagation()
                            handleDeleteAttachment(att.id)
                          }}
                          title="Eliminar archivo"
                        >
                          <X size={18} />
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
              <input
                ref={fileInputRef}
                type="file"
                id="task-file-upload"
                style={{ display: 'none' }}
                accept=".pdf,.doc,.docx,.xls,.xlsx,.jpg,.jpeg,.png,.gif,.webp,.mp3,.wav,.ogg"
                onChange={handleFileUpload}
              />
              <label
                htmlFor="task-file-upload"
                style={{
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: '6px',
                  padding: '8px 14px',
                  border: '1px dashed #cbd5e1',
                  borderRadius: '8px',
                  cursor: isUploadingFile ? 'not-allowed' : 'pointer',
                  fontSize: '13px',
                  color: '#64748b',
                  background: 'white',
                  opacity: isUploadingFile ? 0.6 : 1,
                  transition: 'all 0.2s',
                }}
              >
                {isUploadingFile ? '⏳ Subiendo...' : <><Paperclip size={14} /> Adjuntar archivo</>}
              </label>
            </div>

            <div className={styles['task-section']}>
              <h4>Comentarios ({taskComments.length || 0})</h4>
              <div className={styles['add-comment']}>
                <textarea
                  placeholder="Añadir un comentario..."
                  value={newComment}
                  onChange={(e) => setNewComment(e.target.value)}
                  rows={2}
                />
                <button
                  className={styles['btn-add-comment']}
                  onClick={handleAddComment}
                  disabled={!newComment.trim() || isSubmittingComment}
                >
                  {isSubmittingComment ? 'Publicando...' : 'Publicar'}
                </button>
              </div>
              <div className={styles['comments-section']}>
                {isLoadingComments ? (
                  <div className={styles['comments-loading'] || 'comments-loading'} style={{ display: 'flex', gap: '8px', alignItems: 'center', padding: '12px', color: '#64748b' }}>
                    <div className={styles['spinner']} style={{ width: '16px', height: '16px', borderWidth: '2px' }} />
                    <span style={{ fontSize: '13px' }}>Cargando comentarios...</span>
                  </div>
                ) : taskComments.length > 0 ? (
                  taskComments.map((comment) => (
                    <div key={comment.id} className={styles['comment-item']}>
                      <div className={styles['comment-avatar']}>
                        {comment.user?.name?.charAt(0).toUpperCase() || '?'}
                      </div>
                      <div className={styles['comment-content']}>
                        <span className={styles['comment-author'] || 'comment-author'}>{comment.user?.name || 'Usuario'}</span>
                        <p>{comment.content}</p>
                        <span className={styles['comment-date'] || 'comment-date'}>
                          {new Date(comment.created_at).toLocaleDateString()}
                        </span>
                      </div>
                    </div>
                  ))
                ) : (
                  <span className={styles['no-data']}>No hay comentarios aún</span>
                )}
              </div>
            </div>

            <div className={styles['panel-actions']}>
              <button className={styles['btn-edit']} onClick={() => setIsEditing(true)}>
                <Pencil size={16} /> Editar
              </button>
              <button className={styles['btn-delete']} onClick={() => {
                if (confirm('¿Eliminar esta tarea?')) {
                  setIsDeleting(true)
                  onDelete(task.id)
                  onClose()
                }
              }} disabled={isDeleting}>
                {isDeleting ? 'Eliminando...' : <><Trash2 size={16} /> Eliminar</>}
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
