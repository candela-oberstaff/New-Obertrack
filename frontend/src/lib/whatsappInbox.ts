import { ticketService } from '../services/ticket.service'

/** Solo los dígitos del teléfono; '' si no hay nada usable. */
export const waDigits = (phone?: string | null) => (phone || '').replace(/\D/g, '')

type NavigateLike = (to: string, options?: { state?: unknown }) => void

/**
 * Abre la conversación de WhatsApp de un teléfono en NUESTRA bandeja, creándola
 * si no existe, y lleva a ella.
 *
 * Es el único camino: la aplicación ya no enlaza a wa.me desde ningún botón de
 * contacto. El enlace externo sacaba a la persona de Obertrack y el mensaje
 * salía desde el WhatsApp personal de quien hacía clic, así que la respuesta del
 * profesional no quedaba registrada en ninguna parte y el seguimiento se perdía.
 *
 * Devuelve false si la bandeja no pudo abrirla, para que quien llama lo avise.
 * Antes eso caía a wa.me en silencio; ahora se dice, porque un envío que se va
 * por fuera sin que nadie lo sepa es peor que un error visible.
 *
 * draft deja el mensaje escrito en el campo de envío (los seguimientos lo usan).
 * No envía nada por su cuenta.
 */
export async function openWaConversation(
  phone: string | null | undefined,
  name: string | undefined,
  navigate: NavigateLike,
  opts?: { draft?: string },
): Promise<boolean> {
  if (!waDigits(phone)) return false
  try {
    const chat = await ticketService.openWaChat(phone as string, name)
    if (!chat.ticket_id) return false
    navigate(`/tickets/wa/${chat.ticket_id}`, opts?.draft ? { state: { draft: opts.draft } } : undefined)
    return true
  } catch {
    return false
  }
}
