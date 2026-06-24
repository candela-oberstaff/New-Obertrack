import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import axios from 'axios'
import { useAuth } from '../context/AuthContext'
import styles from './Auth.module.css'
import AuthLayout from '../components/layout/AuthLayout'

export default function Login() {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const { login } = useAuth()
  const navigate = useNavigate()

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setIsLoading(true)

    try {
      await login(email, password)
      navigate('/dashboard')
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.status === 429) {
        setError('Demasiados intentos. Espera un momento e inténtalo de nuevo.')
      } else {
        setError('Credenciales inválidas')
      }
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <AuthLayout title="Iniciar Sesión">
      {error && <div className={styles['error-message']}>{error}</div>}

      <form onSubmit={handleSubmit}>
        <div className={styles['form-group']}>
          <label htmlFor="email">Email</label>
          <input
            id="email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
        </div>

        <div className={styles['form-group']}>
          <label htmlFor="password">Contraseña</label>
          <input
            id="password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
        </div>

        <button type="submit" className={styles['btn-primary']} disabled={isLoading}>
          {isLoading ? 'Iniciando...' : 'Iniciar Sesión'}
        </button>
      </form>

      <p className={styles['auth-link']} style={{ marginTop: '12px', marginBottom: '0' }}>
        <a href="/forgot-password">¿Olvidaste tu contraseña?</a>
      </p>
    </AuthLayout>
  )
}
