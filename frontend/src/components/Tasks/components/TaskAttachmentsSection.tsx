import { useState, useRef } from 'react'
import { taskService } from '../../../services/api'
import { useConfirm } from '../../ui/ConfirmProvider'
import type { TaskAttachment } from '../../../types'
import {
  X,
  Paperclip,
  Image as ImageIcon,
  FileText,
  BarChart,
  Music
} from 'lucide-react'

interface TaskAttachmentsSectionProps {
  taskId: number
  attachments: TaskAttachment[]
  onAttachmentAdded: (attachment: TaskAttachment) => void
  onAttachmentDeleted: (id: number) => void
  styles: any
}

export function TaskAttachmentsSection({
  taskId,
  attachments,
  onAttachmentAdded,
  onAttachmentDeleted,
  styles
}: TaskAttachmentsSectionProps) {
  const [isUploadingFile, setIsUploadingFile] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const confirm = useConfirm()

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    setIsUploadingFile(true)
    try {
      const attachment = await taskService.addAttachment(taskId, file)
      onAttachmentAdded(attachment)
    } catch (error) {
      console.error('Error uploading file:', error)
    } finally {
      setIsUploadingFile(false)
      if (fileInputRef.current) fileInputRef.current.value = ''
    }
  }

  const handleDeleteAttachment = async (attachmentId: number) => {
    const ok = await confirm({
      title: 'Eliminar archivo',
      message: '¿Eliminar este archivo adjunto?',
      confirmLabel: 'Eliminar',
      variant: 'danger',
    })
    if (!ok) return
    try {
      await taskService.deleteAttachment(taskId, attachmentId)
      onAttachmentDeleted(attachmentId)
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

  return (
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
  )
}
