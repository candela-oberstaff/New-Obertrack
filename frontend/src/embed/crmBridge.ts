// Mensajes de la vista embebida al CRM (contrato CRM, §6). Solo van del iframe
// al CRM, con objetos planos ({ type, ...datos }) y nunca con un token.
//
// El origen del CRM no se compila en el bundle: lo publica nginx en
// /embed/config.js a partir de CRM_ORIGIN (una sola variable, en tiempo de
// ejecución). Sin origen válido no se envía nada: nunca se usa "*".

export type CrmMessage =
  | { type: 'obertrack:ready'; company_id: number }
  | { type: 'obertrack:error'; code: string }
  | { type: 'obertrack:session_expired' }

declare global {
  interface Window {
    __OBERTRACK_EMBED__?: { crmOrigin?: string }
  }
}

// El mismo criterio que docker-entrypoint.d/18-crm-origin.envsh: un origen a
// secas, https, o http solo en localhost.
const ORIGIN_RE = /^(https:\/\/[A-Za-z0-9.-]+|http:\/\/(localhost|127\.0\.0\.1))(:\d+)?$/

let crmOrigin: string | null = null
let configLoaded = false
// Lo que se quiso enviar antes de conocer el origen (config.js llega async).
const pending: CrmMessage[] = []

/** Carga /embed/config.js una vez. Sin origen válido, los mensajes se descartan. */
export function loadEmbedConfig(): void {
  const done = () => {
    configLoaded = true
    pending.splice(0).forEach(send)
  }
  const script = document.createElement('script')
  script.src = '/embed/config.js'
  script.onload = () => {
    const origin = window.__OBERTRACK_EMBED__?.crmOrigin ?? ''
    crmOrigin = ORIGIN_RE.test(origin) ? origin : null
    if (!crmOrigin) console.warn('[embed] CRM_ORIGIN no está configurado: no se enviarán mensajes al CRM')
    done()
  }
  script.onerror = () => {
    console.warn('[embed] no se pudo cargar /embed/config.js')
    done()
  }
  document.head.appendChild(script)
}

function send(message: CrmMessage): void {
  if (!crmOrigin || window.parent === window) return
  window.parent.postMessage(message, crmOrigin)
}

export function postToCrm(message: CrmMessage): void {
  if (!configLoaded) {
    pending.push(message)
    return
  }
  send(message)
}
