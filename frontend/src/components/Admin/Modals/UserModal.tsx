import { User } from '../../../types'
import { Select } from '../../ui/Select'
import { Modal, Button } from '../../ui'
import { COUNTRY_OPTIONS, getStatesForCountry } from '../../Auth/countries'
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
  error?: string | null
  /** Modo empleador: oculta tipo de usuario, empresa y contraseña (el alta genera
   *  una temporal). Fuerza profesional + su empresa fuera de este componente. */
  employerMode?: boolean
  /** Texto del botón de envío (por defecto Crear Usuario / Guardar Cambios). */
  submitLabel?: string
  /** Estado de carga del botón de envío. */
  busy?: boolean
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
  setNewPassword,
  error,
  employerMode = false,
  submitLabel,
  busy = false,
}: UserModalProps) {
  return (
    <Modal
      isOpen
      onClose={onClose}
      title={title}
      size="md"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Cancelar</Button>
          <Button type="submit" form="user-form" loading={busy}>
            {submitLabel ?? (mode === 'create' ? 'Crear Usuario' : 'Guardar Cambios')}
          </Button>
        </>
      }
    >
      <form id="user-form" onSubmit={onSubmit} className={styles['user-form']}>
        <div className={styles['form-group']}>
          <label>Nombre</label>
          <input
            type="text"
            value={form.name}
            onChange={e => setForm({ ...form, name: e.target.value })}
            required
            autoFocus
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

        {mode === 'create' && !employerMode && (
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

        {!employerMode && (
          <div className={styles['form-group']}>
            <label>Tipo de Usuario (rol)</label>
            <Select
              fullWidth
              value={form.user_type}
              onChange={v => setForm({ ...form, user_type: String(v) })}
              options={[
                { value: 'profesional', label: 'Profesional' },
                { value: 'empleador', label: 'Empresa' },
                { value: 'customer_success', label: 'Customer Success' },
                { value: 'analista_it', label: 'Analista de IT' },
                { value: 'superadmin', label: 'Super Administrador' },
              ]}
            />
          </div>
        )}

        {!employerMode && form.user_type === 'empleador' && (
          <div className={styles['form-group']}>
            <label>Nombre de Empresa</label>
            <input
              type="text"
              value={form.company_name}
              onChange={e => setForm({ ...form, company_name: e.target.value })}
            />
          </div>
        )}

        {!employerMode && (form.user_type === 'profesional' || form.user_type === 'customer_success') && (
          <div className={styles['form-group']}>
            <label>Empresa</label>
            <Select
              fullWidth
              clearable
              placeholder="Seleccionar empresa..."
              value={form.empleador_id || ''}
              onChange={v => setForm({ ...form, empleador_id: v ? Number(v) : undefined })}
              options={employers.map(emp => ({ value: emp.id, label: emp.company_name || emp.name }))}
            />
          </div>
        )}

        {form.user_type === 'profesional' && (
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
            <div className={styles['section-label']}>Contacto y ubicación</div>
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
                <Select
                  fullWidth
                  clearable
                  value={form.country || ''}
                  onChange={v => setForm({ ...form, country: v ? String(v) : '', state: '' })}
                  placeholder="Seleccionar país..."
                  options={form.country && !COUNTRY_OPTIONS.some(o => o.value === form.country)
                    ? [{ value: form.country, label: form.country }, ...COUNTRY_OPTIONS]
                    : COUNTRY_OPTIONS}
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
              <label>Provincia / Estado</label>
              {getStatesForCountry(form.country).length > 0 ? (
                <Select
                  fullWidth
                  clearable
                  value={form.state || ''}
                  onChange={v => setForm({ ...form, state: v ? String(v) : '' })}
                  placeholder="Seleccionar provincia/estado..."
                  options={getStatesForCountry(form.country)}
                />
              ) : (
                <input
                  type="text"
                  value={form.state || ''}
                  onChange={e => setForm({ ...form, state: e.target.value })}
                />
              )}
            </div>
            <div className={styles['form-group']}>
              <label>Ubicación</label>
              <input
                type="text"
                value={form.location}
                onChange={e => setForm({ ...form, location: e.target.value })}
              />
            </div>

            <div className={styles['section-label']}>Permisos</div>
            <div className={styles['permissions-group']}>
              <label className={styles['checkbox-label']}>
                <span>Es Gerente/Manager</span>
                <input
                  type="checkbox"
                  checked={form.is_manager}
                  onChange={e => setForm({ ...form, is_manager: e.target.checked })}
                />
              </label>
              <label className={styles['checkbox-label']}>
                <span>Usuario Activo</span>
                <input
                  type="checkbox"
                  checked={form.is_active}
                  onChange={e => setForm({ ...form, is_active: e.target.checked })}
                />
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
                  <Button type="button" onClick={onResetPassword} disabled={!newPassword}>Cambiar</Button>
                </div>
              </div>
            )}
          </>
        )}

        {error && (
          <div style={{ color: '#dc2626', fontWeight: 600, fontSize: '0.85rem' }}>{error}</div>
        )}
      </form>
    </Modal>
  )
}
