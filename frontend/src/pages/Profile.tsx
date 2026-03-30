import { useState, useEffect, useRef } from 'react'
import { useAuth } from '../context/AuthContext'
import { userService, workHourService, uploadService } from '../services/api'
import type { WorkHour, User } from '../types'
import styles from './Profile.module.css'

const MONTHS_ES = ['Ene', 'Feb', 'Mar', 'Abr', 'May', 'Jun', 'Jul', 'Ago', 'Sep', 'Oct', 'Nov', 'Dic']

export default function Profile() {
  const { user, setUser, logout } = useAuth()
  const [isEditing, setIsEditing] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  const [message, setMessage] = useState({ type: '', text: '' })
  const [showPasswordModal, setShowPasswordModal] = useState(false)
  const [passwordData, setPasswordData] = useState({ current: '', new: '', confirm: '' })
  const [pendingHours, setPendingHours] = useState<WorkHour[]>([])
  const [selectedPending, setSelectedPending] = useState<number[]>([])
  const [isLoadingPending, setIsLoadingPending] = useState(false)
  const [isUploadingAvatar, setIsUploadingAvatar] = useState(false)
  const avatarInputRef = useRef<HTMLInputElement>(null)
  
  // Team management
  const [myTeam, setMyTeam] = useState<User[]>([])
  const [companyStaff, setCompanyStaff] = useState<User[]>([])
  const [isManager, setIsManager] = useState(false)
  const [isEmployer, setIsEmployer] = useState(false)

  const [formData, setFormData] = useState({
    name: user?.name || '',
    phone_number: user?.phone_number || '',
    country: user?.country || '',
    city: user?.city || '',
    location: user?.location || '',
    job_title: user?.job_title || '',
  })

  const canApprove = user?.is_superadmin || user?.is_manager || user?.user_type === 'empleador'

  useEffect(() => {
    if (canApprove) {
      fetchPendingHours()
    }
    
    // Check if user is manager or employer
    setIsManager(user?.is_manager || false)
    setIsEmployer(user?.user_type === 'empleador' || user?.is_superadmin || false)
    
    if (user?.is_manager) {
      fetchMyTeam()
    }
    if (user?.user_type === 'empleador' || user?.is_superadmin) {
      fetchCompanyStaff()
    }
  }, [user, canApprove])

  const fetchMyTeam = async () => {
    try {
      const data = await userService.getMyTeam()
      setMyTeam(data)
    } catch (error) {
      console.error('Error fetching team:', error)
    }
  }

  const fetchCompanyStaff = async () => {
    try {
      const data = await userService.getAll()
      const allUsers = data.data || []
      setCompanyStaff(allUsers.filter((u: User) => u.user_type === 'profesional' && u.empleador_id === user?.empleador_id))
    } catch (error) {
      console.error('Error fetching company staff:', error)
    }
  }

  const handlePromoteToManager = async (userId: number) => {
    if (!confirm('¿Promover a este profesional a Manager?')) return
    try {
      await userService.promoteToManager(userId)
      fetchCompanyStaff()
      setMessage({ type: 'success', text: 'Usuario promovido a Manager' })
      setTimeout(() => setMessage({ type: '', text: '' }), 3000)
    } catch (error) {
      setMessage({ type: 'error', text: 'Error al promover usuario' })
    }
  }

  const fetchPendingHours = async () => {
    try {
      const data = await workHourService.getPending()
      setPendingHours(data)
    } catch (error) {
      console.error('Error fetching pending hours:', error)
    }
  }

  const handleApproveHours = async () => {
    if (selectedPending.length === 0) return
    setIsLoadingPending(true)
    try {
      await workHourService.approve(selectedPending)
      setSelectedPending([])
      fetchPendingHours()
      setMessage({ type: 'success', text: 'Horas aprobadas correctamente' })
    } catch (error) {
      setMessage({ type: 'error', text: 'Error al aprobar horas' })
    } finally {
      setIsLoadingPending(false)
    }
  }

  const toggleSelectPending = (id: number) => {
    setSelectedPending(prev =>
      prev.includes(id) ? prev.filter(i => i !== id) : [...prev, id]
    )
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!user) return
    
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

  const handlePasswordChange = async (e: React.FormEvent) => {
    e.preventDefault()
    if (passwordData.new !== passwordData.confirm) {
      setMessage({ type: 'error', text: 'Las contraseñas no coinciden' })
      return
    }
    setIsLoading(true)
    setMessage({ type: '', text: '' })
    
    try {
      await userService.changePassword(user!.id, passwordData.current, passwordData.new)
      setMessage({ type: 'success', text: 'Contraseña actualizada correctamente' })
      setShowPasswordModal(false)
      setPasswordData({ current: '', new: '', confirm: '' })
    } catch (error: any) {
      const errorMsg = error.response?.data?.error || 'Error al cambiar la contraseña'
      setMessage({ type: 'error', text: errorMsg })
    } finally {
      setIsLoading(false)
    }
  }

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
    return 'Profesional'
  }

  const getRoleColor = () => {
    if (user?.is_superadmin) return '#8b5cf6'
    if (user?.is_manager) return '#f59e0b'
    if (user?.user_type === 'empleador') return '#3b82f6'
    return '#10b981'
  }

  return (
    <div className={styles['profile-page']}>
      <div className={styles['profile-header']}>
        <div className={styles['profile-cover']}></div>
        <div className={styles['profile-main']}>
          <div className={styles['profile-avatar-section']}>
            <div className={styles['profile-avatar']} onClick={() => !isUploadingAvatar && avatarInputRef.current?.click()}>
              {user?.avatar ? (
                <img src={user.avatar} alt={user.name} className={`${styles['avatar-image']} ${styles['large']}`} />
              ) : (
                <div className={`${styles['avatar-placeholder']} ${styles['large']}`}>
                  {user?.name?.charAt(0).toUpperCase()}
                </div>
              )}
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
          <div className={styles['profile-actions']}>
            {!isEditing ? (
              <>
                <button className={styles['btn-outline']} onClick={() => setShowPasswordModal(true)}>
                  Cambiar Contraseña
                </button>
                <button className={styles['btn-primary']} onClick={() => setIsEditing(true)}>
                  Editar Perfil
                </button>
              </>
            ) : (
              <button className={styles['btn-cancel']} onClick={() => setIsEditing(false)}>
                Cancelar
              </button>
            )}
          </div>
        </div>
      </div>

      <div className={styles['profile-content']}>
        <div className={styles['profile-main-info']}>
          <div className={styles['info-card']}>
            <div className={styles['card-header']}>
              <h3>Información Personal</h3>
            </div>
            {message.text && (
              <div className={`${styles['alert']} ${styles[message.type]}`}>
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
                  <span className={styles['info-value']}>{user?.email}</span>
                </div>
                <div className={styles['info-item']}>
                  <span className={styles['info-label']}>Teléfono</span>
                  <span className={styles['info-value']}>{user?.phone_number || 'No registrado'}</span>
                </div>
                <div className={styles['info-item']}>
                  <span className={styles['info-label']}>País</span>
                  <span className={styles['info-value']}>{user?.country || 'No registrado'}</span>
                </div>
                <div className={styles['info-item']}>
                  <span className={styles['info-label']}>Ciudad</span>
                  <span className={styles['info-value']}>{user?.city || 'No registrado'}</span>
                </div>
                <div className={styles['info-item']}>
                  <span className={styles['info-label']}>Puesto</span>
                  <span className={styles['info-value']}>{user?.job_title || 'No registrado'}</span>
                </div>
                <div className={styles['info-item']}>
                  <span className={styles['info-label']}>Dirección</span>
                  <span className={styles['info-value']}>{user?.location || 'No registrada'}</span>
                </div>
                {user?.company_name && (
                  <div className={styles['info-item']}>
                    <span className={styles['info-label']}>Empresa</span>
                    <span className={styles['info-value']}>{user.company_name}</span>
                  </div>
                )}
              </div>
            )}
          </div>
        </div>

        <div className={styles['profile-sidebar']}>
          <div className={styles['sidebar-card']}>
            <h3>Estadísticas</h3>
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

          {canApprove && pendingHours.length > 0 && (
            <div className={styles['sidebar-card']}>
              <h3>Horas Pendientes</h3>
              <p className={styles['pending-count']}>{pendingHours.length} registro(s) sin aprobar</p>
              
              <div className={styles['pending-list']}>
                {pendingHours.slice(0, 5).map(wh => (
                  <div key={wh.id} className={styles['pending-item']} onClick={() => toggleSelectPending(wh.id)}>
                    <input 
                      type="checkbox" 
                      checked={selectedPending.includes(wh.id)}
                      readOnly
                    />
                    <div className={styles['pending-info']}>
                      <span className={styles['pending-user']}>{wh.user?.name || 'Usuario'}</span>
                      <span className={styles['pending-date']}>
                        {new Date(wh.work_date).getDate()} {MONTHS_ES[new Date(wh.work_date).getMonth()]}
                      </span>
                      <span className={`${styles['pending-type']} ${styles[wh.work_type] || wh.work_type}`}>
                        {wh.work_type === 'complete' ? 'Completa' : 'Ausencia'}
                      </span>
                    </div>
                  </div>
                ))}
              </div>

              {pendingHours.length > 5 && (
                <p className={styles['more-pending']}>+ {pendingHours.length - 5} más</p>
              )}

              {selectedPending.length > 0 && (
                <button 
                  className={styles['btn-approve']} 
                  onClick={handleApproveHours}
                  disabled={isLoadingPending}
                >
                  {isLoadingPending ? 'Aprobando...' : `Aprobar (${selectedPending.length})`}
                </button>
              )}
            </div>
          )}

          {isManager && myTeam.length > 0 && (
            <div className={`${styles['sidebar-card']} ${styles['team-card']}`}>
              <h3>👥 Mi Equipo</h3>
              <p className={styles['team-count']}>{myTeam.length} profesional(es) a mi cargo</p>
              <div className={styles['team-list']}>
                {myTeam.map(member => (
                  <div key={member.id} className={styles['team-member']}>
                    <div className={styles['member-avatar']}>
                      {member.name?.charAt(0).toUpperCase()}
                    </div>
                    <div className={styles['member-info']}>
                      <span className={styles['member-name']}>{member.name}</span>
                      <span className={styles['member-role']}>{member.job_title || 'Profesional'}</span>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {isEmployer && companyStaff.length > 0 && (
            <div className={styles['sidebar-card']}>
              <h3>🏢 Personal de la Empresa</h3>
              <p className={styles['team-count']}>{companyStaff.length} profesional(es) registrado(s)</p>
              <div className={styles['team-list']}>
                {companyStaff.map(member => (
                  <div key={member.id} className={styles['team-member']}>
                    <div className={styles['member-avatar']}>
                      {member.name?.charAt(0).toUpperCase()}
                    </div>
                    <div className={styles['member-info']}>
                      <span className={styles['member-name']}>{member.name}</span>
                      <span className={styles['member-role']}>
                        {member.is_manager ? '👔 Manager' : '💼 Profesional'}
                      </span>
                    </div>
                    {!member.is_manager && (
                      <button 
                        className={styles['btn-promote']}
                        onClick={() => handlePromoteToManager(member.id)}
                        title="Promover a Manager"
                      >
                        ⬆️
                      </button>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}
          
          <div className={styles['sidebar-card']}>
            <h3>Cuenta</h3>
            <button className={styles['btn-logout']} onClick={logout}>
              Cerrar Sesión
            </button>
          </div>
        </div>
      </div>
      {showPasswordModal && (
        <div className={styles['modal-overlay'] || 'modal-overlay'} onClick={() => setShowPasswordModal(false)}>
          <div className={styles['modal'] || 'modal'} onClick={(e) => e.stopPropagation()}>
            <div className={styles['modal-header'] || 'modal-header'}>
              <h2>Cambiar Contraseña</h2>
              <button className={styles['close-btn'] || 'close-btn'} onClick={() => setShowPasswordModal(false)}>✕</button>
            </div>
            <form onSubmit={handlePasswordChange}>
              <div className={styles['form-group'] || 'form-group'}>
                <label>Contraseña actual</label>
                <input
                  type="password"
                  value={passwordData.current}
                  onChange={(e) => setPasswordData({ ...passwordData, current: e.target.value })}
                  required
                />
              </div>
              <div className={styles['form-group'] || 'form-group'}>
                <label>Nueva contraseña</label>
                <input
                  type="password"
                  value={passwordData.new}
                  onChange={(e) => setPasswordData({ ...passwordData, new: e.target.value })}
                  required
                  minLength={6}
                />
              </div>
              <div className={styles['form-group'] || 'form-group'}>
                <label>Confirmar contraseña</label>
                <input
                  type="password"
                  value={passwordData.confirm}
                  onChange={(e) => setPasswordData({ ...passwordData, confirm: e.target.value })}
                  required
                />
              </div>
              <div className={styles['modal-actions'] || 'modal-actions'}>
                <button type="button" className={styles['btn-cancel'] || 'btn-cancel'} onClick={() => setShowPasswordModal(false)}>
                  Cancelar
                </button>
                <button type="submit" className={styles['btn-primary'] || 'btn-primary'} disabled={isLoading}>
                  Cambiar
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
