import { Activity, Sparkles, UserPlus, UserMinus, Clock, ClipboardList, Ban, CheckCircle2, StickyNote, Send, Mail, MessageSquare, Phone, Users, MessageSquareQuote } from 'lucide-react'

// Presentación de los movimientos del expediente. Vive fuera de la pantalla de
// la empresa porque el MISMO expediente se lee desde dos sitios: entero en la
// ficha del tenant y acotado a una persona en la del profesional. Si cada una
// tuviera su tabla, el mismo evento saldría de dos colores según por dónde
// entres.

// Icono y color por tipo de evento.
export const ACTIVITY_STYLE: Record<string, { icon: typeof Activity; color: string }> = {
  company_created: { icon: Sparkles, color: '#2563eb' },
  employee_joined: { icon: UserPlus, color: '#059669' },
  employee_left: { icon: UserMinus, color: '#b45309' },
  work_hour: { icon: Clock, color: '#64748b' },
  follow_up: { icon: ClipboardList, color: '#7c3aed' },
  company_suspended: { icon: Ban, color: '#dc2626' },
  company_reactivated: { icon: CheckCircle2, color: '#059669' },
  company_note: { icon: StickyNote, color: '#0891b2' },
  company_contact: { icon: Send, color: '#4f46e5' },
  company_testimonial: { icon: MessageSquareQuote, color: '#c026d3' },
}

// Un tipo desconocido (uno nuevo del backend que aún no está en la tabla) se
// pinta neutro en vez de romper la fila.
export const ACTIVITY_FALLBACK = { icon: Activity, color: '#64748b' }

// Etiqueta corta del movimiento, para leer el expediente de un vistazo sin
// tener que deducir el tipo por el icono.
export const ACTIVITY_LABEL: Record<string, string> = {
  company_created: 'Alta',
  employee_joined: 'Incorporación',
  employee_left: 'Baja',
  work_hour: 'Jornada',
  follow_up: 'Gestión',
  company_suspended: 'Suspensión',
  company_reactivated: 'Reactivación',
  company_note: 'Nota',
  company_contact: 'Contacto',
  company_testimonial: 'Testimonio',
}

// Icono y etiqueta por canal de contacto. El canal viaja aparte del texto para
// poder distinguir de un vistazo un correo de una llamada, sin leer el detalle.
export const CONTACT_STYLE: Record<string, { icon: typeof Activity; label: string }> = {
  email: { icon: Mail, label: 'Correo' },
  whatsapp: { icon: MessageSquare, label: 'WhatsApp' },
  call: { icon: Phone, label: 'Llamada' },
  meeting: { icon: Users, label: 'Reunión' },
}
