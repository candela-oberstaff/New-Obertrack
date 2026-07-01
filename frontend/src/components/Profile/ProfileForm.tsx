import React, { useRef, useState } from 'react'
import { Upload, FileCheck2, Eye, Loader2 } from 'lucide-react'
import { userService, uploadService } from '../../services/api'
import type { User } from '../../types'
import Tooltip from '../Common/Tooltip'
import { Select } from '../ui/Select'
import { COUNTRY_OPTIONS, getStatesForCountry } from '../Auth/countries'
import styles from '../../pages/Profile.module.css'

interface ProfileFormProps {
  user: User
  setUser: (user: User) => void
  isEditing: boolean
  setIsEditing: (val: boolean) => void
}

export function ProfileForm({ user, setUser, isEditing, setIsEditing }: ProfileFormProps) {
  const [isLoading, setIsLoading] = useState(false)
  const [isUploadingDoc, setIsUploadingDoc] = useState(false)
  const [message, setMessage] = useState({ type: '', text: '' })
  const docInputRef = useRef<HTMLInputElement>(null)
  const [formData, setFormData] = useState({
    name: user.name || '',
    phone_number: user.phone_number || '',
    country: user.country || '',
    state: user.state || '',
    city: user.city || '',
    location: user.location || '',
    job_title: user.job_title || '',
    identity_document: user.identity_document || '',
  })

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setIsLoading(true)
    setMessage({ type: '', text: '' })

    try {
      const updatedUser = await userService.update(user.id, formData)
      setUser(updatedUser)
      setMessage({ type: 'success', text: 'Perfil actualizado correctamente' })
      setIsEditing(false)
    } catch (error) {
      setMessage({ type: 'error', text: 'Error al actualizar el perfil' })
      console.error('Error updating profile:', error)
    } finally {
      setIsLoading(false)
    }
  }

  const handleDocUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return

    const allowed = ['application/pdf', 'image/jpeg', 'image/png', 'image/webp', 'image/gif']
    if (!allowed.includes(file.type)) {
      setMessage({ type: 'error', text: 'El documento debe ser un PDF o una imagen' })
      if (docInputRef.current) docInputRef.current.value = ''
      return
    }
    if (file.size > 10 * 1024 * 1024) {
      setMessage({ type: 'error', text: 'El documento debe ser menor a 10MB' })
      if (docInputRef.current) docInputRef.current.value = ''
      return
    }

    setIsUploadingDoc(true)
    setMessage({ type: '', text: '' })
    try {
      const result = await uploadService.upload(file)
      setFormData((prev) => ({ ...prev, identity_document: result.url }))
      setMessage({ type: 'success', text: 'Documento cargado. Guarda los cambios para confirmar.' })
    } catch (error) {
      setMessage({ type: 'error', text: 'Error al subir el documento' })
    } finally {
      setIsUploadingDoc(false)
      if (docInputRef.current) docInputRef.current.value = ''
    }
  }

  const isEmployer = user?.user_type === 'empleador' || user?.is_superadmin || user?.is_manager
  const isProfessional = user?.user_type === 'profesional' || user?.user_type === 'customer_success'

  // Profiles saved before this was a dropdown may hold free text — keep that
  // value selectable instead of silently dropping it.
  const countryOptions = !formData.country || COUNTRY_OPTIONS.some(o => o.value === formData.country)
    ? COUNTRY_OPTIONS
    : [{ value: formData.country, label: formData.country }, ...COUNTRY_OPTIONS]

  const baseStateOptions = getStatesForCountry(formData.country)
  const stateOptions = !formData.state || baseStateOptions.some(o => o.value === formData.state)
    ? baseStateOptions
    : [{ value: formData.state, label: formData.state }, ...baseStateOptions]

  return (
    <div className={styles['info-card']}>
      <div className={styles['card-header']}>
        <h3>
          Información Personal{' '}
          <Tooltip content={isEmployer ? "Registro de tu información personal" : "Registro de tu información personal (es necesaria para el registro de horas)."} size={14} />
        </h3>
      </div>
      
      {message.text && (
        <div className={`${styles['alert']} ${styles[message.type] || message.type}`}>
          {message.text}
        </div>
      )}

      {isEditing ? (
        <form onSubmit={handleSubmit} className={styles['edit-form']}>
          <div className={styles['form-row']}>
            <div className={styles['form-group']}>
              <label>Nombre completo</label>
              <input
                type="text"
                value={formData.name}
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                required
              />
            </div>
            <div className={styles['form-group']}>
              <label>Teléfono</label>
              <input
                type="tel"
                value={formData.phone_number}
                onChange={(e) => setFormData({ ...formData, phone_number: e.target.value })}
              />
            </div>
          </div>

          <div className={styles['form-row']}>
            <div className={styles['form-group']}>
              <label>País</label>
              <Select
                fullWidth
                value={formData.country}
                onChange={(v) => setFormData({ ...formData, country: String(v), state: '' })}
                placeholder="Selecciona un país..."
                options={countryOptions}
              />
            </div>
            <div className={styles['form-group']}>
              <label>Provincia / Estado</label>
              <Select
                fullWidth
                value={formData.state}
                onChange={(v) => setFormData({ ...formData, state: String(v) })}
                placeholder="Selecciona una provincia..."
                options={stateOptions}
                disabled={!formData.country || stateOptions.length === 0}
              />
            </div>
          </div>

          <div className={styles['form-row']}>
            <div className={styles['form-group']}>
              <label>Ciudad</label>
              <input
                type="text"
                value={formData.city}
                onChange={(e) => setFormData({ ...formData, city: e.target.value })}
              />
            </div>
          </div>

          <div className={styles['form-group']}>
            <label>Puesto / Cargo</label>
            <input
              type="text"
              value={formData.job_title}
              onChange={(e) => setFormData({ ...formData, job_title: e.target.value })}
              placeholder="Ej: Desarrollador Frontend"
            />
          </div>

          <div className={styles['form-group']}>
            <label>Dirección</label>
            <textarea
              value={formData.location}
              onChange={(e) => setFormData({ ...formData, location: e.target.value })}
              rows={2}
              placeholder="Dirección completa"
            />
          </div>

          {isProfessional && (
            <div className={styles['form-group']}>
              <label>Documento de identidad</label>
              <input
                ref={docInputRef}
                type="file"
                accept="application/pdf,image/*"
                onChange={handleDocUpload}
                style={{ display: 'none' }}
              />
              {formData.identity_document ? (
                <div className={styles['doc-loaded']}>
                  <FileCheck2 size={22} color="#10b981" style={{ flexShrink: 0 }} />
                  <div className={styles['doc-loaded-info']}>
                    <span className={styles['doc-loaded-title']}>Documento cargado</span>
                    <a
                      href={formData.identity_document}
                      target="_blank"
                      rel="noopener noreferrer"
                      className={styles['doc-loaded-link']}
                    >
                      <Eye size={14} /> Ver documento
                    </a>
                  </div>
                  <button
                    type="button"
                    className={styles['doc-replace-btn']}
                    onClick={() => docInputRef.current?.click()}
                    disabled={isUploadingDoc}
                  >
                    {isUploadingDoc ? 'Subiendo…' : 'Reemplazar'}
                  </button>
                </div>
              ) : (
                <button
                  type="button"
                  className={styles['doc-dropzone']}
                  onClick={() => docInputRef.current?.click()}
                  disabled={isUploadingDoc}
                >
                  {isUploadingDoc ? (
                    <Loader2 size={26} className={styles['doc-spin']} />
                  ) : (
                    <Upload size={26} />
                  )}
                  <span className={styles['doc-dropzone-title']}>
                    {isUploadingDoc ? 'Subiendo documento…' : 'Haz clic para subir tu documento'}
                  </span>
                  <span className={styles['doc-dropzone-hint']}>PDF o imagen, hasta 10MB</span>
                </button>
              )}
            </div>
          )}

          <div className={styles['form-actions']}>
            <button type="submit" className={styles['btn-primary']} disabled={isLoading}>
              {isLoading ? 'Guardando...' : 'Guardar Cambios'}
            </button>
          </div>
        </form>
      ) : (
        <div className={styles['info-list']}>
          <div className={styles['info-item']}>
            <span className={styles['info-label']}>Email</span>
            <span className={styles['info-value']}>{user.email}</span>
          </div>
          <div className={styles['info-item']}>
            <span className={styles['info-label']}>Teléfono</span>
            <span className={styles['info-value']}>{user.phone_number || 'No registrado'}</span>
          </div>
          <div className={styles['info-item']}>
            <span className={styles['info-label']}>País</span>
            <span className={styles['info-value']}>{user.country || 'No registrado'}</span>
          </div>
          <div className={styles['info-item']}>
            <span className={styles['info-label']}>Provincia / Estado</span>
            <span className={styles['info-value']}>{user.state || 'No registrado'}</span>
          </div>
          <div className={styles['info-item']}>
            <span className={styles['info-label']}>Ciudad</span>
            <span className={styles['info-value']}>{user.city || 'No registrado'}</span>
          </div>
          <div className={styles['info-item']}>
            <span className={styles['info-label']}>Puesto</span>
            <span className={styles['info-value']}>{user.job_title || 'No registrado'}</span>
          </div>
          <div className={styles['info-item']}>
            <span className={styles['info-label']}>Dirección</span>
            <span className={styles['info-value']}>{user.location || 'No registrada'}</span>
          </div>
          {isProfessional && (
            <div className={styles['info-item']}>
              <span className={styles['info-label']}>Documento de identidad</span>
              <span className={styles['info-value']}>
                {user.identity_document ? (
                  <a href={user.identity_document} target="_blank" rel="noopener noreferrer" style={{ color: 'var(--primary, #8b5cf6)' }}>
                    Ver documento
                  </a>
                ) : (
                  'No registrado'
                )}
              </span>
            </div>
          )}
          {user.company_name && (
            <div className={styles['info-item']}>
              <span className={styles['info-label']}>Empresa</span>
              <span className={styles['info-value']}>{user.company_name}</span>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
