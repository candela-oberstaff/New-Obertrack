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
  created_by: number
  unread_count: number
  created_at: string
  recipient?: User
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
  unread_count: number
  last_message?: string
  last_message_at?: string
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
