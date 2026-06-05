import DOMPurify from 'dompurify'

// Allow-list of formatting tags produced by the RichTextEditor. Anything else
// (script, iframe, event handlers, etc.) is stripped to prevent stored XSS
// (audit finding C-05).
const ALLOWED_TAGS = [
  'b', 'strong', 'i', 'em', 'u', 's', 'p', 'br', 'ul', 'ol', 'li',
  'h1', 'h2', 'h3', 'h4', 'blockquote', 'a', 'span', 'div',
]
const ALLOWED_ATTR = ['href', 'target', 'rel']

/**
 * sanitizeHtml returns a safe HTML string ready to be injected via
 * dangerouslySetInnerHTML. Never render untrusted HTML without passing it
 * through this function first.
 */
export function sanitizeHtml(dirty: string | null | undefined): string {
  if (!dirty) return ''
  return DOMPurify.sanitize(dirty, {
    ALLOWED_TAGS,
    ALLOWED_ATTR,
    // Force external links to be safe.
    ADD_ATTR: ['target'],
  })
}

/**
 * htmlToText returns a plain-text representation (all markup removed), safe to
 * render as a normal React text node. Use this for previews/snippets.
 */
export function htmlToText(html: string | null | undefined): string {
  if (!html) return ''
  // DOMPurify strips tags but returns HTML-encoded text (e.g. "&amp;" instead of "&").
  // Use a textarea to decode all HTML entities back to their plain-text equivalents.
  const clean = DOMPurify.sanitize(html, { ALLOWED_TAGS: [], ALLOWED_ATTR: [] })
  const decoder = document.createElement('textarea')
  decoder.innerHTML = clean
  return (decoder.value || clean).replace(/\s+/g, ' ').trim()
}

// Harden anchor links: open in new tab without leaking the opener and block
// javascript: URLs (DOMPurify already drops those, this is defense in depth).
DOMPurify.addHook('afterSanitizeAttributes', (node) => {
  if (node.tagName === 'A') {
    node.setAttribute('target', '_blank')
    node.setAttribute('rel', 'noopener noreferrer')
  }
})
