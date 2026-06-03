import { useState, useRef } from 'react'
import { useAuth } from '../context/AuthContext'
import { userService, uploadService } from '../services/api'
import { ProfileForm } from '../components/Profile/ProfileForm'
import { PasswordModal } from '../components/Profile/PasswordModal'
import { PendingHoursPanel } from '../components/Profile/PendingHoursPanel'
import { TeamPanel } from '../components/Profile/TeamPanel'
import Avatar from '../components/Common/Avatar'
import Tooltip from '../components/Common/Tooltip'
import styles from './Profile.module.css'

export default function Profile() {
  const { user, setUser, logout } = useAuth()
  const [isEditing, setIsEditing] = useState(false)
  const [showPasswordModal, setShowPasswordModal] = useState(false)
  const [isUploadingAvatar, setIsUploadingAvatar] = useState(false)
  const [message, setMessage] = useState({ type: '', text: '' })
  const avatarInputRef = useRef<HTMLInputElement>(null)
  
  const canApprove = user?.is_superadmin || user?.is_manager || user?.user_type === 'empleador'




  const handleAvatarChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file || !user) return
    
    if (!file.type.startsWith('image/')) {
      setMessage({ type: 'error', text: 'Por favor selecciona una imagen' })
      return
    }
    
    if (file.size > 5 * 1024 * 1024) {
      setMessage({ type: 'error', text: 'La imagen debe ser menor a 5MB' })
      return
    }
    
    setIsUploadingAvatar(true)
    setMessage({ type: '', text: '' })
    
    try {
      const uploadResult = await uploadService.upload(file)
      const updatedUser = await userService.update(user.id, { avatar: uploadResult.url })
      setUser(updatedUser)
      setMessage({ type: 'success', text: 'Foto de perfil actualizada' })
    } catch (error) {
      setMessage({ type: 'error', text: 'Error al subir la imagen' })
    } finally {
      setIsUploadingAvatar(false)
      if (avatarInputRef.current) avatarInputRef.current.value = ''
    }
  }

  const getRoleLabel = () => {
    if (user?.is_superadmin) return 'Super Administrador'
    if (user?.is_manager) return 'Manager'
    if (user?.user_type === 'empleador') return 'Empresa'
    if (user?.user_type === 'customer_success') return 'Customer Success'
    return 'Profesional'
  }

  const getRoleColor = () => {
    if (user?.is_superadmin) return '#8b5cf6'
    if (user?.is_manager) return '#f59e0b'
    if (user?.user_type === 'empleador') return 'var(--primary)'
    if (user?.user_type === 'customer_success') return '#cc33cc'
    return '#10b981'
  }

  return (
    <div className={styles['profile-page']}>
      <div className={styles['profile-header']} data-tour="profile-header">
        <div className={styles['profile-cover']}></div>
        <div className={styles['profile-main']}>
          <div className={styles['profile-avatar-section']}>
            <div className={styles['profile-avatar']} onClick={() => !isUploadingAvatar && avatarInputRef.current?.click()}>
              <Avatar 
                src={user?.avatar} 
                name={user?.name} 
                size="xl" 
              />
              {isUploadingAvatar && <div className={styles['avatar-loading']}>⟳</div>}
            </div>
            <input
              ref={avatarInputRef}
              type="file"
              accept="image/*"
              onChange={handleAvatarChange}
              style={{ display: 'none' }}
            />
            <div className={styles['profile-title']}>
              <h1>{user?.name}</h1>
              <p className={styles['profile-role']} style={{ color: getRoleColor() }}>
                {getRoleLabel()}
              </p>
            </div>
          </div>
          {message.text && (
            <div className={`${styles['alert']} ${styles[message.type] || message.type}`} style={{ padding: '8px 16px', margin: '0 24px', borderRadius: '8px', fontSize: '13px', gridColumn: 'span 2' }}>
              {message.text}
            </div>
          )}
        </div>
      </div>

      <div className={styles['profile-content']}>
        <div className={styles['profile-main-info']} data-tour="profile-form">
          {user && (
            <ProfileForm 
              user={user} 
              setUser={setUser} 
              isEditing={isEditing} 
              setIsEditing={setIsEditing} 
            />
          )}
        </div>

        <div className={styles['profile-sidebar']}>
          <div className={styles['sidebar-card']} style={{ marginBottom: '16px' }} data-tour="profile-options">
            <h3 style={{ marginBottom: '12px' }}>Opciones</h3>
            <div style={{ display: 'flex', gap: '8px' }}>
              <button 
                className={styles['btn-outline']} 
                style={{ flex: 1, padding: '8px' }}
                onClick={() => setShowPasswordModal(true)}
              >
                Cambiar Contraseña
              </button>
              <button 
                className={styles['btn-primary']} 
                style={{ flex: 1, padding: '8px' }}
                onClick={() => setIsEditing(!isEditing)}
              >
                {isEditing ? 'Cancelar' : 'Editar Perfil'}
              </button>
            </div>
          </div>

          <div className={styles['sidebar-card']} data-tour="profile-stats">
            <h3>
              Estadísticas{' '}
              <Tooltip content="Estado actual de tu cuenta y fecha en la que te uniste a Obertrack" size={14} />
            </h3>
            <div className={styles['stat-item']}>
              <span className={styles['stat-label']}>Miembro desde</span>
              <span className={styles['stat-value']}>
                {user?.created_at ? new Date(user.created_at).toLocaleDateString('es-ES', { month: 'long', year: 'numeric' }) : 'Reciente'}
              </span>
            </div>
            <div className={styles['stat-item']}>
              <span className={styles['stat-label']}>Estado</span>
              <span className={styles['stat-value']} style={{ color: '#10b981' }}>Activo</span>
            </div>
          </div>

          {canApprove && <PendingHoursPanel />}

          {user?.is_manager && user?.id && <TeamPanel type="manager" userId={user.id} />}

          {(user?.user_type === 'empleador' || user?.is_superadmin) && user?.id && (
             <TeamPanel type="employer" userId={user.id} employerId={user.empleador_id} />
          )}
          
          <div className={styles['sidebar-card']} data-tour="profile-account">
            <h3>Cuenta</h3>
            <button className={styles['btn-logout']} onClick={logout}>
              Cerrar Sesión
            </button>
          </div>
        </div>
      </div>
      {user && (
        <PasswordModal 
          isOpen={showPasswordModal} 
          onClose={() => setShowPasswordModal(false)} 
          userId={user.id!} 
        />
      )}
    </div>
  )
}
