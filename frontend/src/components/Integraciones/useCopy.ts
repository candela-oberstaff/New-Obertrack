import { useState } from 'react'
import { useNotification } from '../../context/NotificationContext'

/** Copia al portapapeles y recuerda unos segundos qué se copió (para el ✓). */
export function useCopy() {
  const [copied, setCopied] = useState<string | null>(null)
  const { error: showError } = useNotification()
  const copy = async (text: string, key: string) => {
    try {
      await navigator.clipboard.writeText(text)
      setCopied(key)
      window.setTimeout(() => setCopied((c) => (c === key ? null : c)), 1800)
    } catch {
      showError('No se pudo copiar. Selecciona el texto y cópialo a mano.')
    }
  }
  return { copied, copy }
}
