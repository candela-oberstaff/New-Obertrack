import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { authService } from '../services/api'
import styles from './Auth.module.css'
import AuthLayout from '../components/layout/AuthLayout'
import { ProfessionalFields } from '../components/Auth/ProfessionalFields'
import { EmployerFields } from '../components/Auth/EmployerFields'
import { Select } from '../components/ui/Select'

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
                placeholder="Min. 8 caracteres con letras y números"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                minLength={8}
              />
            </div>

            <div className={styles['form-group']}>
              <label htmlFor="userType">Tipo de usuario</label>
              <Select
                fullWidth
                id="userType"
                value={userType}
                onChange={(v) => setUserType(String(v))}
                options={[
                  { value: 'profesional', label: 'Profesional (Profesional que presta servicios)' },
                  { value: 'empleador', label: 'Empresa (Dueño o Administrador de empresa)' },
                  { value: 'customer_success', label: 'Customer Success (Gestión de soporte)' },
                  { value: 'superadmin', label: 'Super Administrador (Control Total)' },
                ]}
              />
            </div>

            {userType === 'profesional' && (
              <ProfessionalFields
                jobTitle={jobTitle}
                setJobTitle={setJobTitle}
                phoneNumber={phoneNumber}
                setPhoneNumber={setPhoneNumber}
                location={location}
                setLocation={setLocation}
                selectedCompanyId={selectedCompanyId}
                setSelectedCompanyId={setSelectedCompanyId}
                companies={companies}
                styles={styles}
              />
            )}

            {userType === 'empleador' && (
              <EmployerFields
                companyName={companyName}
                setCompanyName={setCompanyName}
                phoneNumber={phoneNumber}
                setPhoneNumber={setPhoneNumber}
                location={location}
                setLocation={setLocation}
                styles={styles}
              />
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
