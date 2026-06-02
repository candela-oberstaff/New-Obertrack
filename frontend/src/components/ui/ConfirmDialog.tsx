import { useEffect, type ReactNode } from 'react'
import { AlertTriangle, X } from 'lucide-react'
import './ConfirmDialog.css'

export type ConfirmVariant = 'danger' | 'primary'

export interface ConfirmDialogProps {
  open: boolean
  title: string
  message?: ReactNode
  confirmLabel?: string
  cancelLabel?: string
  variant?: ConfirmVariant
  loading?: boolean
  icon?: ReactNode
  onConfirm: () => void
  onCancel: () => void
}

export function ConfirmDialog({
  open,
  title,
  message,
  confirmLabel = 'Confirmar',
  cancelLabel = 'Cancelar',
  variant = 'danger',
  loading = false,
  icon,
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  // Close on Escape
  useEffect(() => {
    if (!open) return
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !loading) onCancel()
    }
    document.addEventListener('keydown', handleKey)
    return () => document.removeEventListener('keydown', handleKey)
  }, [open, loading, onCancel])

  if (!open) return null

  return (
    <div className="ui-confirm__overlay" onClick={() => !loading && onCancel()}>
      <div
        className="ui-confirm__dialog"
        onClick={(e) => e.stopPropagation()}
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="ui-confirm-title"
      >
        <button className="ui-confirm__close" onClick={onCancel} disabled={loading} aria-label="Cerrar">
          <X size={18} />
        </button>

        <div className={`ui-confirm__icon ui-confirm__icon--${variant}`}>
          {icon || <AlertTriangle size={24} />}
        </div>

        <h2 id="ui-confirm-title" className="ui-confirm__title">{title}</h2>
        {message && <div className="ui-confirm__message">{message}</div>}

        <div className="ui-confirm__actions">
          <button
            type="button"
            className="ui-confirm__btn ui-confirm__btn--cancel"
            onClick={onCancel}
            disabled={loading}
          >
            {cancelLabel}
          </button>
          <button
            type="button"
            className={`ui-confirm__btn ui-confirm__btn--${variant}`}
            onClick={onConfirm}
            disabled={loading}
            autoFocus
          >
            {loading ? 'Procesando...' : confirmLabel}
          </button>
        </div>
      </div>
    </div>
  )
}
