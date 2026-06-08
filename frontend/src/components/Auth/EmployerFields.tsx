import React from 'react'
import { Select } from '../ui/Select'
import { COUNTRY_OPTIONS, getStatesForCountry } from './countries'
import { INDUSTRY_OPTIONS } from './industries'

interface EmployerFieldsProps {
  companyName: string
  setCompanyName: (val: string) => void
  industry: string
  setIndustry: (val: string) => void
  phoneNumber: string
  setPhoneNumber: (val: string) => void
  country: string
  setCountry: (val: string) => void
  province: string
  setProvince: (val: string) => void
  location: string
  setLocation: (val: string) => void
  address: string
  setAddress: (val: string) => void
  styles: Record<string, string>
}

export const EmployerFields: React.FC<EmployerFieldsProps> = ({
  companyName,
  setCompanyName,
  industry,
  setIndustry,
  phoneNumber,
  setPhoneNumber,
  country,
  setCountry,
  province,
  setProvince,
  location,
  setLocation,
  address,
  setAddress,
  styles,
}) => {
  const states = getStatesForCountry(country)
  return (
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
        <label htmlFor="industryCompany">Rubro o industria</label>
        <Select
          fullWidth
          required
          id="industryCompany"
          value={industry}
          onChange={(v) => setIndustry(String(v))}
          placeholder="Selecciona un rubro..."
          options={INDUSTRY_OPTIONS}
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
        <label htmlFor="countryCompany">País</label>
        <Select
          fullWidth
          required
          id="countryCompany"
          value={country}
          onChange={(v) => {
            setCountry(String(v))
            setProvince('')
          }}
          placeholder="Selecciona un país..."
          options={COUNTRY_OPTIONS}
        />
      </div>

      {states.length > 0 && (
        <div className={styles['form-group']}>
          <label htmlFor="stateCompany">Estado / Provincia</label>
          <Select
            fullWidth
            id="stateCompany"
            value={province}
            onChange={(v) => setProvince(String(v))}
            placeholder="Selecciona un estado..."
            options={states}
          />
        </div>
      )}

      <div className={styles['form-group']}>
        <label htmlFor="locationCompany">Ubicación (opcional)</label>
        <input
          id="locationCompany"
          type="text"
          placeholder="Ej: Ciudad, provincia o región"
          value={location}
          onChange={(e) => setLocation(e.target.value)}
        />
      </div>

      <div className={styles['form-group']}>
        <label htmlFor="addressCompany">Dirección (opcional)</label>
        <input
          id="addressCompany"
          type="text"
          placeholder="Ej: Calle, número, piso..."
          value={address}
          onChange={(e) => setAddress(e.target.value)}
        />
      </div>
    </>
  )
}
