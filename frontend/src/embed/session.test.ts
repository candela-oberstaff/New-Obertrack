import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// session.ts y crmBridge.ts guardan estado de módulo (así vive el token en
// memoria), así que cada test los importa de cero.

function b64url(obj: unknown): string {
  return btoa(JSON.stringify(obj)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

const TOKEN = `${b64url({ alg: 'HS256' })}.${b64url({ tenant_id: 42, scope: 'tasks' })}.firma`

let postMessage: ReturnType<typeof vi.fn>

async function boot(url: string, crmOrigin = 'https://obersuite.oberstaff.com') {
  window.history.replaceState(null, '', url)
  vi.resetModules()
  const session = await import('./session')
  session.bootEmbedSession()
  // jsdom no carga scripts: se simula la llegada de /embed/config.js.
  const load = () => {
    window.__OBERTRACK_EMBED__ = { crmOrigin }
    const script = document.head.querySelector<HTMLScriptElement>('script[src="/embed/config.js"]')
    script?.onload?.(new Event('load'))
  }
  return { session, load }
}

beforeEach(() => {
  postMessage = vi.fn()
  Object.defineProperty(window, 'parent', { value: { postMessage }, configurable: true })
})

afterEach(() => {
  document.head.querySelectorAll('script[src="/embed/config.js"]').forEach((s) => s.remove())
  delete window.__OBERTRACK_EMBED__
  Object.defineProperty(window, 'parent', { value: window, configurable: true })
  window.history.replaceState(null, '', '/')
})

describe('bootEmbedSession', () => {
  it('toma el token del hash, lo borra de la URL y lee la empresa', async () => {
    const { session } = await boot(`/embed/tareas#s=${TOKEN}`)
    expect(session.isEmbedMode()).toBe(true)
    expect(session.getEmbedToken()).toBe(TOKEN)
    expect(session.getEmbedCompanyId()).toBe(42)
    expect(window.location.hash).toBe('')
    expect(window.location.pathname).toBe('/embed/tareas')
  })

  it('fuera de /embed/ no hace nada', async () => {
    const { session } = await boot(`/tasks#s=${TOKEN}`)
    expect(session.isEmbedMode()).toBe(false)
    expect(session.getEmbedToken()).toBeNull()
    expect(window.location.hash).toBe(`#s=${TOKEN}`)
  })

  it('sin token (F5 dentro del iframe) arranca en modo embebido y sin sesión', async () => {
    const { session } = await boot('/embed/tareas')
    expect(session.isEmbedMode()).toBe(true)
    expect(session.getEmbedToken()).toBeNull()
  })
})

describe('mensajes al CRM', () => {
  it('encola hasta tener el origen y manda ready una sola vez, con targetOrigin fijo', async () => {
    const { session, load } = await boot(`/embed/tareas#s=${TOKEN}`)
    session.notifyEmbedReady()
    expect(postMessage).not.toHaveBeenCalled()

    load()
    session.notifyEmbedReady()
    expect(postMessage).toHaveBeenCalledTimes(1)
    expect(postMessage).toHaveBeenCalledWith(
      { type: 'obertrack:ready', company_id: 42 },
      'https://obersuite.oberstaff.com',
    )
  })

  it('session_expired: una sola vez, sin token en el mensaje, y la sesión queda vacía', async () => {
    const { session, load } = await boot(`/embed/tareas#s=${TOKEN}`)
    load()
    const listener = vi.fn()
    session.onEmbedSessionExpired(listener)

    session.expireEmbedSession()
    session.expireEmbedSession()

    expect(postMessage).toHaveBeenCalledTimes(1)
    expect(postMessage).toHaveBeenCalledWith({ type: 'obertrack:session_expired' }, 'https://obersuite.oberstaff.com')
    expect(JSON.stringify(postMessage.mock.calls)).not.toContain(TOKEN)
    expect(listener).toHaveBeenCalledTimes(1)
    expect(session.getEmbedToken()).toBeNull()

    // Tras expirar, ya no se anuncia ready.
    session.notifyEmbedReady()
    expect(postMessage).toHaveBeenCalledTimes(1)
  })

  it.each([
    ['vacío', ''],
    ['comodín', '*'],
    ['con ruta', 'https://obersuite.oberstaff.com/ventas'],
    ['http fuera de localhost', 'http://obersuite.oberstaff.com'],
  ])('con un origen %s no manda nada', async (_name, origin) => {
    const { session, load } = await boot(`/embed/tareas#s=${TOKEN}`, origin)
    load()
    session.notifyEmbedReady()
    expect(postMessage).not.toHaveBeenCalled()
  })
})

describe('restoreUploadUrls', () => {
  it('fuera del modo embebido deja el HTML igual', async () => {
    await boot('/tasks')
    const { restoreUploadUrls } = await import('./authedFiles')
    const html = '<p><img src="blob:http://localhost/abc"></p>'
    expect(restoreUploadUrls(html)).toBe(html)
  })
})
