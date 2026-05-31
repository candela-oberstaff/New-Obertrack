import type { User } from './tasks'

export interface Tutorial {
  id: number
  title: string
  description: string
  google_drive_url: string
  icon_name: string
  category: string
  duration_min: number
  order_index: number
  is_active: boolean
  created_by: number
  creator?: User
  created_at: string
  updated_at: string
}

export interface CreateTutorialInput {
  title: string
  description: string
  google_drive_url: string
  icon_name: string
  category: string
  duration_min: number
  order_index: number
  is_active: boolean
}

export type UpdateTutorialInput = Partial<CreateTutorialInput>
