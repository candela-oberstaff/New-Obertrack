import axios from 'axios'

// Auth is carried by httpOnly cookies (audit findings A-03/A-04); the browser
// attaches them automatically. withCredentials must be true so cookies are sent
// on cross-origin requests and stored from responses.
const api = axios.create({
  baseURL: '/api',
  withCredentials: true,
})

// On a 401, try to silently refresh the session once, then replay the request.
// If refresh fails, redirect to login.
let refreshing: Promise<void> | null = null

const publicAuthPaths = ['/login', '/register', '/forgot-password', '/reset-password']

function isPublicAuthPath(pathname: string) {
  return publicAuthPaths.some((path) => pathname === path || pathname.startsWith(`${path}?`))
}

api.interceptors.response.use(
  (response) => response,
  async (error) => {
    const original = error.config
    const status = error.response?.status
    const url: string = original?.url || ''

    const isAuthEndpoint =
      url.includes('/auth/refresh') || url.includes('/auth/login') || url.includes('/auth/logout')

    if (status === 401 && !original?._retry && !isAuthEndpoint) {
      original._retry = true
      try {
        if (!refreshing) {
          refreshing = api.post('/auth/refresh').then(() => undefined)
        }
        await refreshing
        refreshing = null
        return api(original)
      } catch (refreshErr) {
        refreshing = null
        if (!isPublicAuthPath(window.location.pathname)) {
          window.location.href = '/login'
        }
        return Promise.reject(refreshErr)
      }
    }

    // Surface tenant-scope rejections with a clear error message
    if (status === 403) {
      const serverMsg = error.response?.data?.error || 'Access denied'
      const enhanced = new Error(serverMsg)
      return Promise.reject(enhanced)
    }

    return Promise.reject(error)
  }
)

export default api
