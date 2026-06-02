import { useState } from 'react'
import { useSearchParams, useNavigate } from 'react-router-dom'
import { authService } from '../services/auth.service'
import styles from './Auth.module.css'
import AuthLayout from '../components/layout/AuthLayout'

export default function ResetPassword() {
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const token = searchParams.get('token') || ''

  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState(false)
  const [isLoading, setIsLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')

    if (password.length < 8) {
      setError('La contraseña debe tener al menos 8 caracteres e incluir letras y números.')
      return
    }

    if (password !== confirmPassword) {
      setError('Las contraseñas no coinciden.')
      return
    }

    if (!token) {
      setError('Token de recuperación no válido.')
      return
    }

    setIsLoading(true)

    try {
      await authService.resetPassword(token, password)
      setSuccess(true)
      setTimeout(() => navigate('/login'), 3000)
    } catch (err: unknown) {
      const responseError = err && typeof err === 'object' && 'response' in err
        ? (err as { response?: { data?: { error?: string } } }).response?.data?.error
        : undefined
      const msg = responseError || 'Ocurrió un error. Intenta de nuevo.'
      if (msg.includes('expired')) {
        setError('El enlace ha expirado. Solicita uno nuevo.')
      } else if (msg.includes('invalid')) {
        setError('El enlace no es válido. Solicita uno nuevo.')
      } else {
        setError(msg)
      }
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <AuthLayout title="Nueva Contraseña">
      {success ? (
        <div style={{ textAlign: 'center', marginTop: '16px' }}>
          <div
            style={{
              width: '64px',
              height: '64px',
              borderRadius: '50%',
              background: 'rgba(16, 185, 129, 0.15)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              margin: '0 auto 16px',
              fontSize: '28px',
            }}
          >
            ✓
          </div>
          <p style={{ color: 'rgba(255,255,255,0.85)', fontSize: '15px', lineHeight: '1.6', marginBottom: '16px' }}>
            Tu contraseña ha sido actualizada exitosamente.
          </p>
          <p style={{ color: 'rgba(255,255,255,0.45)', fontSize: '13px' }}>
            Serás redirigido al inicio de sesión en unos segundos...
          </p>
          <p className={styles['auth-link']} style={{ marginTop: '24px' }}>
            <a href="/login">Ir al inicio de sesión</a>
          </p>
        </div>
      ) : (
        <>
          {!token && (
            <div style={{ textAlign: 'center', marginTop: '16px' }}>
              <p style={{ color: '#fca5a5', fontSize: '14px', marginBottom: '16px' }}>
                No se encontró un token de recuperación válido en la URL.
              </p>
              <p className={styles['auth-link']}>
                <a href="/forgot-password">Solicitar un nuevo enlace</a>
              </p>
            </div>
          )}

          {token && (
            <>
              <p style={{ color: 'rgba(255,255,255,0.5)', fontSize: '13px', textAlign: 'center', marginBottom: '20px' }}>
                Ingresa tu nueva contraseña para completar el proceso.
              </p>

              {error && <div className={styles['error-message']}>{error}</div>}

              <form onSubmit={handleSubmit}>
                <div className={styles['form-group']}>
                  <label htmlFor="password">Nueva Contraseña</label>
                  <input
                    id="password"
                    type="password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    placeholder="Mínimo 8 caracteres con letras y números"
                    required
                    minLength={8}
                  />
                </div>

                <div className={styles['form-group']}>
                  <label htmlFor="confirmPassword">Confirmar Contraseña</label>
                  <input
                    id="confirmPassword"
                    type="password"
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
                    placeholder="Repite tu contraseña"
                    required
                    minLength={8}
                  />
                </div>

                <button type="submit" disabled={isLoading}>
                  {isLoading ? 'Actualizando...' : 'Restablecer Contraseña'}
                </button>
              </form>

              <p className={styles['auth-link']}>
                <a href="/login">Volver al inicio de sesión</a>
              </p>
            </>
          )}
        </>
      )}
    </AuthLayout>
  )
}
