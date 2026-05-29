import React from 'react'
import PhoneInput from 'react-phone-number-input'
import Select from 'react-select'
import { COUNTRIES } from '../../constants/countries' // Ajusta la ruta según tu proyecto
import 'react-phone-number-input/style.css'

interface EmployerFieldsProps {
  companyName: string
  setCompanyName: (val: string) => void
  phoneNumber: string
  setPhoneNumber: (val: string) => void
  country: string
  setCountry: (val: string) => void
  location: string
  setLocation: (val: string) => void
  specialization: string
  setSpecialization: (val: string) => void
  styles: Record<string, string>
}

export const EmployerFields: React.FC<EmployerFieldsProps> = ({
  companyName,
  setCompanyName,
  phoneNumber,
  setPhoneNumber,
  country,
  setCountry,
  location,
  setLocation,
  specialization,
  setSpecialization,
  styles,
}) => {
  const currentCountryOption = COUNTRIES.find(c => c.value === country) || null

  return (
    <>
      <div className={styles['form-group']}>
        <label htmlFor="companyName">Nombre de la empresa</label>
        <input
          id="companyName"
          type="text"
          placeholder="Ej: ACME Corp"
          value={companyName}
          onChange={(e) => setCompanyName(e.target.value)}
          required
        />
      </div>

      <div className={styles['form-group']}>
        <label htmlFor="phoneNumberCompany">Teléfono de contacto de la empresa</label>
        <PhoneInput
          international
          defaultCountry="AR"
          value={phoneNumber}
          onChange={(val) => setPhoneNumber(val || '')}
          placeholder="Ej: 600 000 000"
          id="phoneNumberCompany"
          required
        />
      </div>

      <div className={styles['form-group']}>
        <label htmlFor="countryCompany">País de origen de la empresa</label>
        <Select
          id="countryCompany"
          options={COUNTRIES}
          value={currentCountryOption}
          onChange={(option) => setCountry(option?.value || '')}
          placeholder="Busca y selecciona el país de la empresa..."
          isSearchable
          noOptionsMessage={() => "No se encontraron países"}
          classNamePrefix="react-select"
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

      <div className={styles['form-group']}>
        <label htmlFor="specialization">Área de especialización / actividad</label>
        <input
          id="specialization"
          type="text"
          placeholder="Ej: Servicios financieros, Desarrollo de software..."
          value={specialization}
          onChange={(e) => setSpecialization(e.target.value)}
          required
        />
      </div>
    </>
  )
}