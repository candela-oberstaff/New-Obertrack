import React from 'react'

interface EmployerFieldsProps {
  companyName: string
  setCompanyName: (val: string) => void
  phoneNumber: string
  setPhoneNumber: (val: string) => void
  location: string
  setLocation: (val: string) => void
  styles: Record<string, string>
}

export const EmployerFields: React.FC<EmployerFieldsProps> = ({
  companyName,
  setCompanyName,
  phoneNumber,
  setPhoneNumber,
  location,
  setLocation,
  styles,
}) => {
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
  )
}
