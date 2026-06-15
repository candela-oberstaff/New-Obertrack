import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { authService } from '../services/api'
import styles from './Auth.module.css'
import AuthLayout from '../components/layout/AuthLayout'
import { ProfessionalFields } from '../components/Auth/ProfessionalFields'
import { EmployerFields } from '../components/Auth/EmployerFields'
import { Select } from '../components/ui/Select'
import { COUNTRY_OPTIONS, getStatesForCountry } from '../components/Auth/countries'

export default function Register() {
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [userType, setUserType] = useState('profesional')
  const [companyName, setCompanyName] = useState('')
  const [industry, setIndustry] = useState('')
  const [selectedCompanyId, setSelectedCompanyId] = useState<number | ''>('')
  const [phoneNumber, setPhoneNumber] = useState('')
  const [country, setCountry] = useState('')
  const [province, setProvince] = useState('')
  const [city, setCity] = useState('')
  const [location, setLocation] = useState('')
  const [address, setAddress] = useState('')
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
      if (!country.trim()) {
        setError('El país es obligatorio')
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
      if (!country.trim()) {
        setError('El país es obligatorio')
        return
      }
      if (!industry.trim()) {
        setError('El rubro o industria es obligatorio')
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
        industry: userType === 'empleador' ? industry : undefined,
        empleador_id:
          userType === 'profesional' || userType === 'customer_success'
            ? (selectedCompanyId as number) || undefined
            : undefined,
        phone_number: phoneNumber,
        country: country,
        state: province || undefined,
        city: city || undefined,
        location: location || undefined,
        address: userType === 'empleador' ? address : undefined,
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
                  { value: 'empleador', label: 'Empresa' },
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
                country={country}
                setCountry={setCountry}
                province={province}
                setProvince={setProvince}
                city={city}
                setCity={setCity}
                location={location}
                setLocation={setLocation}
                selectedCompanyId={selectedCompanyId}
                setSelectedCompanyId={setSelectedCompanyId}
                companies={companies}
                styles={styles}
              />
            )}

            {userType === 'customer_success' && (
              <>
                <div className={styles['form-group']}>
                  <label htmlFor="csCompanySelect">Empresa asignada (opcional)</label>
                  <Select
                    fullWidth
                    clearable
                    id="csCompanySelect"
                    value={selectedCompanyId}
                    onChange={(v) => setSelectedCompanyId(Number(v) || '')}
                    placeholder="Selecciona una empresa..."
                    options={companies.map((c) => ({ value: c.id, label: c.name }))}
                  />
                  <p className={styles['field-hint']}>
                    Vincula esta cuenta de soporte a una empresa concreta, o déjala vacía para soporte global.
                  </p>
                </div>

                <div className={styles['form-group']}>
                  <label htmlFor="csCountry">País</label>
                  <Select
                    fullWidth
                    id="csCountry"
                    value={country}
                    onChange={(v) => { setCountry(String(v)); setProvince('') }}
                    placeholder="Selecciona un país..."
                    options={COUNTRY_OPTIONS}
                  />
                </div>
                {getStatesForCountry(country).length > 0 && (
                  <div className={styles['form-group']}>
                    <label htmlFor="csState">Estado / Provincia</label>
                    <Select
                      fullWidth
                      id="csState"
                      value={province}
                      onChange={(v) => setProvince(String(v))}
                      placeholder="Selecciona un estado..."
                      options={getStatesForCountry(country)}
                    />
                  </div>
                )}
              </>
            )}

            {userType === 'empleador' && (
              <EmployerFields
                companyName={companyName}
                setCompanyName={setCompanyName}
                industry={industry}
                setIndustry={setIndustry}
                phoneNumber={phoneNumber}
                setPhoneNumber={setPhoneNumber}
                country={country}
                setCountry={setCountry}
                province={province}
                setProvince={setProvince}
                city={city}
                setCity={setCity}
                location={location}
                setLocation={setLocation}
                address={address}
                setAddress={setAddress}
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
