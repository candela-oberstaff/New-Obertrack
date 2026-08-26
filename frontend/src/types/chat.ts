import { User } from './index'

export type SupportStatus = 'open' | 'assigned' | 'resolved'

export interface SupportInfo {
  ticket_id?: number
  subject?: string
  priority?: string
  module?: string
  status: SupportStatus
  assigned_to?: number
  assignee_name?: string
  requester_id: number
  requester_name?: string
  requester_email?: string
  requester_phone?: string
  company_name?: string
  created_at?: string
}

export interface Channel {
  id: number
  name: string
  description: string
  type: 'public' | 'private' | 'direct'
  // Empresa dueña del canal. El alcance del superadmin acota el sidebar por
  // empresa, así que un deep link a un canal de otra necesita este dato para
  // saber a cuál cambiar antes de abrirlo.
  tenant_id?: number
  created_by: number
  unread_count: number
  // Subconjunto de unread_count que MENCIONA al usuario: el sidebar lo pinta
  // en rojo para que "te nombraron" no se pierda entre "hay mensajes".
  mention_count?: number
  // El usuario silenció este canal: sin campana (las menciones sí suenan).
  muted?: boolean
  // Canal de ANUNCIOS: solo los administradores publican; el resto lee,
  // reacciona y comenta en hilos.
  is_announcement?: boolean
  created_at: string
  recipient?: User
  // Solo DMs donde participas: hasta cuándo leyó el OTRO ("✓✓ Visto").
  recipient_last_read_at?: string
  // Solo en DMs vistos por un no-participante (supervisión de superadmin):
  // ambos miembros, para mostrar "A ↔ B" en vez de un nombre arbitrario.
  participants?: User[]
  support?: SupportInfo
  // True cuando un superadmin audita un canal en el que NO participa
  // (DM o privado ajeno). Públicos y canales propios → false.
  supervised?: boolean
}

export interface SupportTicket {
  id: number
  channel_id: number
  requester_id: number
  subject?: string
  priority?: string
  module?: string
  status: SupportStatus
  assigned_to?: number
  assignee?: User
  requester?: User
  created_at?: string
}

/**
 * Agente de soporte como destino de reasignación. El backend devuelve solo
 * estos campos (id/nombre/correo): el modelo User completo exponía teléfono y
 * documento de los agentes.
 */
export interface SupportAgentRef {
  id: number
  name: string
  email: string
}

export interface MySupportTicket {
  id: number
  channel_id: number
  subject?: string
  priority?: string
  module?: string
  status: SupportStatus
  assignee_name?: string
  created_at: string
  updated_at: string
  resolved_at?: string
  /** Mensajes sin leer de la CONVERSACIÓN. Sólo lo trae la solicitud viva: todas
   *  las de una persona comparten el mismo canal de soporte. */
  unread_count: number
  last_message?: string
  last_message_at?: string
}

/** Una página de solicitudes propias, con los totales para la cabecera. */
export interface MySupportPage {
  data: MySupportTicket[]
  total: number
  page: number
  limit: number
  /** Totales GLOBALES, no los de esta página. */
  open: number
  resolved: number
}

export interface DMChannel extends Channel {
  recipient?: User
}

export interface MessageReaction {
  id: number
  message_id: number
  user_id: number
  emoji: string
  user?: User
}

export interface Message {
  id: number
  channel_id: number
  user_id: number
  content: string
  attachment?: string
  file_name?: string
  file_size?: number
  file_type?: string
  is_pinned?: boolean
  is_edited?: boolean
  is_deleted?: boolean
  parent_id?: number
  reply_count?: number
  replies?: Message[]
  reactions?: MessageReaction[]
  created_at: string
  user?: User
  tempId?: string
}

/** Resultado de la búsqueda global: el mensaje + la etiqueta de su canal. */
export interface GlobalSearchHit extends Message {
  channel_name?: string
  channel_type?: Channel['type']
}

export interface ChannelMember {
  id: number
  name: string
  email: string
  role?: 'admin' | 'member'
}

export interface UserStatus {
  user_id: number
  status: 'online' | 'away' | 'offline'
  last_seen: string
}
