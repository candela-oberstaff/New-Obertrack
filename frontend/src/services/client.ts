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
let isRedirecting = false

// Rutas que se navegan SIN sesión: si una petición falla con 401 estando aquí,
// no se debe expulsar al usuario a /login. La inducción y la encuesta de las
// empresas entran en este grupo porque quien las abre no tiene por qué tener
// sesión: su credencial es el token del enlace que recibió por correo.
const publicAuthPaths = [
  '/login',
  '/register',
  '/forgot-password',
  '/reset-password',
  '/induccion',
  '/testimonio',
  '/encuesta',
]

function isPublicAuthPath(pathname: string) {
  // El `${path}/` es lo que cubre las que llevan token (/encuesta/3.7.…): sin
  // él, la lista nombraba rutas que en la práctica nunca coincidían y un 401
  // mandaba a /login a alguien que no tiene cuenta con la que volver.
  return publicAuthPaths.some(
    (path) => pathname === path || pathname.startsWith(`${path}?`) || pathname.startsWith(`${path}/`),
  )
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
      if (isRedirecting) {
        return Promise.reject(error)
      }

      original._retry = true

      if (refreshing) {
        try {
          await refreshing
          return api(original)
        } catch (err) {
          return Promise.reject(error)
        }
      }

      try {
        refreshing = api.post('/auth/refresh').then(() => undefined)
        await refreshing
        refreshing = null
        return api(original)
      } catch (refreshErr) {
        refreshing = null
        if (!isPublicAuthPath(window.location.pathname)) {
          isRedirecting = true
          window.location.href = '/login'
        }
        return Promise.reject(error)
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
