import { useEffect, useRef, type ReactNode } from 'react'
import { X } from 'lucide-react'
import './Modal.css'

export type ModalSize = 'sm' | 'md' | 'lg' | 'xl'

export interface ModalProps {
  isOpen: boolean
  onClose: () => void
  title?: ReactNode
  children: ReactNode
  footer?: ReactNode
  size?: ModalSize
  /** Close when clicking the backdrop (default true). */
  closeOnOverlayClick?: boolean
  /** Hide the default close (X) button. */
  hideCloseButton?: boolean
  ariaLabel?: string
}

const FOCUSABLE =
  'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])'

/**
 * Accessible modal primitive: focus trap, focus restoration, Escape to close,
 * backdrop click, scroll lock and aria-modal semantics. Replaces ad-hoc overlay
 * divs scattered across pages.
 */
export function Modal({
  isOpen,
  onClose,
  title,
  children,
  footer,
  size = 'md',
  closeOnOverlayClick = true,
  hideCloseButton = false,
  ariaLabel,
}: ModalProps) {
  const panelRef = useRef<HTMLDivElement>(null)
  const previouslyFocused = useRef<HTMLElement | null>(null)

  // Keep the latest onClose without making the focus-trap effect depend on it.
  // Otherwise a parent passing an inline onClose re-runs the effect on every
  // render (e.g. each keystroke), stealing focus back to the first element.
  const onCloseRef = useRef(onClose)
  onCloseRef.current = onClose

  useEffect(() => {
    if (!isOpen) return

    previouslyFocused.current = document.activeElement as HTMLElement | null

    // Lock background scroll.
    const prevOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'

    // Move focus into the dialog (only if focus isn't already inside it).
    const focusFirst = () => {
      const panel = panelRef.current
      if (!panel || panel.contains(document.activeElement)) return
      const focusables = panel.querySelectorAll<HTMLElement>(FOCUSABLE)
      ;(focusables[0] ?? panel).focus()
    }
    const raf = requestAnimationFrame(focusFirst)

    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onCloseRef.current()
        return
      }
      if (e.key !== 'Tab') return
      const panel = panelRef.current
      if (!panel) return
      const focusables = Array.from(panel.querySelectorAll<HTMLElement>(FOCUSABLE))
      if (focusables.length === 0) {
        e.preventDefault()
        return
      }
      const first = focusables[0]
      const last = focusables[focusables.length - 1]
      const active = document.activeElement as HTMLElement
      if (e.shiftKey && active === first) {
        e.preventDefault()
        last.focus()
      } else if (!e.shiftKey && active === last) {
        e.preventDefault()
        first.focus()
      }
    }

    document.addEventListener('keydown', handleKey)
    return () => {
      cancelAnimationFrame(raf)
      document.removeEventListener('keydown', handleKey)
      document.body.style.overflow = prevOverflow
      previouslyFocused.current?.focus?.()
    }
  }, [isOpen])

  if (!isOpen) return null

  return (
    <div className="ui-modal__overlay" onClick={() => closeOnOverlayClick && onClose()}>
      <div
        ref={panelRef}
        className={`ui-modal__panel ui-modal__panel--${size}`}
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-label={typeof title === 'string' ? title : ariaLabel}
        tabIndex={-1}
      >
        {(title || !hideCloseButton) && (
          <div className="ui-modal__header">
            {title ? <h2 className="ui-modal__title">{title}</h2> : <span />}
            {!hideCloseButton && (
              <button type="button" className="ui-modal__close" onClick={onClose} aria-label="Cerrar">
                <X size={18} />
              </button>
            )}
          </div>
        )}
        <div className="ui-modal__body">{children}</div>
        {footer && <div className="ui-modal__footer">{footer}</div>}
      </div>
    </div>
  )
}
