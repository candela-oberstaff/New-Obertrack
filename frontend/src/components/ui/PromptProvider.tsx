import { createContext, useContext, useState, useCallback, useRef, type ReactNode } from 'react'
import { PromptDialog } from './PromptDialog'

export interface PromptOptions {
  title: string
  message?: ReactNode
  placeholder?: string
  initialValue?: string
  confirmLabel?: string
  cancelLabel?: string
  icon?: ReactNode
}

// Resolves to the entered string, or null if the user cancelled.
type PromptFn = (options: PromptOptions) => Promise<string | null>

const PromptContext = createContext<PromptFn | null>(null)

interface DialogState extends PromptOptions {
  open: boolean
}

export function PromptProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<DialogState>({ open: false, title: '' })
  const resolverRef = useRef<((value: string | null) => void) | null>(null)

  const prompt = useCallback<PromptFn>((options) => {
    return new Promise<string | null>((resolve) => {
      resolverRef.current = resolve
      setState({ ...options, open: true })
    })
  }, [])

  const close = useCallback((result: string | null) => {
    resolverRef.current?.(result)
    resolverRef.current = null
    setState((prev) => ({ ...prev, open: false }))
  }, [])

  return (
    <PromptContext.Provider value={prompt}>
      {children}
      <PromptDialog
        open={state.open}
        title={state.title}
        message={state.message}
        placeholder={state.placeholder}
        initialValue={state.initialValue}
        confirmLabel={state.confirmLabel}
        cancelLabel={state.cancelLabel}
        icon={state.icon}
        onConfirm={(value) => close(value)}
        onCancel={() => close(null)}
      />
    </PromptContext.Provider>
  )
}

/**
 * Imperative text-input dialog. Drop-in replacement for window.prompt():
 *
 *   const prompt = usePrompt()
 *   const name = await prompt({ title: 'Renombrar', initialValue: board.name })
 *   if (name) { ... }
 */
export function usePrompt(): PromptFn {
  const ctx = useContext(PromptContext)
  if (!ctx) {
    throw new Error('usePrompt debe usarse dentro de <PromptProvider>')
  }
  return ctx
}
