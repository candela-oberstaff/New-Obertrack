// Vuelta tras el login a la autorización de un asistente de IA (MCP).
//
// El backend manda a /login?returnTo=/api/mcp/oauth/authorize?... cuando un
// asistente pide acceso y no hay sesión. Solo se acepta ESA ruta, relativa al
// propio dominio: cualquier otra cosa en returnTo sería un redirect abierto.

const ALLOWED_PREFIX = '/api/mcp/oauth/authorize?'

export function safeReturnTo(raw: string | null | undefined): string | null {
  if (!raw) return null
  if (!raw.startsWith(ALLOWED_PREFIX)) return null
  // "//" o "\" tras el prefijo no pueden convertirlo en otro origen, pero se
  // rechazan igual por higiene; tampoco se admiten saltos de línea.
  if (/[\\\r\n]/.test(raw)) return null
  return raw
}

/** Lee returnTo de la URL actual, ya validado. */
export function currentReturnTo(): string | null {
  return safeReturnTo(new URLSearchParams(window.location.search).get('returnTo'))
}
