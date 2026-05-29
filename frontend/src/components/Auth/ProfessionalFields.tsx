import React from 'react'
import PhoneInput from 'react-phone-number-input'
import Select from 'react-select'
import { COUNTRIES } from '../../constants/countries' // Ajusta la ruta según tu proyecto
import 'react-phone-number-input/style.css'

interface ProfessionalFieldsProps {
  jobTitle: string
  setJobTitle: (val: string) => void
  phoneNumber: string
  setPhoneNumber: (val: string) => void
  country: string
  setCountry: (val: string) => void
  location: string
  setLocation: (val: string) => void
  selectedCompanyId: number | ''
  setSelectedCompanyId: (val: number | '') => void
  companies: { id: number; name: string }[]
  styles: Record<string, string>
}

export const ProfessionalFields: React.FC<ProfessionalFieldsProps> = ({
  jobTitle,
  setJobTitle,
  phoneNumber,
  setPhoneNumber,
  country,
  setCountry,
  location,
  setLocation,
  selectedCompanyId,
  setSelectedCompanyId,
  companies,
  styles,
}) => {
  // Encontramos el objeto actual para que react-select muestre el valor correcto
  const currentCountryOption = COUNTRIES.find(c => c.value === country) || null

  return (
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
        <PhoneInput
          international
          defaultCountry="AR"
          value={phoneNumber}
          onChange={(val) => setPhoneNumber(val || '')}
          placeholder="Ej: 600 000 000"
          id="phoneNumber"
          required
        />
      </div>

      <div className={styles['form-group']}>
        <label htmlFor="country">País</label>
        <Select
          id="country"
          options={COUNTRIES}
          value={currentCountryOption}
          onChange={(option) => setCountry(option?.value || '')}
          placeholder="Busca y selecciona tu país..."
          isSearchable
          noOptionsMessage={() => "No se encontraron países"}
          classNamePrefix="react-select" // Permite estilizarlo desde CSS si lo requieres
        />
      </div>

      <div className={styles['form-group']}>
        <label htmlFor="location">Ubicación (Ciudad, Provincia/Estado)</label>
        <input
          id="location"
          type="text"
          placeholder="Ej: Buenos Aires, CABA"
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
  )
}