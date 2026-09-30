import api from '../services/client'
import { isEmbedMode } from './session'

// Archivos subidos dentro de la vista embebida.
//
// /api/uploads/:filename exige sesión, y en el iframe la sesión viaja solo en
// la cabecera Authorization: un <img src> o un <a href> no la llevan. Aquí se
// piden con la cabecera y se sirven como blob:, sin tocar los componentes de
// Tareas:
//   - las <img> se reescriben al entrar en el DOM (MutationObserver);
//   - los clics en enlaces a /api/uploads/ se interceptan y abren el blob.
//
// El editor de texto enriquecido guarda el HTML de lo que muestra, así que
// restoreUploadUrls devuelve las URLs originales antes de guardar: un blob: en
// una descripción no sirve fuera de esta pestaña.

const UPLOADS_RE = /^(?:https?:\/\/[^/]+)?\/api\/uploads\/[^?#]+/

const blobByPath = new Map<string, Promise<string>>()
const pathByBlob = new Map<string, string>()

function uploadPath(url: string | null): string | null {
  if (!url) return null
  const m = UPLOADS_RE.exec(url)
  if (!m) return null
  // Solo el propio origen: un enlace absoluto a otro sitio no se toca.
  if (/^https?:/.test(m[0]) && !m[0].startsWith(window.location.origin)) return null
  return m[0].replace(/^https?:\/\/[^/]+/, '')
}

function blobFor(path: string): Promise<string> {
  let p = blobByPath.get(path)
  if (!p) {
    p = api
      .get(path.replace(/^\/api/, ''), { responseType: 'blob' })
      .then((res) => {
        const url = URL.createObjectURL(res.data as Blob)
        pathByBlob.set(url, path)
        return url
      })
    // Un fallo no se cachea: el siguiente intento vuelve a pedirlo.
    p.catch(() => blobByPath.delete(path))
    blobByPath.set(path, p)
  }
  return p
}

function rewriteImage(img: HTMLImageElement): void {
  const path = uploadPath(img.getAttribute('src'))
  if (!path) return
  blobFor(path)
    .then((url) => {
      // Si mientras tanto la imagen cambió de src, no se pisa.
      if (uploadPath(img.getAttribute('src')) === path) img.setAttribute('src', url)
    })
    .catch(() => { /* se queda rota, como sin sesión */ })
}

function scan(node: Node): void {
  if (node instanceof HTMLImageElement) {
    rewriteImage(node)
  } else if (node instanceof Element) {
    node.querySelectorAll('img').forEach(rewriteImage)
  }
}

async function openUpload(path: string, filename: string): Promise<void> {
  // La ventana se abre YA, dentro del clic, para que no la bloquee el
  // navegador; el blob se le asigna cuando llega.
  const win = window.open('', '_blank')
  try {
    const url = await blobFor(path)
    if (win) {
      win.location.href = url
      return
    }
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    document.body.appendChild(a)
    a.click()
    a.remove()
  } catch {
    win?.close()
  }
}

function onClick(e: MouseEvent): void {
  if (e.defaultPrevented || e.button !== 0) return
  const a = (e.target as Element | null)?.closest?.('a[href]') as HTMLAnchorElement | null
  if (!a) return
  const path = uploadPath(a.getAttribute('href'))
  if (!path) return
  e.preventDefault()
  void openUpload(path, a.getAttribute('download') || path.split('/').pop() || 'archivo')
}

/** Activa la reescritura para todo el documento. Devuelve cómo apagarla. */
export function installAuthedFiles(): () => void {
  scan(document.body)
  const observer = new MutationObserver((mutations) => {
    for (const m of mutations) {
      if (m.type === 'attributes') scan(m.target)
      else m.addedNodes.forEach(scan)
    }
  })
  observer.observe(document.body, { childList: true, subtree: true, attributes: true, attributeFilter: ['src'] })
  document.addEventListener('click', onClick, true)
  return () => {
    observer.disconnect()
    document.removeEventListener('click', onClick, true)
  }
}

/**
 * Para HTML que se escribe fuera del DOM vigilado (la ventana de impresión de
 * Horas): cambia cada src de /api/uploads por su blob:. Fuera del modo
 * embebido no hace nada, porque allí las imágenes van con la cookie.
 */
export async function resolveUploadImages(html: string): Promise<string> {
  if (!isEmbedMode()) return html
  const srcs = new Set<string>()
  html.replace(/<img\s[^>]*?\bsrc="([^"]+)"/gi, (_m, src: string) => {
    if (uploadPath(src)) srcs.add(src)
    return _m
  })
  let out = html
  for (const src of srcs) {
    try {
      const blob = await blobFor(uploadPath(src)!)
      out = out.split(`src="${src}"`).join(`src="${blob}"`)
    } catch {
      /* se queda rota, como sin sesión */
    }
  }
  return out
}

/** Devuelve a su URL original los blob: que salieron de aquí. Fuera del modo embebido no hace nada. */
export function restoreUploadUrls(html: string): string {
  if (!isEmbedMode() || pathByBlob.size === 0) return html
  let out = html
  pathByBlob.forEach((path, blob) => {
    out = out.split(blob).join(path)
  })
  return out
}
