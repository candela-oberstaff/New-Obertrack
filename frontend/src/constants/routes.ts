export const ROUTES = {
  login: '/login',
  register: '/register',
  forgotPassword: '/forgot-password',
  resetPassword: '/reset-password',
  dashboard: '/dashboard',
} as const

export const AUTH_REDIRECT_PATH = ROUTES.dashboard
export const UNAUTHORIZED_REDIRECT_PATH = ROUTES.dashboard
