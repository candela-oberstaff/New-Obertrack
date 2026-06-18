import { User } from './index'

export type SupportStatus = 'open' | 'assigned' | 'resolved'

export interface SupportInfo {
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
  support?: SupportInfo
}

export interface SupportTicket {
  id: number
  channel_id: number
  requester_id: number
  status: SupportStatus
  assigned_to?: number
  assignee?: User
  requester?: User
  created_at?: string
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
}

export interface UserStatus {
  user_id: number
  status: 'online' | 'away' | 'offline'
  last_seen: string
}
