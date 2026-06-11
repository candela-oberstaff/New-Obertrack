import { createContext, useContext, useState, useEffect, type ReactNode } from 'react'
import axios from 'axios'
import type { User } from '../types'
import { authService } from '../services/api'

interface AuthContextType {
  user: User | null
  login: (email: string, password: string) => Promise<void>
  register: (data: RegisterData) => Promise<void>
  logout: () => Promise<void>
  isLoading: boolean
  setUser: (user: User | null) => void
}

interface RegisterData {
  name: string
  email: string
  password: string
  user_type?: string
  company_name?: string
  industry?: string
  empleador_id?: number
  phone_number?: string
  country?: string
  state?: string
  city?: string
  location?: string
  address?: string
  job_title?: string
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  // Auth state is driven by the httpOnly session cookie. On load we ask the
  // server who we are; if the cookie is missing/expired this 401s and we stay
  // logged out (audit findings A-03/A-04).
  useEffect(() => {
    const initAuth = async () => {
      try {
        const userData = await authService.me()
        setUser(userData)
      } catch {
        setUser(null)
      }
      setIsLoading(false)
    }
    initAuth()
  }, [])

  const login = async (email: string, password: string) => {
    const response = await authService.login(email, password)
    setUser(response.user)
  }

  const register = async (data: RegisterData) => {
    try {
      const response = await authService.register(data)
      setUser(response.user)
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.data) {
        const errorData = err.response.data as { error?: string; details?: string }
        throw new Error(errorData.details || errorData.error || 'Error al registrarse')
      }
      throw new Error('Error al registrarse')
    }
  }

  const logout = async () => {
    try {
      await authService.logout()
    } finally {
      setUser(null)
    }
  }

  return (
    <AuthContext.Provider value={{ user, login, register, logout, isLoading, setUser }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}
