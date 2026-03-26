import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { authService } from '../services/api'

export default function Register() {
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [userType, setUserType] = useState('profesional')
  const [companyName, setCompanyName] = useState('')
  const [empleadorId, setEmpleadorId] = useState<number | undefined>(undefined)
  const [employers, setEmployers] = useState<{ id: number; name: string; company_name: string }[]>([])
  const [error, setError] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const { register } = useAuth()
  const navigate = useNavigate()

  // Fetch employers when the component mounts
  useEffect(() => {
    const fetchEmployers = async () => {
      try {
        const data = await authService.getEmployers()
        setEmployers(data)
      } catch (err) {
        console.error('Error fetching employers:', err)
      }
    }
    fetchEmployers()
  }, [])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setIsLoading(true)

    try {
      await register({
        name,
        email,
        password,
        user_type: userType,
        company_name: companyName,
        empleador_id: userType === 'profesional' ? empleadorId : undefined,
      })
      navigate('/dashboard')
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Error al registrarse')
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className="auth-container">
      <div className="auth-card">
        <h1>Obertrack</h1>
        <h2>Crear Cuenta</h2>

        {error && <div className="error-message">{error}</div>}

        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label htmlFor="name">Nombre completo</label>
            <input
              id="name"
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
            />
          </div>

          <div className="form-group">
            <label htmlFor="email">Email</label>
            <input
              id="email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
            />
          </div>

          <div className="form-group">
            <label htmlFor="password">Contraseña</label>
            <input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              minLength={6}
            />
          </div>

          <div className="form-group">
            <label htmlFor="userType">Tipo de usuario</label>
            <select
              id="userType"
              value={userType}
              onChange={(e) => {
                setUserType(e.target.value)
                // Reset related fields when changing type
                setEmpleadorId(undefined)
                setCompanyName('')
              }}
            >
              <option value="profesional">Profesional</option>
              <option value="empleador">Empresa</option>
            </select>
          </div>

          {userType === 'profesional' && employers.length > 0 && (
            <div className="form-group">
              <label htmlFor="empleadorId">Empresa a la que pertenece</label>
              <select
                id="empleadorId"
                value={empleadorId ?? ''}
                onChange={(e) => setEmpleadorId(e.target.value ? Number(e.target.value) : undefined)}
              >
                <option value="">Seleccionar empresa...</option>
                {employers.map((emp) => (
                  <option key={emp.id} value={emp.id}>
                    {emp.company_name || emp.name}
                  </option>
                ))}
              </select>
            </div>
          )}

          {userType === 'empleador' && (
            <div className="form-group">
              <label htmlFor="companyName">Nombre de la empresa</label>
              <input
                id="companyName"
                type="text"
                value={companyName}
                onChange={(e) => setCompanyName(e.target.value)}
              />
            </div>
          )}

          <button type="submit" disabled={isLoading}>
            {isLoading ? 'Creando cuenta...' : 'Registrarse'}
          </button>
        </form>

        <p className="auth-link">
          ¿Ya tienes cuenta? <a href="/login">Inicia sesión</a>
        </p>
      </div>
    </div>
  )
}
