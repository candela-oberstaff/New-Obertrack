export type UserType = 'empleador' | 'profesional' | 'superadmin'
export type TaskStatus = 'por_hacer' | 'en_proceso' | 'finalizado'
export type TaskPriority = 'low' | 'medium' | 'high' | 'urgent'

export interface User {
  id: number
  name: string
  email: string
  avatar?: string
  user_type: string
  is_manager: boolean
  is_superadmin: boolean
  is_active: boolean
  empleador_id?: number
  manager_id?: number
  company_name?: string
  job_title?: string
  phone_number?: string
  country?: string
  city?: string
  location?: string
  created_at: string
  updated_at: string
  password?: string
}

export interface Task {
  id: number
  title: string
  description: string
  status: TaskStatus
  priority: TaskPriority
  start_date?: string
  end_date?: string
  completed: boolean
  created_by: number
  created_at: string
  updated_at: string
  creator?: User
  assignees?: User[]
  comments?: Comment[]
  attachments?: TaskAttachment[]
}

export interface TaskAttachment {
  id: number
  task_id: number
  file_name: string
  file_url: string
  file_size: number
  mime_type: string
  uploaded_by: number
  created_at: string
}

export interface CreateTaskInput {
  title: string
  description?: string
  priority?: string
  end_date?: string
  assignees?: number[]
  board_id?: number
}

export interface Comment {
  id: number
  task_id: number
  user_id: number
  content: string
  created_at: string
  updated_at: string
  user?: User
}

export interface WorkHour {
  id: number
  user_id: number
  work_date: string
  work_type: 'complete' | 'absence'
  hours_worked: number
  activities?: string
  start_time?: string
  end_time?: string
  approved: boolean
  approved_by?: number
  approved_at?: string
  approved_by_user?: User
  comments?: string
  absence_reason?: string
  absence_hours?: number
  created_at: string
  updated_at: string
  user?: User
}

export interface Notification {
  id: number
  user_id: number
  type: string
  title: string
  message: string
  data?: Record<string, unknown>
  read_at?: string
  created_at: string
}

export interface PaginatedResponse<T> {
  data: T[]
  total: number
  page: number
  limit: number
}

export interface Board {
  id: number
  name: string
  description: string
  color: string
  created_by: number
  created_at: string
  updated_at: string
  creator?: User
  members?: User[]
  phases?: Phase[]
}

export interface Phase {
  id: number
  name: string
  status?: string
  color: string
  order: number
}

export interface CreateBoardInput {
  name?: string
  description?: string
  color?: string
  member_ids?: number[]
  phases?: { name: string; color?: string }[]
}
