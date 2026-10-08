/**
 * Lo que el aviso de novedades ya hizo en ESTE inicio de sesión: qué avisos
 * se mostraron (para contar cada sesión una sola vez) y cuáles se cerraron
 * (para que no vuelvan a salir hasta la próxima sesión).
 *
 * Vive en sessionStorage y no en memoria porque una recarga de la página no es
 * un inicio de sesión nuevo: con un ref, cada F5 gastaba una de las «veces»
 * del aviso y lo volvía a poner delante. Se borra al entrar y al salir.
 */
const PREFIX = 'novedades.aviso.'

export interface AnnouncerSession {
  shown: number[]
  dismissed: number[]
}

function key(userId: number): string {
  return `${PREFIX}${userId}`
}

export function readAnnouncerSession(userId: number): AnnouncerSession {
  try {
    const raw = sessionStorage.getItem(key(userId))
    const parsed = raw ? JSON.parse(raw) : null
    return {
      shown: Array.isArray(parsed?.shown) ? parsed.shown : [],
      dismissed: Array.isArray(parsed?.dismissed) ? parsed.dismissed : [],
    }
  } catch {
    return { shown: [], dismissed: [] }
  }
}

export function writeAnnouncerSession(userId: number, session: AnnouncerSession): void {
  try {
    sessionStorage.setItem(key(userId), JSON.stringify(session))
  } catch {
    /* modo incógnito o almacenamiento bloqueado: se cuenta solo en memoria */
  }
}

/** Empieza una sesión limpia: al iniciar o cerrar sesión. */
export function clearAnnouncerSession(): void {
  try {
    for (let i = sessionStorage.length - 1; i >= 0; i--) {
      const k = sessionStorage.key(i)
      if (k?.startsWith(PREFIX)) sessionStorage.removeItem(k)
    }
  } catch {
    /* sin almacenamiento no hay nada que limpiar */
  }
}
