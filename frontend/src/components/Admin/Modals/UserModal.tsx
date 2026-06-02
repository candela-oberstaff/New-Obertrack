import { User } from '../../../types'
import { X } from 'lucide-react'
import { Select } from '../../ui/Select'
import styles from '../Admin.module.css'

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
    <div className={styles['modal-overlay']} onClick={onClose}>
      <div className={styles['modal']} onClick={e => e.stopPropagation()}>
        <div className={styles['modal-header']}>
          <h2>{title}</h2>
          <button className={styles['close-btn']} onClick={onClose}><X size={20} /></button>
        </div>
        <form onSubmit={onSubmit}>
          <div className={styles['form-group']}>
            <label>Nombre</label>
            <input
              type="text"
              value={form.name}
              onChange={e => setForm({ ...form, name: e.target.value })}
              required
            />
          </div>
          <div className={styles['form-group']}>
            <label>Email</label>
            <input
              type="email"
              value={form.email}
              onChange={e => setForm({ ...form, email: e.target.value })}
              required
            />
          </div>
          
          {mode === 'create' && (
            <div className={styles['form-group']}>
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
            <div className={styles['form-group']}>
              <label>Tipo de Usuario</label>
              <Select
                fullWidth
                value={form.user_type}
                onChange={v => setForm({ ...form, user_type: String(v) })}
                options={[
                  { value: 'profesional', label: 'Profesional' },
                  { value: 'empleador', label: 'Empresa' },
                ]}
              />
            </div>
          )}

          {(mode === 'edit' || form.user_type === 'empleador') && (
            <div className={styles['form-group']}>
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
              <div className={styles['form-group']}>
                <label>Empresa (Empleador)</label>
                <Select
                  fullWidth
                  clearable
                  placeholder="Seleccionar empresa..."
                  value={form.empleador_id || ''}
                  onChange={v => setForm({ ...form, empleador_id: v ? Number(v) : undefined })}
                  options={employers.map(emp => ({ value: emp.id, label: emp.company_name || emp.name }))}
                />
              </div>
              <div className={styles['form-group']}>
                <label>Manager Asignado</label>
                <Select
                  fullWidth
                  clearable
                  placeholder="Sin manager asignado"
                  value={form.manager_id || ''}
                  onChange={v => setForm({ ...form, manager_id: v ? Number(v) : undefined })}
                  options={managers.filter(m => m.id !== form.empleador_id).map(m => ({ value: m.id, label: m.name }))}
                />
              </div>
            </>
          )}

          <div className={styles['form-group']}>
            <label>Puesto</label>
            <input
              type="text"
              value={form.job_title}
              onChange={e => setForm({ ...form, job_title: e.target.value })}
            />
          </div>

          {mode === 'edit' && (
            <>
              <div className={styles['form-group']}>
                <label>Teléfono</label>
                <input
                  type="text"
                  value={form.phone_number}
                  onChange={e => setForm({ ...form, phone_number: e.target.value })}
                />
              </div>
              <div className={styles['form-row']}>
                <div className={styles['form-group']}>
                  <label>País</label>
                  <input
                    type="text"
                    value={form.country}
                    onChange={e => setForm({ ...form, country: e.target.value })}
                  />
                </div>
                <div className={styles['form-group']}>
                  <label>Ciudad</label>
                  <input
                    type="text"
                    value={form.city}
                    onChange={e => setForm({ ...form, city: e.target.value })}
                  />
                </div>
              </div>
              <div className={styles['form-group']}>
                <label className={styles['checkbox-label']}>
                  <input
                    type="checkbox"
                    checked={form.is_manager}
                    onChange={e => setForm({ ...form, is_manager: e.target.checked })}
                  />
                  Es Gerente/Manager
                </label>
              </div>
              <div className={styles['form-group']}>
                <label className={styles['checkbox-label']}>
                  <input
                    type="checkbox"
                    checked={form.is_active}
                    onChange={e => setForm({ ...form, is_active: e.target.checked })}
                  />
                  Usuario Activo
                </label>
              </div>
              
              {onResetPassword && setNewPassword && (
                <div className={styles['form-group']}>
                  <label>Restablecer Contraseña</label>
                  <div className={styles['password-reset-row']}>
                    <input
                      type="password"
                      placeholder="Nueva contraseña"
                      value={newPassword}
                      onChange={e => setNewPassword(e.target.value)}
                      className={styles['password-input']}
                    />
                    <button
                      type="button"
                      className={styles['btn-primary']}
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

          <div className={styles['modal-actions']}>
            <button type="button" className={styles['btn-secondary']} onClick={onClose}>
              Cancelar
            </button>
            <button type="submit" className={styles['btn-primary']}>
              {mode === 'create' ? 'Crear Usuario' : 'Guardar Cambios'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
