import DOMPurify from 'dompurify'

// 1. Configuración Estándar (Para inputs y textos normales de la app)
const ALLOWED_TAGS = [
  'b', 'strong', 'i', 'em', 'u', 's', 'p', 'br', 'ul', 'ol', 'li',
  'h1', 'h2', 'h3', 'h4', 'blockquote', 'a', 'span', 'div',
]
const ALLOWED_ATTR = ['href', 'target', 'rel']

// 2. Configuración Especial para Emails (Permite estructura y diseño, bloquea JS)
const EMAIL_ALLOWED_TAGS = [
  ...ALLOWED_TAGS,
  'html', 'head', 'body', 'style', 'meta', 'title', 'table', 'thead', 'tbody', 'tr', 'th', 'td', 'img'
]
const EMAIL_ALLOWED_ATTR = [
  ...ALLOWED_ATTR,
  'class', 'style', 'id', 'width', 'height', 'align', 'valign', 'bgcolor', 'border', 'cellspacing', 'cellpadding'
]

/**
 * sanitizeHtml devuelve un string HTML seguro para renderizado estándar en la app.
 */
export function sanitizeHtml(dirty: string | null | undefined): string {
  if (!dirty) return ''
  return DOMPurify.sanitize(dirty, {
    ALLOWED_TAGS,
    ALLOWED_ATTR,
    ADD_ATTR: ['target'],
  })
}

// 3. Configuración "rich": como la estándar pero permite <img> (para editores
// de texto enriquecido, p. ej. la descripción de tareas con imágenes pegadas).
// DOMPurify sigue bloqueando src peligrosos (javascript:) y solo deja
// http/https/data para imágenes.
const RICH_ALLOWED_TAGS = [...ALLOWED_TAGS, 'img']
const RICH_ALLOWED_ATTR = [...ALLOWED_ATTR, 'src', 'alt', 'title', 'width', 'height']

/**
 * sanitizeRichHtml es como sanitizeHtml pero conserva imágenes. Úsalo para
 * contenido que puede incluir imágenes (descripciones de tareas).
 */
export function sanitizeRichHtml(dirty: string | null | undefined): string {
  if (!dirty) return ''
  return DOMPurify.sanitize(dirty, {
    ALLOWED_TAGS: RICH_ALLOWED_TAGS,
    ALLOWED_ATTR: RICH_ALLOWED_ATTR,
    ADD_ATTR: ['target'],
  })
}

/**
 * compileAndSanitizeEmail toma el HTML con el CDN de Tailwind, genera el CSS real
 * en un entorno aislado, remueve los scripts y sanitiza el resultado final de forma segura.
 */
export const compileAndSanitizeEmail = (rawHTML: string): Promise<string> => {
  return new Promise((resolve) => {
    // A. Creamos el iframe oculto para que el CDN de Tailwind procese las clases
    const iframe = document.createElement('iframe')
    iframe.style.display = 'none'
    document.body.appendChild(iframe)

    const iframeDoc = iframe.contentDocument || iframe.contentWindow?.document
    if (!iframeDoc) {
      document.body.removeChild(iframe)
      resolve(DOMPurify.sanitize(rawHTML, { ALLOWED_TAGS: EMAIL_ALLOWED_TAGS, ALLOWED_ATTR: EMAIL_ALLOWED_ATTR }))
      return
    }

    iframeDoc.open()
    iframeDoc.write(rawHTML)
    iframeDoc.close()

    // B. Esperamos 100ms a que Tailwind genere el árbol de estilos
    setTimeout(() => {
      const styleTags = iframeDoc.querySelectorAll('style')
      let compiledCSS = ''
      styleTags.forEach((style) => {
        compiledCSS += style.innerHTML
      })

      // C. Clonamos el resultado y removemos los scripts pesados/peligrosos
      const clone = iframeDoc.documentElement.cloneNode(true) as HTMLElement
      clone.querySelectorAll('script').forEach((s) => s.remove())

      // D. Inyectamos el bloque <style> real con el CSS compilado
      const finalStyleTag = iframeDoc.createElement('style')
      finalStyleTag.innerHTML = compiledCSS
      clone.querySelector('head')?.appendChild(finalStyleTag)

      const rawResult = '<!DOCTYPE html>\n' + clone.outerHTML
      document.body.removeChild(iframe)

      // E. PASO DE SEGURIDAD CRÍTICO: Sanitizamos el HTML final permitiendo estilos corporativos
      const cleanEmailHTML = DOMPurify.sanitize(rawResult, {
        ALLOWED_TAGS: EMAIL_ALLOWED_TAGS,
        ALLOWED_ATTR: EMAIL_ALLOWED_ATTR,
        FORCE_BODY: false, // Evita que tire el <head> a la basura
      })

      resolve(cleanEmailHTML)
    }, 100)
  })
}

/**
 * htmlToLines convierte HTML a texto plano CONSERVANDO los saltos de bloque.
 * Para lo que se escribió en un editor enriquecido (actividades de una jornada,
 * descripción de una tarea) y hay que enseñar sin markup.
 */
export function htmlToLines(html: string | null | undefined): string {
  if (!html) return ''

  // Los límites de bloque se marcan ANTES de quitar el markup. Sin este paso,
  // "…y no podía ayer</div><div>Saber si…" se queda en "ayerSaber": el markup
  // era lo único que separaba las dos frases.
  const withBreaks = html
    .replace(/<br\s*\/?>/gi, '\n')
    .replace(/<li[^>]*>/gi, '\n• ')
    // 'li' NO va en la lista de cierres: su apertura ya abrió línea, y cerrarla
    // otra vez dejaba un renglón en blanco entre viñeta y viñeta.
    .replace(/<\/(p|div|h[1-6]|tr|ul|ol|blockquote)>/gi, '\n')

  const clean = DOMPurify.sanitize(withBreaks, { ALLOWED_TAGS: [], ALLOWED_ATTR: [] })
  // textContent decodifica las entidades (&nbsp;, &amp;, &#39;…) sin tener que
  // mantener a mano una tabla que siempre se queda corta.
  const decoded = new DOMParser().parseFromString(clean, 'text/html').body.textContent || ''

  return decoded
    .replace(/[ \t]+/g, ' ')
    .split('\n')
    .map(line => line.trim())
    .join('\n')
    .replace(/\n{3,}/g, '\n\n')
    .trim()
}

/**
 * htmlToText es lo mismo en UNA línea, para previsualizaciones, celdas de tabla
 * y comprobaciones de "¿escribió algo?".
 */
export function htmlToText(html: string | null | undefined): string {
  return htmlToLines(html).replace(/\s+/g, ' ').trim()
}

// Hook de defensa en profundidad para enlaces externos
DOMPurify.addHook('afterSanitizeAttributes', (node) => {
  if (node.tagName === 'A') {
    node.setAttribute('target', '_blank')
    node.setAttribute('rel', 'noopener noreferrer')
  }
})