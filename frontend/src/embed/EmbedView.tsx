import { lazy, Suspense, useEffect, useState } from 'react'
import { useAuth } from '../context/AuthContext'
import { LoadingScreen } from '../routes/LoadingScreen'
import { installAuthedFiles } from './authedFiles'
import { EmbedMessage } from './EmbedMessage'
import {
  expireEmbedSession,
  getEmbedToken,
  isEmbedSessionExpired,
  onEmbedSessionExpired,
} from './session'

// Las páginas que se pueden embeber, por alcance de la sesión (contrato, §4).
const PAGES = {
  tasks: lazy(() => import('../pages/Tasks')),
  hours: lazy(() => import('../pages/WorkHours')),
}

export type EmbedScope = keyof typeof PAGES

// /embed/tareas y /embed/horas: la página sin el Layout de Obertrack (ni menú,
// ni campana, ni sockets), para el iframe del CRM. La sesión la trae el hash
// que leyó bootEmbedSession; aquí solo se decide qué pintar. Qué rutas puede
// usar cada vista lo decide el backend según el alcance del token.
export default function EmbedView({ scope }: { scope: EmbedScope }) {
  const { user, isLoading } = useAuth()
  const [expired, setExpired] = useState(isEmbedSessionExpired)
  const Page = PAGES[scope]

  useEffect(() => onEmbedSessionExpired(() => setExpired(true)), [])

  // Montada sin token (F5 dentro del iframe) o sin usuario tras preguntar: la
  // sesión no sirve y el CRM tiene que pedir otra (contrato, §6).
  useEffect(() => {
    if (!getEmbedToken() || (!isLoading && !user)) expireEmbedSession()
  }, [isLoading, user])

  // Adjuntos e imágenes de /api/uploads con la cabecera de la sesión.
  useEffect(() => installAuthedFiles(), [])

  if (expired || !getEmbedToken()) return <EmbedMessage code="session_expired" />
  if (isLoading || !user) return <LoadingScreen />

  return (
    <div style={{ minHeight: '100vh', boxSizing: 'border-box', padding: 16, background: 'var(--bg-secondary, #f8fafc)' }}>
      <Suspense fallback={<LoadingScreen />}>
        <Page />
      </Suspense>
    </div>
  )
}
