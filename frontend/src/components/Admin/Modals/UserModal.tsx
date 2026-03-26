import { User } from '../../../types'
import { CountryCitySelector } from '../../common/CountryCitySelector'

interface UserModalProps {
  title: string
  mode: 'create' | 'edit'
  form: any
  setForm: (form: any) => void
  employers: User[]
  managers: User[]
  onClose: () => void
  onSubmit: (e: React.FormEvent) => void
  onResetPassword?: () => void
  newPassword?: string
  setNewPassword?: (val: string) => void
}

export function UserModal({
  title,
  mode,
  form,
  setForm,
  employers,
  managers,
  onClose,
  onSubmit,
  onResetPassword,
  newPassword,
  setNewPassword
}: UserModalProps) {
  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h2>{title}</h2>
          <button className="close-btn" onClick={onClose}>✕</button>
        </div>
        <form onSubmit={onSubmit}>
          <div className="form-group">
            <label>Nombre</label>
            <input
              type="text"
              value={form.name}
              onChange={e => setForm({ ...form, name: e.target.value })}
              required
            />
          </div>
          <div className="form-group">
            <label>Email</label>
            <input
              type="email"
              value={form.email}
              onChange={e => setForm({ ...form, email: e.target.value })}
              required
            />
          </div>
          
          {mode === 'create' && (
            <div className="form-group">
              <label>Contraseña</label>
              <input
                type="password"
                value={form.password}
                onChange={e => setForm({ ...form, password: e.target.value })}
                required
              />
            </div>
          )}

          {mode === 'create' && (
            <div className="form-group">
              <label>Tipo de Usuario</label>
              <select
                value={form.user_type}
                onChange={e => setForm({ ...form, user_type: e.target.value })}
              >
                <option value="profesional">Profesional</option>
                <option value="empleador">Empresa</option>
              </select>
            </div>
          )}

          {(mode === 'edit' || form.user_type === 'empleador') && (
            <div className="form-group">
              <label>Nombre de Empresa</label>
              <input
                type="text"
                value={form.company_name}
                onChange={e => setForm({ ...form, company_name: e.target.value })}
              />
            </div>
          )}

          {(mode === 'edit' || form.user_type === 'profesional') && (
            <>
              <div className="form-group">
                <label>Empresa (Empleador)</label>
                <select
                  value={form.empleador_id || ''}
                  onChange={e => setForm({ ...form, empleador_id: e.target.value ? Number(e.target.value) : undefined })}
                >
                  <option value="">Seleccionar empresa...</option>
                  {employers.map(emp => (
                    <option key={emp.id} value={emp.id}>{emp.company_name || emp.name}</option>
                  ))}
                </select>
              </div>
              <div className="form-group">
                <label>Manager Asignado</label>
                <select
                  value={form.manager_id || ''}
                  onChange={e => setForm({ ...form, manager_id: e.target.value ? Number(e.target.value) : undefined })}
                >
                  <option value="">Sin manager asignado</option>
                  {managers.filter(m => m.id !== form.empleador_id).map(m => (
                    <option key={m.id} value={m.id}>{m.name}</option>
                  ))}
                </select>
              </div>
            </>
          )}

          <div className="form-group">
            <label>Puesto</label>
            <input
              type="text"
              value={form.job_title}
              onChange={e => setForm({ ...form, job_title: e.target.value })}
            />
          </div>

          {mode === 'edit' && (
            <>
              <div className="form-group">
                <label>Teléfono</label>
                <input
                  type="text"
                  value={form.phone_number}
                  onChange={e => setForm({ ...form, phone_number: e.target.value })}
                />
              </div>
              <CountryCitySelector
                countryValue={form.country}
                cityValue={form.city}
                onCountryChange={(val) => setForm({ ...form, country: val, city: '' })}
                onCityChange={(val) => setForm({ ...form, city: val })}
              />
              <div className="form-group">
                <label className="checkbox-label">
                  <input
                    type="checkbox"
                    checked={form.is_manager}
                    onChange={e => setForm({ ...form, is_manager: e.target.checked })}
                  />
                  Es Gerente/Manager
                </label>
              </div>
              <div className="form-group">
                <label className="checkbox-label">
                  <input
                    type="checkbox"
                    checked={form.is_active}
                    onChange={e => setForm({ ...form, is_active: e.target.checked })}
                  />
                  Usuario Activo
                </label>
              </div>
              
              {onResetPassword && setNewPassword && (
                <div className="form-group" style={{ borderTop: '1px solid #e2e8f0', paddingTop: '16px', marginTop: '8px' }}>
                  <label>Restablecer Contraseña</label>
                  <div className="password-reset-row">
                    <input
                      type="password"
                      placeholder="Nueva contraseña"
                      value={newPassword}
                      onChange={e => setNewPassword(e.target.value)}
                      className="password-input"
                    />
                    <button
                      type="button"
                      className="btn-primary"
                      onClick={onResetPassword}
                      disabled={!newPassword}
                    >
                      Cambiar
                    </button>
                  </div>
                </div>
              )}
            </>
          )}

          <div className="modal-actions">
            <button type="button" className="btn-cancel" onClick={onClose}>
              Cancelar
            </button>
            <button type="submit" className="btn-primary">
              {mode === 'create' ? 'Crear Usuario' : 'Guardar Cambios'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
