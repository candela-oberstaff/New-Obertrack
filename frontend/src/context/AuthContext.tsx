import { createContext, useContext, useState, useEffect, type ReactNode } from 'react'
import axios from 'axios'
import type { User } from '../types'
import { authService } from '../services/api'

interface AuthContextType {
  user: User | null
  token: string | null
  login: (email: string, password: string) => Promise<void>
  register: (data: RegisterData) => Promise<void>
  logout: () => void
  isLoading: boolean
  setUser: (user: User | null) => void
}

interface RegisterData {
  name: string
  email: string
  password: string
  user_type?: string
  company_name?: string
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [token, setToken] = useState<string | null>(localStorage.getItem('token'))
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    const initAuth = async () => {
      const storedToken = localStorage.getItem('token')
      if (storedToken) {
        try {
          const userData = await authService.me()
          setUser(userData)
        } catch {
          localStorage.removeItem('token')
          setToken(null)
        }
      }
      setIsLoading(false)
    }
    initAuth()
  }, [])

  const login = async (email: string, password: string) => {
    const response = await authService.login(email, password)
    localStorage.setItem('token', response.access_token)
    setToken(response.access_token)
    setUser(response.user)
  }

  const register = async (data: RegisterData) => {
    try {
      const response = await authService.register(data)
      localStorage.setItem('token', response.access_token)
      setToken(response.access_token)
      setUser(response.user)
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.data) {
        const errorData = err.response.data as { error?: string; details?: string }
        throw new Error(errorData.details || errorData.error || 'Error al registrarse')
      }
      throw new Error('Error al registrarse')
    }
  }

  const logout = () => {
    localStorage.removeItem('token')
    setToken(null)
    setUser(null)
  }

  return (
    <AuthContext.Provider value={{ user, token, login, register, logout, isLoading, setUser }}>
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
