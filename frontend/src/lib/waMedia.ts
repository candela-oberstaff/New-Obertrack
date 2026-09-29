import { ticketService } from '../services/ticket.service'

/**
 * Adjuntos de WhatsApp (WAHA), compartido entre la bandeja de WhatsApp y el
 * detalle del ticket de Soporte para que los dos se comporten igual.
 *
 * Un mensaje con adjunto se guarda con un marcador de texto (lo genera el
 * backend en MediaPlaceholder) o, en importaciones viejas, con el nombre del
 * archivo. El archivo en sí no se guarda: se pide a WAHA por el id externo
 * del mensaje cada vez que alguien lo descarga.
 */

/** Marcadores que escribe el backend cuando llega un archivo en vez de texto. */
export const WA_MEDIA_PLACEHOLDERS = [
  '📷 Imagen recibida',
  '🎥 Video recibido',
  '🎤 Nota de voz recibida',
  '📄 Documento recibido',
  '🏷️ Sticker recibido',
  '📍 Ubicación recibida',
  '👤 Contacto recibido',
  '📎 Archivo adjunto recibido',
]

const FILE_EXTENSIONS = [
  '.pdf', '.doc', '.docx', '.xls', '.xlsx', '.ppt', '.pptx', '.txt', '.csv', '.rtf', '.odt',
  '.png', '.jpg', '.jpeg', '.gif', '.webp', '.heic',
  '.ogg', '.opus', '.mp3', '.wav', '.m4a', '.aac',
  '.mp4', '.mov', '.avi', '.webm', '.3gp',
  '.zip', '.rar', '.7z',
]

/** Un mensaje es adjunto si tiene id externo y su texto es un marcador o un nombre de archivo. */
export function isWaMediaMessage(msg: { content: string; external_id?: string | null }): boolean {
  if (!msg.external_id) return false
  const content = (msg.content ?? '').trim()
  if (!content) return false
  if (WA_MEDIA_PLACEHOLDERS.includes(content)) return true
  const lower = content.toLowerCase()
  return FILE_EXTENSIONS.some((ext) => lower.endsWith(ext))
}

/** Extensión de archivo según el tipo de contenido que devolvió WAHA. */
export function extensionForContentType(contentType: string): string {
  const ct = contentType.toLowerCase()
  const table: [string, string][] = [
    ['image/jpeg', '.jpg'],
    ['image/png', '.png'],
    ['image/webp', '.webp'],
    ['image/gif', '.gif'],
    ['video/mp4', '.mp4'],
    ['video/3gpp', '.3gp'],
    ['video/webm', '.webm'],
    ['audio/ogg', '.ogg'],
    ['audio/opus', '.opus'],
    ['audio/mpeg', '.mp3'],
    ['audio/mp4', '.m4a'],
    ['audio/aac', '.aac'],
    ['audio/wav', '.wav'],
    ['application/pdf', '.pdf'],
    ['application/msword', '.doc'],
    ['application/vnd.openxmlformats-officedocument.wordprocessingml.document', '.docx'],
    ['application/vnd.ms-excel', '.xls'],
    ['application/vnd.openxmlformats-officedocument.spreadsheetml.sheet', '.xlsx'],
    ['application/vnd.ms-powerpoint', '.ppt'],
    ['application/vnd.openxmlformats-officedocument.presentationml.presentation', '.pptx'],
    ['text/plain', '.txt'],
    ['text/csv', '.csv'],
    ['application/zip', '.zip'],
    ['application/x-rar-compressed', '.rar'],
  ]
  for (const [type, ext] of table) {
    if (ct.includes(type)) return ext
  }
  return ''
}

/** Nombre sugerido: si el mensaje trae el nombre del archivo, se conserva. */
function suggestedName(content: string, fallback: string, ext: string): string {
  const trimmed = (content ?? '').trim()
  if (trimmed && !WA_MEDIA_PLACEHOLDERS.includes(trimmed) && /\.[a-z0-9]{2,5}$/i.test(trimmed)) {
    return trimmed.replace(/[\\/:*?"<>|]/g, '_')
  }
  return `${fallback}${ext}`
}

/**
 * Pide el archivo a WAHA a través del backend y dispara la descarga en el
 * navegador. Lanza si WAHA ya no lo tiene: quien llama decide cómo avisar.
 */
export async function downloadWaMedia(
  ticketId: string | number,
  msg: { id: string | number; external_id?: string | null; content: string }
): Promise<void> {
  if (!msg.external_id) throw new Error('El mensaje no tiene adjunto')
  const { blob, contentType } = await ticketService.downloadWaMedia(String(ticketId), msg.external_id)
  const ext = extensionForContentType(contentType)
  const url = window.URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = suggestedName(msg.content, `adjunto_${msg.id}`, ext)
  document.body.appendChild(a)
  a.click()
  window.URL.revokeObjectURL(url)
  document.body.removeChild(a)
}
