import { loadEmbedConfig, postToCrm } from './crmBridge'

// Sesión de la vista embebida en el CRM (contrato CRM, §4 y §6).
//
// El canje redirige a /embed/tareas#s=<access_token>. El token se lee UNA vez,
// antes de montar React, y vive solo en esta variable de módulo: nada de
// cookies ni de almacenamiento. Por eso un F5 dentro del iframe arranca sin
// token, y eso se comunica al CRM como session_expired para que pida otro.

const EMBED_PREFIX = '/embed/'

let embedMode = false
let token: string | null = null
let companyId: number | null = null
let scope: string | null = null
let expired = false
let readySent = false
const expiredListeners = new Set<() => void>()

export function isEmbedMode(): boolean {
  return embedMode
}

export function getEmbedToken(): string | null {
  return token
}

/** Empresa de la sesión (el tenant forzado por el canje). */
export function getEmbedCompanyId(): number | null {
  return companyId
}

/** Alcance de la sesión (la vista que pidió el CRM): "tasks" o "hours". */
export function getEmbedScope(): string | null {
  return scope
}

/**
 * Se llama en main.tsx antes de montar React: si la URL es de la vista
 * embebida, activa el modo, toma el token del hash y lo borra de la URL.
 */
export function bootEmbedSession(): void {
  const { pathname, search, hash } = window.location
  if (!pathname.startsWith(EMBED_PREFIX)) return
  embedMode = true
  installMemoryStorageIfBlocked()
  loadEmbedConfig()

  if (hash) {
    const s = new URLSearchParams(hash.slice(1)).get('s')
    if (s) {
      token = s
      const claims = readClaims(s)
      companyId = typeof claims.tenant_id === 'number' ? claims.tenant_id : null
      scope = typeof claims.scope === 'string' ? claims.scope : null
    }
    // El hash se limpia siempre: el token no debe quedar en el historial.
    history.replaceState(history.state, '', pathname + search)
  }
}

/** La sesión terminó (401 o montaje sin token). Avisa al CRM una sola vez. */
export function expireEmbedSession(): void {
  if (expired) return
  expired = true
  token = null
  postToCrm({ type: 'obertrack:session_expired' })
  expiredListeners.forEach((fn) => fn())
}

export function isEmbedSessionExpired(): boolean {
  return expired
}

export function onEmbedSessionExpired(fn: () => void): () => void {
  expiredListeners.add(fn)
  return () => { expiredListeners.delete(fn) }
}

/** La vista se pintó: se avisa al CRM una sola vez (contrato, §6). */
export function notifyEmbedReady(): void {
  if (!embedMode || readySent || expired || companyId === null || scope === null) return
  readySent = true
  postToCrm({ type: 'obertrack:ready', company_id: companyId, scope })
}

// El payload se lee sin verificar: solo para anunciar en "ready" la empresa y
// la vista, que el CRM compara con lo que pidió. Quien decide el acceso es el
// backend, que sí verifica la firma.
function readClaims(jwt: string): { tenant_id?: unknown; scope?: unknown } {
  try {
    const part = jwt.split('.')[1] ?? ''
    return JSON.parse(atob(part.replace(/-/g, '+').replace(/_/g, '/')))
  } catch {
    return {}
  }
}

// Dentro de un iframe de terceros, Chrome con las cookies de terceros
// bloqueadas hace que tocar localStorage LANCE una excepción. La vista de
// Tareas lo usa para recordar preferencias (vista, filtros), así que en ese
// caso se sustituye por uno en memoria: las preferencias no se recuerdan,
// pero la vista funciona.
function installMemoryStorageIfBlocked(): void {
  try {
    const probe = '__obertrack_probe__'
    window.localStorage.setItem(probe, '1')
    window.localStorage.removeItem(probe)
  } catch {
    const data = new Map<string, string>()
    const memory: Storage = {
      get length() { return data.size },
      clear: () => data.clear(),
      getItem: (k) => (data.has(k) ? data.get(k)! : null),
      key: (i) => Array.from(data.keys())[i] ?? null,
      removeItem: (k) => { data.delete(k) },
      setItem: (k, v) => { data.set(k, String(v)) },
    }
    try {
      Object.defineProperty(window, 'localStorage', { value: memory, configurable: true })
    } catch {
      console.warn('[embed] localStorage bloqueado y no se pudo sustituir')
    }
  }
}
