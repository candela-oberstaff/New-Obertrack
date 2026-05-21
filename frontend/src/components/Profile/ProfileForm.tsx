import React, { useState } from 'react'
import { userService } from '../../services/api'
import type { User } from '../../types'
import Tooltip from '../Common/Tooltip'
import styles from '../../pages/Profile.module.css'

interface ProfileFormProps {
  user: User
  setUser: (user: User) => void
  isEditing: boolean
  setIsEditing: (val: boolean) => void
}

export function ProfileForm({ user, setUser, isEditing, setIsEditing }: ProfileFormProps) {
  const [isLoading, setIsLoading] = useState(false)
  const [message, setMessage] = useState({ type: '', text: '' })
  const [formData, setFormData] = useState({
    name: user.name || '',
    phone_number: user.phone_number || '',
    country: user.country || '',
    city: user.city || '',
    location: user.location || '',
    job_title: user.job_title || '',
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

  const isEmployer = user?.user_type === 'empleador' || user?.is_superadmin || user?.is_manager

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
              <input
                type="text"
                value={formData.country}
                onChange={(e) => setFormData({ ...formData, country: e.target.value })}
              />
            </div>
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
