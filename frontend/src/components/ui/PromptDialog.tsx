import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Pencil, X } from 'lucide-react'
import './ConfirmDialog.css'

export interface PromptDialogProps {
  open: boolean
  title: string
  message?: ReactNode
  placeholder?: string
  initialValue?: string
  confirmLabel?: string
  cancelLabel?: string
  loading?: boolean
  icon?: ReactNode
  onConfirm: (value: string) => void
  onCancel: () => void
}

export function PromptDialog({
  open,
  title,
  message,
  placeholder,
  initialValue = '',
  confirmLabel = 'Guardar',
  cancelLabel = 'Cancelar',
  loading = false,
  icon,
  onConfirm,
  onCancel,
}: PromptDialogProps) {
  const [value, setValue] = useState(initialValue)
  const inputRef = useRef<HTMLInputElement>(null)

  // Reset the field each time the dialog opens and focus it (selecting the text
  // so renaming feels like an inline edit).
  useEffect(() => {
    if (open) {
      setValue(initialValue)
      requestAnimationFrame(() => {
        inputRef.current?.focus()
        inputRef.current?.select()
      })
    }
  }, [open, initialValue])

  useEffect(() => {
    if (!open) return
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !loading) onCancel()
    }
    document.addEventListener('keydown', handleKey)
    return () => document.removeEventListener('keydown', handleKey)
  }, [open, loading, onCancel])

  if (!open) return null

  const submit = () => {
    if (loading) return
    onConfirm(value)
  }

  return (
    <div className="ui-confirm__overlay" onClick={() => !loading && onCancel()}>
      <form
        className="ui-confirm__dialog"
        onClick={(e) => e.stopPropagation()}
        onSubmit={(e) => {
          e.preventDefault()
          submit()
        }}
        role="dialog"
        aria-modal="true"
        aria-labelledby="ui-prompt-title"
      >
        <button
          type="button"
          className="ui-confirm__close"
          onClick={onCancel}
          disabled={loading}
          aria-label="Cerrar"
        >
          <X size={18} />
        </button>

        <div className="ui-confirm__icon ui-confirm__icon--primary">
          {icon || <Pencil size={24} />}
        </div>

        <h2 id="ui-prompt-title" className="ui-confirm__title">{title}</h2>
        {message && <div className="ui-confirm__message">{message}</div>}

        <input
          ref={inputRef}
          className="ui-confirm__input"
          value={value}
          placeholder={placeholder}
          disabled={loading}
          onChange={(e) => setValue(e.target.value)}
        />

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
            type="submit"
            className="ui-confirm__btn ui-confirm__btn--primary"
            disabled={loading || !value.trim()}
          >
            {loading ? 'Procesando...' : confirmLabel}
          </button>
        </div>
      </form>
    </div>
  )
}
