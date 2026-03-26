import { User } from './index'

export interface Channel {
  id: number
  name: string
  description: string
  type: 'public' | 'private'
  created_by: number
  unread_count: number
  created_at: string
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
  created_at: string
  user?: User
  tempId?: string
}

export interface ChannelMember {
  id: number
  name: string
  email: string
}
