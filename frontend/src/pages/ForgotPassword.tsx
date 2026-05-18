import { useState } from 'react'
import { authService } from '../services/auth.service'
import styles from './Auth.module.css'

export default function ForgotPassword() {
  const [email, setEmail] = useState('')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState(false)
  const [isLoading, setIsLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setIsLoading(true)

    try {
      await authService.forgotPassword(email)
      setSuccess(true)
    } catch (err: any) {
      setError(err?.response?.data?.error || 'Ocurrió un error. Intenta de nuevo.')
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className={styles['auth-container']}>
      <div className={styles['auth-card']}>
        <img src="/logos/Vertical_Blanco.png" alt="Obertrack" className={styles['auth-logo']} />
        <p className={styles['auth-tagline']}>Remote Work Tracking</p>
        <h2>Recuperar Contraseña</h2>

        {success ? (
          <div style={{ textAlign: 'center', marginTop: '16px' }}>
            <div style={{
              width: '64px',
              height: '64px',
              borderRadius: '50%',
              background: 'rgba(16, 185, 129, 0.15)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              margin: '0 auto 16px',
              fontSize: '28px',
            }}>
              ✉️
            </div>
            <p style={{ color: 'rgba(255,255,255,0.85)', fontSize: '15px', lineHeight: '1.6', marginBottom: '24px' }}>
              Si el correo <strong style={{ color: '#15f4ee' }}>{email}</strong> está registrado, recibirás un enlace para restablecer tu contraseña.
            </p>
            <p style={{ color: 'rgba(255,255,255,0.45)', fontSize: '13px' }}>
              Revisa tu bandeja de entrada y carpeta de spam.
            </p>
            <p className={styles['auth-link']} style={{ marginTop: '32px' }}>
              <a href="/login">← Volver al inicio de sesión</a>
            </p>
          </div>
        ) : (
          <>
            <p style={{ color: 'rgba(255,255,255,0.5)', fontSize: '13px', textAlign: 'center', marginBottom: '20px' }}>
              Ingresa tu correo electrónico y te enviaremos un enlace para restablecer tu contraseña.
            </p>

            {error && <div className={styles['error-message']}>{error}</div>}

            <form onSubmit={handleSubmit}>
              <div className={styles['form-group']}>
                <label htmlFor="email">Email</label>
                <input
                  id="email"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="tu@correo.com"
                  required
                />
              </div>

              <button type="submit" disabled={isLoading}>
                {isLoading ? 'Enviando...' : 'Enviar Enlace'}
              </button>
            </form>

            <p className={styles['auth-link']}>
              <a href="/login">← Volver al inicio de sesión</a>
            </p>
          </>
        )}
      </div>
    </div>
  )
}
