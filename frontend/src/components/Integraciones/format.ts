import axios from 'axios'

/** Fecha y hora corta en español: «16 oct 2026, 13:05». */
export function formatDate(iso?: string | null) {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleString('es', { day: '2-digit', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' })
}

/** El mensaje del backend ({error}) o, si no hay, el de respaldo. */
export function errorMessage(err: unknown, fallback: string) {
  if (axios.isAxiosError(err)) {
    const msg = (err.response?.data as { error?: string } | undefined)?.error
    if (msg) return msg
  }
  if (err instanceof Error && err.message) return err.message
  return fallback
}
