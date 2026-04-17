import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { authService } from '../services/api'
import styles from './Auth.module.css'

export default function Register() {
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [userType, setUserType] = useState('profesional')
  const [companyName, setCompanyName] = useState('')
  const [selectedCompanyId, setSelectedCompanyId] = useState<number | ''>('')
  const [companies, setCompanies] = useState<{ id: number; name: string }[]>([])
  const [error, setError] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  
  const { register } = useAuth()
  const navigate = useNavigate()

  useEffect(() => {
    const fetchCompanies = async () => {
      try {
        const data = await authService.getPublicCompanies()
        console.log('Public companies fetched:', data)
        setCompanies(data)
      } catch (err) {
        console.error('Error fetching companies:', err)
      }
    }
    fetchCompanies()
  }, [])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    
    if (userType === 'profesional' && !selectedCompanyId) {
      setError('Debes seleccionar una empresa para registrarte como profesional')
      return
    }

    setIsLoading(true)

    try {
      await register({
        name,
        email,
        password,
        user_type: userType,
        company_name: userType === 'empleador' ? companyName : undefined,
        empleador_id: userType === 'profesional' ? (selectedCompanyId as number) : undefined,
      })
      navigate('/dashboard')
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Error al registrarse')
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className={styles['auth-container']}>
      <div className={styles['auth-card']}>
        <h1>Obertrack</h1>
        <div className={styles['auth-header']}>
          <h2>Crear Cuenta</h2>
          <p>Únete a la plataforma de gestión de equipos</p>
        </div>

        {error && <div className={styles['error-message']}>{error}</div>}

        <form onSubmit={handleSubmit}>
          <div className={styles['form-group']}>
            <label htmlFor="name">Nombre completo</label>
            <input
              id="name"
              type="text"
              placeholder="Juan Pérez"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
            />
          </div>

          <div className={styles['form-group']}>
            <label htmlFor="email">Email</label>
            <input
              id="email"
              type="email"
              placeholder="juan@ejemplo.com"
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
              placeholder="Min. 6 caracteres"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              minLength={6}
            />
          </div>

          <div className={styles['form-group']}>
            <label htmlFor="userType">Tipo de usuario</label>
            <select
              id="userType"
              value={userType}
              onChange={(e) => setUserType(e.target.value)}
            >
              <option value="profesional">Profesional (Profesional que presta servicios)</option>
              <option value="empleador">Empresa (Dueño o Administrador de empresa)</option>
              <option value="superadmin">Super Admin</option>
            </select>
          </div>

          {userType === 'profesional' && (
            <div className={styles['form-group']}>
              <label htmlFor="companySelect">Empresa a la que perteneces (Cargadas: {companies.length})</label>
              <select
                id="companySelect"
                value={selectedCompanyId}
                onChange={(e) => setSelectedCompanyId(Number(e.target.value))}
                required
              >
                <option value="">Selecciona una empresa...</option>
                {companies.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </select>
              {companies.length === 0 && (
                <p className={styles['field-hint']}>
                  Si no ves tu empresa, asegúrate de que el administrador de la misma ya haya creado una cuenta.
                </p>
              )}
            </div>
          )}

          {userType === 'empleador' && (
            <div className={styles['form-group']}>
              <label htmlFor="companyName">Nombre de tu empresa</label>
              <input
                id="companyName"
                type="text"
                placeholder="Mi Empresa S.A."
                value={companyName}
                onChange={(e) => setCompanyName(e.target.value)}
                required
              />
            </div>
          )}

          <button type="submit" className={styles['btn-primary']} disabled={isLoading}>
            {isLoading ? 'Creando cuenta...' : 'Registrarse'}
          </button>
        </form>

        <p className={styles['auth-link']}>
          ¿Ya tienes cuenta? <a href="/login">Inicia sesión</a>
        </p>
      </div>
    </div>
  )
}
