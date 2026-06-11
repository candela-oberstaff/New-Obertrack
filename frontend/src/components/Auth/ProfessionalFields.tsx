import React from 'react'
import { Select } from '../ui/Select'
import { ALL_COUNTRY_OPTIONS, getStatesForCountry } from './countries'

interface ProfessionalFieldsProps {
  jobTitle: string
  setJobTitle: (val: string) => void
  phoneNumber: string
  setPhoneNumber: (val: string) => void
  country: string
  setCountry: (val: string) => void
  province: string
  setProvince: (val: string) => void
  city: string
  setCity: (val: string) => void
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
  province,
  setProvince,
  city,
  setCity,
  location,
  setLocation,
  selectedCompanyId,
  setSelectedCompanyId,
  companies,
  styles,
}) => {
  const states = getStatesForCountry(country)
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
        <label htmlFor="country">País</label>
        <Select
          fullWidth
          required
          id="country"
          value={country}
          onChange={(v) => {
            setCountry(String(v))
            setProvince('')
          }}
          placeholder="Selecciona un país..."
          options={ALL_COUNTRY_OPTIONS}
        />
      </div>

      {states.length > 0 && (
        <div className={styles['form-group']}>
          <label htmlFor="stateProf">Estado / Provincia</label>
          <Select
            fullWidth
            id="stateProf"
            value={province}
            onChange={(v) => setProvince(String(v))}
            placeholder="Selecciona un estado..."
            options={states}
          />
        </div>
      )}

      <div className={styles['form-group']}>
        <label htmlFor="cityProf">Ciudad (opcional)</label>
        <input
          id="cityProf"
          type="text"
          placeholder="Ej: Buenos Aires"
          value={city}
          onChange={(e) => setCity(e.target.value)}
        />
      </div>

      <div className={styles['form-group']}>
        <label htmlFor="locationProf">Ubicación (opcional)</label>
        <input
          id="locationProf"
          type="text"
          placeholder="Ej: Ciudad, provincia o región"
          value={location}
          onChange={(e) => setLocation(e.target.value)}
        />
      </div>

      <div className={styles['form-group']}>
        <label htmlFor="companySelect">Empresa a la que perteneces (Cargadas: {companies.length})</label>
        <Select
          fullWidth
          required
          id="companySelect"
          value={selectedCompanyId}
          onChange={(v) => setSelectedCompanyId(Number(v) || '')}
          placeholder="Selecciona una empresa..."
          options={companies.map((c) => ({ value: c.id, label: c.name }))}
        />
        {companies.length === 0 && (
          <p className={styles['field-hint']}>
            Si no ves tu empresa, asegúrate de que el administrador de la misma ya haya creado una cuenta.
          </p>
        )}
      </div>
    </>
  )
}
