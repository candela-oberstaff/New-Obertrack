import type { Dispatch, SetStateAction } from 'react'
import { UserPlus, X } from 'lucide-react'
import { Select } from '../../ui/Select'
import { COUNTRY_OPTIONS, getStatesForCountry } from '../../Auth/countries'
import styles from '../Admin.module.css'

export interface CreateUserForm {
  name: string
  email: string
  password: string
  userType: string
  jobTitle: string
  phoneNumber: string
  selectedCompanyId: number | ''
  managerId: number | ''
  companyName: string
  industry: string
  country: string
  province: string
  city: string
  location: string
  address: string
}

interface CreateUserModalProps {
  form: CreateUserForm
  setForm: Dispatch<SetStateAction<CreateUserForm>>
  onClose: () => void
  onSubmit: (e: React.FormEvent) => void
  loading?: boolean
  error?: string
  publicCompanies: { id: number; name: string }[]
  managers?: { id: number; name: string }[]
  employerMode?: boolean
}

const inputStyle: React.CSSProperties = {
  width: '100%', padding: '11px 14px', border: '1px solid #cbd5e1',
  borderRadius: '10px', fontSize: '14px', color: '#0f172a', background: '#f8fafc',
}
const labelStyle: React.CSSProperties = {
  display: 'block', marginBottom: '7px', fontWeight: 600, fontSize: '13px', color: '#334155',
}

