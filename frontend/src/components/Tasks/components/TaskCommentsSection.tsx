import { useState } from 'react'
import { taskService } from '../../../services/api'
import type { Task } from '../../../types'

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
  const [newComment, setNewComment] = useState('')
  const [isSubmittingComment, setIsSubmittingComment] = useState(false)

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
          comments.map((comment) => (
            <div key={comment.id} className={styles['comment-item']}>
              <div className={styles['comment-avatar']}>
                {comment.user?.name?.charAt(0).toUpperCase() || '?'}
              </div>
              <div className={styles['comment-content']}>
                <span className={styles['comment-author'] || 'comment-author'}>
                  {comment.user?.name || 'Usuario'}
                </span>
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
  )
}
