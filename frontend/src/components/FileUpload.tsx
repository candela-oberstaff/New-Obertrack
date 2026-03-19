import { useState, useRef } from 'react'
import { uploadService } from '../services/api'
import './FileUpload.css'

interface FileUploadProps {
  onUpload: (url: string, filename: string) => void
  accept?: string
  maxSize?: number
  label?: string
}

export default function FileUpload({ 
  onUpload, 
  accept = '.pdf,.doc,.docx,.xls,.xlsx,.jpg,.jpeg,.png,.gif,.webp,.mp3,.wav,.ogg,.webm',
  maxSize = 50 * 1024 * 1024,
  label = 'Adjuntar archivo'
}: FileUploadProps) {
  const [isUploading, setIsUploading] = useState(false)
  const [progress, setProgress] = useState(0)
  const [error, setError] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)

  const handleFileSelect = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return

    setError('')

    if (file.size > maxSize) {
      setError('El archivo es muy grande (máx 50MB)')
      return
    }

    setIsUploading(true)
    setProgress(0)

    try {
      const result = await uploadService.upload(file)
      onUpload(result.url, file.name)
      setProgress(100)
    } catch (err: any) {
      setError(err.response?.data?.error || 'Error al subir archivo')
    } finally {
      setIsUploading(false)
      if (inputRef.current) {
        inputRef.current.value = ''
      }
    }
  }

  return (
    <div className="file-upload">
      <input
        ref={inputRef}
        type="file"
        accept={accept}
        onChange={handleFileSelect}
        disabled={isUploading}
        id="file-upload-input"
        style={{ display: 'none' }}
      />
      
      <label 
        htmlFor="file-upload-input" 
        className={`file-upload-label ${isUploading ? 'uploading' : ''}`}
      >
        {isUploading ? (
          <>
            <span className="upload-spinner">⏳</span>
            <span>Subiendo... {progress}%</span>
          </>
        ) : (
          <>
            <span>📎</span>
            <span>{label}</span>
          </>
        )}
      </label>

      {error && <div className="file-upload-error">{error}</div>}
    </div>
  )
}

interface FileAttachmentProps {
  url: string
  filename: string
  size?: number
  onRemove?: () => void
}

export function FileAttachment({ url, filename, size, onRemove }: FileAttachmentProps) {
  const getFileIcon = (filename: string) => {
    const ext = filename.split('.').pop()?.toLowerCase()
    switch (ext) {
      case 'pdf': return '📄'
      case 'doc':
      case 'docx': return '📝'
      case 'xls':
      case 'xlsx': return '📊'
      case 'jpg':
      case 'jpeg':
      case 'png':
      case 'gif':
      case 'webp': return '🖼️'
      case 'mp3':
      case 'wav':
      case 'ogg':
      case 'webm': return '🎵'
      default: return '📎'
    }
  }

  const formatSize = (bytes?: number) => {
    if (!bytes) return ''
    if (bytes < 1024) return bytes + ' B'
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
  }

  const isImage = (filename: string) => {
    const ext = filename.split('.').pop()?.toLowerCase()
    return ['jpg', 'jpeg', 'png', 'gif', 'webp'].includes(ext || '')
  }

  const handleClick = () => {
    window.open(url, '_blank')
  }

  return (
    <div className="file-attachment">
      {isImage(filename) ? (
        <img src={url} alt={filename} className="file-attachment-image" onClick={handleClick} />
      ) : (
        <div className="file-attachment-icon" onClick={handleClick}>
          {getFileIcon(filename)}
        </div>
      )}
      <div className="file-attachment-info" onClick={handleClick}>
        <span className="file-attachment-name">{filename}</span>
        {size && <span className="file-attachment-size">{formatSize(size)}</span>}
      </div>
      {onRemove && (
        <button className="file-attachment-remove" onClick={onRemove}>×</button>
      )}
    </div>
  )
}

interface FileAttachmentsProps {
  attachments: { url: string; filename: string; size?: number }[]
  onRemove?: (index: number) => void
}

export function FileAttachments({ attachments, onRemove }: FileAttachmentsProps) {
  if (attachments.length === 0) return null

  return (
    <div className="file-attachments">
      {attachments.map((att, index) => (
        <FileAttachment
          key={index}
          url={att.url}
          filename={att.filename}
          size={att.size}
          onRemove={onRemove ? () => onRemove(index) : undefined}
        />
      ))}
    </div>
  )
}
