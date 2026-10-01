import { AlertTriangle, Lock } from 'lucide-react'

// Textos de los códigos del contrato CRM (§7). Los del contrato llevan {vista}
// y los pone el CRM, que sabe qué pestaña abrió; esta pantalla no lo sabe
// (el canje puede fallar antes de leer el scope), así que usa textos neutros.
const INVALID_LINK = {
  title: 'El enlace no es válido',
  text: 'El enlace no es válido. Vuelve a abrir la pestaña desde el CRM.',
}

const EMBED_MESSAGES: Record<string, { title: string; text: string; locked?: boolean }> = {
  invalid_token: INVALID_LINK,
  token_used: INVALID_LINK,
  company_not_found: {
    title: 'Empresa no disponible',
    text: 'Esta empresa no está disponible en Obertrack. Contacta a soporte.',
  },
  user_not_found: {
    title: 'Sin acceso',
    text: 'Tu usuario no tiene acceso a esta vista de la empresa.',
    locked: true,
  },
  scope_not_allowed: {
    title: 'Sin acceso a esta vista',
    text: 'Tu rol no tiene acceso a esta vista en Obertrack. Pide acceso a tu administrador.',
    locked: true,
  },
  // El código no dice cuál de los rechazos del login fue (cuenta suspendida,
  // inducción pendiente o empresa suspendida): el texto los cubre a los tres.
  access_suspended: {
    title: 'Acceso suspendido',
    text: 'Tu cuenta o el acceso de tu empresa está suspendido o en revisión. Contacta al administrador.',
    locked: true,
  },
  session_expired: {
    title: 'Tu sesión terminó',
    text: 'Tu sesión terminó. Vuelve a abrir la pestaña desde el CRM.',
  },
}

export function EmbedMessage({ code }: { code: string }) {
  const msg = EMBED_MESSAGES[code] ?? INVALID_LINK
  const Icon = msg.locked ? Lock : AlertTriangle
  return (
    <div
      role="alert"
      style={{
        minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center',
        padding: 16, boxSizing: 'border-box', background: 'var(--bg-secondary, #f8fafc)',
      }}
    >
      <div style={{ maxWidth: 420, textAlign: 'center' }}>
        <Icon size={40} style={{ display: 'block', margin: '0 auto 16px', color: '#94a3b8' }} aria-hidden />
        <h1 style={{ fontSize: 20, fontWeight: 700, color: 'var(--black, #0f172a)', margin: '0 0 8px' }}>{msg.title}</h1>
        <p style={{ fontSize: 15, color: '#64748b', margin: 0 }}>{msg.text}</p>
      </div>
    </div>
  )
}