export function CreateUserModal({
  form, setForm, onClose, onSubmit, loading = false, error,
  publicCompanies, managers = [], employerMode = false,
}: CreateUserModalProps) {
  return (
    <div
      className={styles['modal-overlay']}
      onClick={onClose}
      style={{ position: 'fixed', inset: 0, zIndex: 1000, display: 'flex', alignItems: 'center', justifyContent: 'center' }}
    >
      <div
        onClick={e => e.stopPropagation()}
        style={{
          background: '#fff', borderRadius: '20px', padding: '32px', width: '100%',
          maxWidth: '560px', maxHeight: '90vh', overflowY: 'auto',
          boxShadow: '0 25px 50px -12px rgba(0,0,0,0.25)', border: '1px solid rgba(0,0,0,0.1)',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '24px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <div style={{ width: 40, height: 40, borderRadius: 12, background: 'linear-gradient(135deg, var(--primary), var(--primary-dark, #1d4ed8))', display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#fff' }}>
              <UserPlus size={20} />
            </div>
            <div>
              <h2 style={{ margin: 0, fontSize: '20px', fontWeight: 800, color: '#0f172a' }}>
                {employerMode ? 'Crear profesional' : 'Crear Usuario'}
              </h2>
              <p style={{ margin: 0, fontSize: '13px', color: '#64748b' }}>Completa todos los campos requeridos</p>
            </div>
          </div>
          <button onClick={onClose} style={{ background: 'none', border: 'none', cursor: 'pointer', color: '#94a3b8', padding: 4 }}>
            <X size={22} />
          </button>
        </div>

        {error && (
          <div style={{ background: 'rgba(239, 68, 68, 0.12)', border: '1px solid rgba(239, 68, 68, 0.3)', color: '#dc2626', padding: '11px 14px', borderRadius: '10px', marginBottom: '18px', fontSize: '13px', fontWeight: 500 }}>
            {error}
          </div>
        )}

        <form onSubmit={onSubmit}>
          <div className={styles['form-group']}>
            <label style={labelStyle}>
              {form.userType === 'empleador' ? 'Nombre del dueño / Administrador' : 'Nombre completo'}
            </label>
            <input
              type="text"
              placeholder={form.userType === 'empleador' ? 'Ej: Juan Pérez' : 'Juan Pérez'}
              value={form.name}
              onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
              required
              style={inputStyle}
            />
          </div>

          <div className={styles['form-group']} style={{ marginTop: '16px' }}>
            <label style={labelStyle}>Email</label>
            <input
              type="email"
              placeholder="juan@ejemplo.com"
              value={form.email}
              onChange={e => setForm(f => ({ ...f, email: e.target.value }))}
              required
              style={inputStyle}
            />
          </div>

          {!employerMode && (
            <div className={styles['form-group']} style={{ marginTop: '16px' }}>
              <label style={labelStyle}>Contraseña</label>
              <input
                type="password"
                placeholder="Min. 8 caracteres con letras y números"
                value={form.password}
                onChange={e => setForm(f => ({ ...f, password: e.target.value }))}
                required
                minLength={8}
                style={inputStyle}
              />
            </div>
          )}

          {!employerMode && (
            <div className={styles['form-group']} style={{ marginTop: '16px' }}>
              <label style={labelStyle}>Tipo de usuario</label>
              <Select
                fullWidth
                value={form.userType}
                onChange={v => setForm(f => ({ ...f, userType: String(v), companyName: '', industry: '', selectedCompanyId: '', phoneNumber: '', country: '', province: '', city: '', location: '', address: '', jobTitle: '' }))}
                options={[
                  { value: 'profesional', label: 'Profesional (presta servicios)' },
                  { value: 'empleador', label: 'Empresa' },
                  { value: 'customer_success', label: 'Customer Success (Gestión de soporte)' },
                  { value: 'analista_it', label: 'Analista de IT (Soporte técnico)' },
                  { value: 'superadmin', label: 'Super Administrador (Control Total)' },
                ]}
              />
            </div>
          )}

          {form.userType === 'profesional' && (
            <div style={{ marginTop: '16px', display: 'flex', flexDirection: 'column', gap: '16px' }}>
              <div>
                <label style={labelStyle}>Rol / Cargo (Ej: Desarrollador Backend...)</label>
                <input
                  type="text"
                  placeholder="Ej: Desarrollador Fullstack"
                  value={form.jobTitle}
                  onChange={e => setForm(f => ({ ...f, jobTitle: e.target.value }))}
                  required={!employerMode}
                  style={inputStyle}
                />
              </div>

              <div>
                <label style={labelStyle}>Teléfono de contacto</label>
                <input
                  type="tel"
                  placeholder="Ej: +34 600 000 000"
                  value={form.phoneNumber}
                  onChange={e => setForm(f => ({ ...f, phoneNumber: e.target.value }))}
                  required={!employerMode}
                  style={inputStyle}
                />
              </div>

              {employerMode ? (
                <div>
                  <label style={labelStyle}>Manager asignado (opcional)</label>
                  <Select
                    fullWidth
                    clearable
                    value={form.managerId}
                    onChange={v => setForm(f => ({ ...f, managerId: Number(v) || '' }))}
                    placeholder="Sin manager asignado"
                    options={managers.map(m => ({ value: m.id, label: m.name }))}
                  />
                </div>
              ) : (
                <div>
                  <label style={labelStyle}>Empresa a la que perteneces</label>
                  <Select
                    fullWidth
                    value={form.selectedCompanyId}
                    onChange={v => setForm(f => ({ ...f, selectedCompanyId: Number(v) || '' }))}
                    placeholder="Selecciona una empresa..."
                    options={publicCompanies.map(c => ({ value: c.id, label: c.name }))}
                  />
                </div>
              )}
            </div>
          )}

          {!employerMode && form.userType === 'empleador' && (
            <div style={{ marginTop: '16px', display: 'flex', flexDirection: 'column', gap: '16px' }}>
              <div>
                <label style={labelStyle}>Nombre de tu empresa</label>
                <input
                  type="text"
                  placeholder="Mi Empresa S.A."
                  value={form.companyName}
                  onChange={e => setForm(f => ({ ...f, companyName: e.target.value }))}
                  required
                  style={inputStyle}
                />
              </div>
              <div>
                <label style={labelStyle}>Rubro o industria</label>
                <input
                  type="text"
                  placeholder="Ej: Tecnología, Marketing..."
                  value={form.industry}
                  onChange={e => setForm(f => ({ ...f, industry: e.target.value }))}
                  required
                  style={inputStyle}
                />
              </div>
              <div>
                <label style={labelStyle}>Teléfono de contacto de la empresa</label>
                <input
                  type="tel"
                  placeholder="Ej: +34 600 000 000"
                  value={form.phoneNumber}
                  onChange={e => setForm(f => ({ ...f, phoneNumber: e.target.value }))}
                  required
                  style={inputStyle}
                />
              </div>
            </div>
          )}

          {form.userType !== 'superadmin' && (
            <div style={{ marginTop: '16px', display: 'flex', flexDirection: 'column', gap: '16px' }}>
              <div>
                <label style={labelStyle}>País</label>
                <Select
                  fullWidth
                  value={form.country}
                  onChange={v => setForm(f => ({ ...f, country: String(v), province: '' }))}
                  placeholder="Selecciona un país..."
                  options={COUNTRY_OPTIONS}
                />
              </div>
              {getStatesForCountry(form.country).length > 0 && (
                <div>
                  <label style={labelStyle}>Estado / Provincia</label>
                  <Select
                    fullWidth
                    value={form.province}
                    onChange={v => setForm(f => ({ ...f, province: String(v) }))}
                    placeholder="Selecciona un estado..."
                    options={getStatesForCountry(form.country)}
                  />
                </div>
              )}
              <div>
                <label style={labelStyle}>Ciudad (opcional)</label>
                <input
                  type="text"
                  placeholder="Ej: Buenos Aires"
                  value={form.city}
                  onChange={e => setForm(f => ({ ...f, city: e.target.value }))}
                  style={inputStyle}
                />
              </div>
              <div>
                <label style={labelStyle}>Ubicación (opcional)</label>
                <input
                  type="text"
                  placeholder="Ej: Ciudad, provincia o región"
                  value={form.location}
                  onChange={e => setForm(f => ({ ...f, location: e.target.value }))}
                  style={inputStyle}
                />
              </div>
            </div>
          )}

          {!employerMode && form.userType === 'customer_success' && (
            <div className={styles['form-group']} style={{ marginTop: '16px' }}>
              <label style={labelStyle}>Empresa asignada (opcional)</label>
              <Select
                fullWidth
                clearable
                value={form.selectedCompanyId}
                onChange={v => setForm(f => ({ ...f, selectedCompanyId: Number(v) || '' }))}
                placeholder="Selecciona una empresa..."
                options={publicCompanies.map(c => ({ value: c.id, label: c.name }))}
              />
              <p style={{ margin: '6px 0 0', fontSize: '12px', color: '#94a3b8' }}>
                Vincula esta cuenta de soporte a una empresa concreta, o déjala vacía para soporte global.
              </p>
            </div>
          )}

          <div style={{ display: 'flex', gap: '12px', marginTop: '24px' }}>
            <button
              type="button"
              onClick={onClose}
              style={{ flex: 1, padding: '12px', borderRadius: '12px', border: '1px solid #e2e8f0', background: '#f8fafc', fontWeight: 600, cursor: 'pointer', color: '#475569' }}
            >
              Cancelar
            </button>
            <button
              type="submit"
              disabled={loading}
              style={{ flex: 1, padding: '12px', borderRadius: '12px', border: 'none', background: 'linear-gradient(135deg, var(--primary), var(--primary-dark, #1d4ed8))', color: '#fff', fontWeight: 700, cursor: loading ? 'not-allowed' : 'pointer', opacity: loading ? 0.7 : 1 }}
            >
              {loading ? 'Creando...' : (employerMode ? 'Crear profesional' : 'Crear Usuario')}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
