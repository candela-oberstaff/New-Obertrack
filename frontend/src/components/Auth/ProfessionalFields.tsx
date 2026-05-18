import React from 'react'

interface ProfessionalFieldsProps {
  jobTitle: string
  setJobTitle: (val: string) => void
  phoneNumber: string
  setPhoneNumber: (val: string) => void
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
  location,
  setLocation,
  selectedCompanyId,
  setSelectedCompanyId,
  companies,
  styles,
}) => {
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
  )
}
