import { Activity, ClipboardList, MessageSquare, Inbox, GraduationCap } from 'lucide-react'

// De dónde sale cada ticket. "internal" son alertas que genera la propia
// plataforma (rechazos de horas); el resto son conversaciones con el cliente.
export const TICKET_ORIGIN: Record<string, { label: string; color: string; icon: typeof Activity }> = {
  internal: { label: 'Alerta interna', color: '#f97316', icon: ClipboardList },
  obersuite: { label: 'Obersuite', color: '#7c3aed', icon: GraduationCap },
  whatsapp: { label: 'WhatsApp', color: '#059669', icon: MessageSquare },
  zoho: { label: 'Zoho Desk', color: '#2563eb', icon: Inbox },
}

// Un origen desconocido se enseña tal cual en vez de dejar la celda vacía.
export const ticketOrigin = (origin: string) =>
  TICKET_ORIGIN[origin] || { label: origin || '—', color: '#64748b', icon: Inbox }

export const TICKET_STAGE: Record<string, string> = {
  new: 'Nuevo',
  in_progress: 'En curso',
  waiting: 'En espera',
  closed: 'Cerrado',
}

// Cada origen tiene su propia pantalla de detalle: es donde se responde el
// WhatsApp o se gestiona la alerta, y llevar a la genérica perdería el hilo.
export function ticketPath(t: { id: number; origin: string }): string {
  if (t.origin === 'whatsapp') return `/tickets/wa/${t.id}`
  // Las altas de Obersuite comparten ficha con las alertas internas: viven en
  // nuestra base y se gestionan igual.
  if (t.origin === 'internal' || t.origin === 'obersuite') return `/tickets/internal/${t.id}`
  return `/tickets/${t.id}`
}
