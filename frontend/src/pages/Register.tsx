import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { authService } from '../services/api'
import styles from './Auth.module.css'
import AuthLayout from '../components/layout/AuthLayout'

export default function Register() {
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [userType, setUserType] = useState('profesional')
  const [companyName, setCompanyName] = useState('')
  const [selectedCompanyId, setSelectedCompanyId] = useState<number | ''>('')
  const [phoneNumber, setPhoneNumber] = useState('')
  const [location, setLocation] = useState('')
  const [jobTitle, setJobTitle] = useState('')
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
    
    if (userType === 'profesional') {
      if (!selectedCompanyId) {
        setError('Debes seleccionar una empresa para registrarte como profesional')
        return
      }
      if (!phoneNumber.trim()) {
        setError('El teléfono es obligatorio')
        return
      }
      if (!location.trim()) {
        setError('La ubicación es obligatoria')
        return
      }
      if (!jobTitle.trim()) {
        setError('El rol o cargo es obligatorio')
        return
      }
    }

    if (userType === 'empleador') {
      if (!companyName.trim()) {
        setError('El nombre de la empresa es obligatorio')
        return
      }
      if (!name.trim()) {
        setError('El nombre del dueño es obligatorio')
        return
      }
      if (!phoneNumber.trim()) {
        setError('El teléfono es obligatorio')
        return
      }
      if (!location.trim()) {
        setError('La ubicación es obligatoria')
        return
      }
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
        phone_number: phoneNumber,
        location: location,
        job_title: userType === 'profesional' ? jobTitle : undefined,
      })
      navigate('/dashboard')
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Error al registrarse')
    } finally {
      setIsLoading(false)
    }
  }

  return (
        <AuthLayout 
          title="Crear Cuenta" 
          subtitle="Únete a la plataforma de gestión de equipos" 
          isRegister={true}
        >
          {error && <div className={styles['error-message']}>{error}</div>}

          <form onSubmit={handleSubmit}>
            <div className={styles['form-group']}>
              <label htmlFor="name">
                {userType === 'empleador' ? 'Nombre del dueño / Administrador' : 'Nombre completo'}
              </label>
              <input
                id="name"
                type="text"
                placeholder={userType === 'empleador' ? 'Ej: Juan Pérez' : 'Juan Pérez'}
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
              <>
                <div className={styles['form-group']}>
                  <label htmlFor="jobTitle">Rol / Cargo (Ej: Desarrollador Backend, Diseñador UI...)</label>
                  <input
                    id="jobTitle"
                    type="text"
                    placeholder="Ej: Desarrollador Fullstack"
                    value={jobTitle}
                    onChange={(e) => setJobTitle(e.target.value)}
                    required
                  />
                </div>

                <div className={styles['form-group']}>
                  <label htmlFor="phoneNumber">Teléfono de contacto</label>
                  <input
                    id="phoneNumber"
                    type="tel"
                    placeholder="Ej: +34 600 000 000"
                    value={phoneNumber}
                    onChange={(e) => setPhoneNumber(e.target.value)}
                    required
                  />
                </div>

                <div className={styles['form-group']}>
                  <label htmlFor="location">Ubicación (Ciudad, País)</label>
                  <input
                    id="location"
                    type="text"
                    placeholder="Ej: Madrid, España"
                    value={location}
                    onChange={(e) => setLocation(e.target.value)}
                    required
                  />
                </div>

                <div className={styles['form-group']}>
                  <label htmlFor="companySelect">Empresa a la que perteneces (Cargadas: {companies.length})</label>
                  <select
                    id="companySelect"
                    value={selectedCompanyId}
                    onChange={(e) => setSelectedCompanyId(Number(e.target.value) || '')}
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
              </>
            )}

            {userType === 'empleador' && (
              <>
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

                <div className={styles['form-group']}>
                  <label htmlFor="phoneNumberCompany">Teléfono de contacto de la empresa</label>
                  <input
                    id="phoneNumberCompany"
                    type="tel"
                    placeholder="Ej: +34 600 000 000"
                    value={phoneNumber}
                    onChange={(e) => setPhoneNumber(e.target.value)}
                    required
                  />
                </div>

                <div className={styles['form-group']}>
                  <label htmlFor="locationCompany">Ubicación de la empresa (Ciudad, País)</label>
                  <input
                    id="locationCompany"
                    type="text"
                    placeholder="Ej: Madrid, España"
                    value={location}
                    onChange={(e) => setLocation(e.target.value)}
                    required
                  />
                </div>
              </>
            )}

            <button type="submit" className={styles['btn-primary']} disabled={isLoading}>
              {isLoading ? 'Creando cuenta...' : 'Registrarse'}
            </button>
          </form>

          <p className={styles['auth-link']}>
            ¿Ya tienes cuenta? <a href="/login">Inicia sesión</a>
          </p>
        </AuthLayout>
  )
}
