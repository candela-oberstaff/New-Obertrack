export type UserType = 'empleador' | 'profesional' | 'superadmin' | 'customer_success'
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
  state?: string
  city?: string
  location?: string
  address?: string
  industry?: string
  created_at: string
  updated_at: string
  password?: string
  zoho_agent_id?: string
}

export interface Tenant {
  id: number
  company_name: string
  owner_name: string
  owner_email: string
  is_active: boolean
  user_count: number
  board_count: number
  task_count: number
  created_at: string
}

export interface EmployeeSummary {
  id: number
  name: string
  email: string
  avatar?: string
  user_type: string
  is_active: boolean
  is_manager: boolean
  hours_this_month: number
  tasks_assigned: number
  tasks_completed: number
  last_active?: string | null
}

export interface EmployeeWorkHour {
  id: number
  work_date: string
  work_type: string
  hours_worked: number
  approved: boolean
  activities: string
}

export interface EmployeeTask {
  id: number
  title: string
  status: string
  completed: boolean
  end_date?: string | null
  board_name: string
}

export interface EmployeeTracking {
  user: User
  summary: EmployeeSummary
  work_hours: EmployeeWorkHour[]
  tasks: EmployeeTask[]
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
  board_id: number
  tenant_id?: number
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
  work_type: 'complete' | 'absence' | 'recover'
  hours_worked: number
  activities?: string
  start_time?: string
  end_time?: string
  approved: boolean
  approved_by?: number
  approved_at?: string
  approved_by_user?: User
  rejected?: boolean
  rejected_by?: number
  rejected_at?: string
  rejected_by_user?: User
  rejection_reason?: string
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

export interface ColumnType {
  id: string
  title: string
  color: string
}
