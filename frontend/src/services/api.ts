/**
 * Barrel re-export — mantiene compatibilidad con todos los imports existentes.
 * Los servicios reales viven en archivos individuales dentro de services/.
 */
export { authService } from './auth.service'
export { userService } from './user.service'
export { taskService } from './task.service'
export { workHourService } from './work-hour.service'
export { adminService } from './admin.service'
export { employerService } from './employer.service'
export { boardService } from './board.service'
export { channelService } from './channel.service'
export { uploadService } from './upload.service'
export type { UploadResponse } from './upload.service'
export { notificationService } from './notification.service'
export { tutorialService } from './tutorial.service'

// Re-export Notification type for Notifications.tsx (import { type Notification } from '../services/api')
export type { Notification } from '../types'

export { default } from './client'
