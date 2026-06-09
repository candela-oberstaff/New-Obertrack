import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from 'react'
import { Loader2 } from 'lucide-react'
import './Button.css'

export type ButtonVariant = 'primary' | 'secondary' | 'danger' | 'ghost'
export type ButtonSize = 'sm' | 'md'

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  size?: ButtonSize
  /** Shows a spinner and disables the button. */
  loading?: boolean
  leftIcon?: ReactNode
  rightIcon?: ReactNode
  fullWidth?: boolean
}

/**
 * Shared button primitive. Replaces ad-hoc inline-styled buttons so every CTA
 * shares the same brand styling, sizes and loading/disabled behaviour.
 */
export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { variant = 'primary', size = 'md', loading = false, leftIcon, rightIcon, fullWidth, className = '', disabled, children, type = 'button', ...rest },
  ref,
) {
  const classes = [
    'ui-btn',
    `ui-btn--${variant}`,
    `ui-btn--${size}`,
    fullWidth ? 'ui-btn--full' : '',
    loading ? 'ui-btn--loading' : '',
    className,
  ].filter(Boolean).join(' ')

  return (
    <button ref={ref} type={type} className={classes} disabled={disabled || loading} aria-busy={loading} {...rest}>
      {loading ? (
        <Loader2 size={size === 'sm' ? 14 : 16} className="ui-btn__spinner" aria-hidden />
      ) : (
        leftIcon && <span className="ui-btn__icon" aria-hidden>{leftIcon}</span>
      )}
      {children && <span className="ui-btn__label">{children}</span>}
      {!loading && rightIcon && <span className="ui-btn__icon" aria-hidden>{rightIcon}</span>}
    </button>
  )
})
