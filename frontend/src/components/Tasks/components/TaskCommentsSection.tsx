import { useState } from 'react'
import { taskService } from '../../../services/api'
import type { Task } from '../../../types'
import { useAuth } from '../../../context/AuthContext'
import { useConfirm } from '../../ui/ConfirmProvider'
import { Pencil, Trash2 } from 'lucide-react'

type TaskComment = NonNullable<Task['comments']>[number]

interface TaskCommentsSectionProps {
  taskId: number
  comments: TaskComment[]
  isLoadingComments: boolean
  refreshTask: () => Promise<void>
  styles: any
}

export function TaskCommentsSection({
  taskId,
  comments,
  isLoadingComments,
  refreshTask,
  styles
}: TaskCommentsSectionProps) {
  const { user } = useAuth()
  const isSuperadmin = user?.is_superadmin
  const confirm = useConfirm()

  const [newComment, setNewComment] = useState('')
  const [isSubmittingComment, setIsSubmittingComment] = useState(false)
  const [editingCommentId, setEditingCommentId] = useState<number | null>(null)
  const [editContent, setEditContent] = useState('')
  const [isUpdatingComment, setIsUpdatingComment] = useState(false)

  const handleAddComment = async () => {
    if (!newComment.trim()) return
    setIsSubmittingComment(true)
    try {
      await taskService.addComment(taskId, newComment)
      setNewComment('')
      await refreshTask()
    } catch (error) {
      console.error('Error adding comment:', error)
    } finally {
      setIsSubmittingComment(false)
    }
  }

  const handleEditClick = (comment: TaskComment) => {
    setEditingCommentId(comment.id)
    setEditContent(comment.content)
  }

  const handleCancelEdit = () => {
    setEditingCommentId(null)
    setEditContent('')
  }

  const handleSaveEdit = async (commentId: number) => {
    if (!editContent.trim()) return
    setIsUpdatingComment(true)
    try {
      await taskService.updateComment(taskId, commentId, editContent)
      setEditingCommentId(null)
      await refreshTask()
    } catch (error) {
      console.error('Error updating comment:', error)
    } finally {
      setIsUpdatingComment(false)
    }
  }

  const handleDeleteComment = async (commentId: number) => {
    const ok = await confirm({
      title: 'Eliminar comentario',
      message: '¿Seguro que deseas eliminar este comentario? Esta acción no se puede deshacer.',
      confirmLabel: 'Eliminar',
      variant: 'danger',
    })
    if (!ok) return

    try {
      await taskService.deleteComment(taskId, commentId)
      await refreshTask()
    } catch (error) {
      console.error('Error deleting comment:', error)
    }
  }

  return (
    <div className={styles['task-section']}>
      <h4>Comentarios ({comments.length || 0})</h4>
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
          <div
            className={styles['comments-loading'] || 'comments-loading'}
            style={{ display: 'flex', gap: '8px', alignItems: 'center', padding: '12px', color: '#64748b' }}
          >
            <div className={styles['spinner']} style={{ width: '16px', height: '16px', borderWidth: '2px' }} />
            <span style={{ fontSize: '13px' }}>Cargando comentarios...</span>
          </div>
        ) : comments.length > 0 ? (
          comments.map((comment) => {
            const isEditing = editingCommentId === comment.id
            const canEdit = isSuperadmin || user?.id === comment.user_id

            return (
              <div key={comment.id} className={styles['comment-item']}>
                <div className={styles['comment-avatar']}>
                  {comment.user?.name?.charAt(0).toUpperCase() || '?'}
                </div>
                <div className={styles['comment-content']}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                    <span className={styles['comment-author'] || 'comment-author'}>
                      {comment.user?.name || 'Usuario'}
                    </span>
                    {canEdit && !isEditing && (
                      <div style={{ display: 'flex', gap: '4px' }}>
                        <button 
                          className={styles['comment-action-btn'] || 'ot-btn-icon'} 
                          onClick={() => handleEditClick(comment)}
                          title="Editar"
                          style={{ background: 'none', border: 'none', cursor: 'pointer', color: '#64748b', padding: '2px' }}
                        >
                          <Pencil size={14} />
                        </button>
                        <button 
                          className={styles['comment-action-btn'] || 'ot-btn-icon'} 
                          onClick={() => handleDeleteComment(comment.id)}
                          title="Eliminar"
                          style={{ background: 'none', border: 'none', cursor: 'pointer', color: '#ef4444', padding: '2px' }}
                        >
                          <Trash2 size={14} />
                        </button>
                      </div>
                    )}
                  </div>
                  
                  {isEditing ? (
                    <div style={{ marginTop: '8px' }}>
                      <textarea
                        value={editContent}
                        onChange={(e) => setEditContent(e.target.value)}
                        rows={2}
                        style={{ width: '100%', padding: '8px', borderRadius: '4px', border: '1px solid #e2e8f0', fontSize: '14px', resize: 'vertical' }}
                      />
                      <div style={{ display: 'flex', gap: '8px', marginTop: '12px', justifyContent: 'flex-end' }}>
                        <button 
                          onClick={handleCancelEdit} 
                          disabled={isUpdatingComment}
                          style={{ 
                            background: 'white', 
                            border: '1px solid #e2e8f0', 
                            color: '#475569',
                            borderRadius: '6px', 
                            padding: '8px 16px', 
                            fontSize: '13px',
                            fontWeight: 500,
                            cursor: 'pointer' 
                          }}
                        >
                          Cancelar
                        </button>
                        <button 
                          className={styles['btn-add-comment']}
                          onClick={() => handleSaveEdit(comment.id)}
                          disabled={!editContent.trim() || isUpdatingComment}
                        >
                          {isUpdatingComment ? 'Guardando...' : 'Guardar'}
                        </button>
                      </div>
                    </div>
                  ) : (
                    <p>{comment.content}</p>
                  )}
                  
                  {!isEditing && (
                    <span className={styles['comment-date'] || 'comment-date'}>
                      {new Date(comment.created_at).toLocaleDateString()}
                    </span>
                  )}
                </div>
              </div>
            )
          })
        ) : (
          <span className={styles['no-data']}>No hay comentarios aún</span>
        )}
      </div>
    </div>
  )
}
