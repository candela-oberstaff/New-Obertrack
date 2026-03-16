import { useState, useEffect } from 'react'
import { useAuth } from '../context/AuthContext'
import { userService, workHourService } from '../services/api'
import type { WorkHour } from '../types'
import './Profile.css'

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
  }, [canApprove])

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
      await userService.update(user!.id, { password: passwordData.new })
      setMessage({ type: 'success', text: 'Contraseña actualizada correctamente' })
      setShowPasswordModal(false)
      setPasswordData({ current: '', new: '', confirm: '' })
    } catch (error) {
      setMessage({ type: 'error', text: 'Error al cambiar la contraseña' })
    } finally {
      setIsLoading(false)
    }
  }

  const getRoleLabel = () => {
    if (user?.is_superadmin) return 'Super Administrador'
    if (user?.is_manager) return 'Manager'
    if (user?.user_type === 'empleador') return 'Empleador'
    return 'Profesional'
  }

  const getRoleColor = () => {
    if (user?.is_superadmin) return '#8b5cf6'
    if (user?.is_manager) return '#f59e0b'
    if (user?.user_type === 'empleador') return '#3b82f6'
    return '#10b981'
  }

  return (
    <div className="profile-page">
      <div className="profile-header">
        <div className="profile-cover"></div>
        <div className="profile-main">
          <div className="profile-avatar-section">
            <div className="profile-avatar">
              <div className="avatar-placeholder large">
                {user?.name?.charAt(0).toUpperCase()}
              </div>
            </div>
            <div className="profile-title">
              <h1>{user?.name}</h1>
              <p className="profile-role" style={{ color: getRoleColor() }}>
                {getRoleLabel()}
              </p>
            </div>
          </div>
          <div className="profile-actions">
            {!isEditing ? (
              <>
                <button className="btn-outline" onClick={() => setShowPasswordModal(true)}>
                  Cambiar Contraseña
                </button>
                <button className="btn-primary" onClick={() => setIsEditing(true)}>
                  Editar Perfil
                </button>
              </>
            ) : (
              <button className="btn-cancel" onClick={() => setIsEditing(false)}>
                Cancelar
              </button>
            )}
          </div>
        </div>
      </div>

      <div className="profile-content">
        <div className="profile-main-info">
          <div className="info-card">
            <div className="card-header">
              <h3>Información Personal</h3>
            </div>
            {message.text && (
              <div className={`alert ${message.type}`}>
                {message.text}
              </div>
            )}
            
            {isEditing ? (
              <form onSubmit={handleSubmit} className="edit-form">
                <div className="form-row">
                  <div className="form-group">
                    <label>Nombre completo</label>
                    <input
                      type="text"
                      value={formData.name}
                      onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                      required
                    />
                  </div>
                  <div className="form-group">
                    <label>Teléfono</label>
                    <input
                      type="tel"
                      value={formData.phone_number}
                      onChange={(e) => setFormData({ ...formData, phone_number: e.target.value })}
                    />
                  </div>
                </div>
                
                <div className="form-row">
                  <div className="form-group">
                    <label>País</label>
                    <input
                      type="text"
                      value={formData.country}
                      onChange={(e) => setFormData({ ...formData, country: e.target.value })}
                    />
                  </div>
                  <div className="form-group">
                    <label>Ciudad</label>
                    <input
                      type="text"
                      value={formData.city}
                      onChange={(e) => setFormData({ ...formData, city: e.target.value })}
                    />
                  </div>
                </div>
                
                <div className="form-group">
                  <label>Puesto / Cargo</label>
                  <input
                    type="text"
                    value={formData.job_title}
                    onChange={(e) => setFormData({ ...formData, job_title: e.target.value })}
                    placeholder="Ej: Desarrollador Frontend"
                  />
                </div>
                
                <div className="form-group">
                  <label>Dirección</label>
                  <textarea
                    value={formData.location}
                    onChange={(e) => setFormData({ ...formData, location: e.target.value })}
                    rows={2}
                    placeholder="Dirección completa"
                  />
                </div>
                
                <div className="form-actions">
                  <button type="submit" className="btn-primary" disabled={isLoading}>
                    {isLoading ? 'Guardando...' : 'Guardar Cambios'}
                  </button>
                </div>
              </form>
            ) : (
              <div className="info-list">
                <div className="info-item">
                  <span className="info-label">Email</span>
                  <span className="info-value">{user?.email}</span>
                </div>
                <div className="info-item">
                  <span className="info-label">Teléfono</span>
                  <span className="info-value">{user?.phone_number || 'No registrado'}</span>
                </div>
                <div className="info-item">
                  <span className="info-label">País</span>
                  <span className="info-value">{user?.country || 'No registrado'}</span>
                </div>
                <div className="info-item">
                  <span className="info-label">Ciudad</span>
                  <span className="info-value">{user?.city || 'No registrado'}</span>
                </div>
                <div className="info-item">
                  <span className="info-label">Puesto</span>
                  <span className="info-value">{user?.job_title || 'No registrado'}</span>
                </div>
                <div className="info-item">
                  <span className="info-label">Dirección</span>
                  <span className="info-value">{user?.location || 'No registrada'}</span>
                </div>
                {user?.company_name && (
                  <div className="info-item">
                    <span className="info-label">Empresa</span>
                    <span className="info-value">{user.company_name}</span>
                  </div>
                )}
              </div>
            )}
          </div>
        </div>

        <div className="profile-sidebar">
          <div className="sidebar-card">
            <h3>Estadísticas</h3>
            <div className="stat-item">
              <span className="stat-label">Miembro desde</span>
              <span className="stat-value">
                {user?.created_at ? new Date(user.created_at).toLocaleDateString('es-ES', { month: 'long', year: 'numeric' }) : '-'}
              </span>
            </div>
          </div>

          {canApprove && pendingHours.length > 0 && (
            <div className="sidebar-card approval-card">
              <h3>Horas Pendientes</h3>
              <p className="pending-count">{pendingHours.length} registro(s) sin aprobar</p>
              
              <div className="pending-list">
                {pendingHours.slice(0, 5).map(wh => (
                  <div key={wh.id} className="pending-item" onClick={() => toggleSelectPending(wh.id)}>
                    <input 
                      type="checkbox" 
                      checked={selectedPending.includes(wh.id)}
                      onChange={() => {}}
                    />
                    <div className="pending-info">
                      <span className="pending-user">{wh.user?.name || 'Usuario'}</span>
                      <span className="pending-date">
                        {new Date(wh.work_date).getDate()} {MONTHS_ES[new Date(wh.work_date).getMonth()]}
                      </span>
                      <span className={`pending-type ${wh.work_type}`}>
                        {wh.work_type === 'complete' ? 'Completa' : 'Ausencia'}
                      </span>
                    </div>
                  </div>
                ))}
              </div>

              {pendingHours.length > 5 && (
                <p className="more-pending">+ {pendingHours.length - 5} más</p>
              )}

              {selectedPending.length > 0 && (
                <button 
                  className="btn-approve" 
                  onClick={handleApproveHours}
                  disabled={isLoadingPending}
                >
                  {isLoadingPending ? 'Aprobando...' : `Aprobar (${selectedPending.length})`}
                </button>
              )}
            </div>
          )}
          
          <div className="sidebar-card danger">
            <h3>Zona Peligrosa</h3>
            <p>Cerrar sesión permanentemente</p>
            <button className="btn-logout" onClick={logout}>
              Cerrar Sesión
            </button>
          </div>
        </div>
      </div>

      {showPasswordModal && (
        <div className="modal-overlay" onClick={() => setShowPasswordModal(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h2>Cambiar Contraseña</h2>
              <button className="close-btn" onClick={() => setShowPasswordModal(false)}>✕</button>
            </div>
            <form onSubmit={handlePasswordChange}>
              <div className="form-group">
                <label>Contraseña actual</label>
                <input
                  type="password"
                  value={passwordData.current}
                  onChange={(e) => setPasswordData({ ...passwordData, current: e.target.value })}
                  required
                />
              </div>
              <div className="form-group">
                <label>Nueva contraseña</label>
                <input
                  type="password"
                  value={passwordData.new}
                  onChange={(e) => setPasswordData({ ...passwordData, new: e.target.value })}
                  required
                  minLength={6}
                />
              </div>
              <div className="form-group">
                <label>Confirmar contraseña</label>
                <input
                  type="password"
                  value={passwordData.confirm}
                  onChange={(e) => setPasswordData({ ...passwordData, confirm: e.target.value })}
                  required
                />
              </div>
              <div className="modal-actions">
                <button type="button" className="btn-cancel" onClick={() => setShowPasswordModal(false)}>
                  Cancelar
                </button>
                <button type="submit" className="btn-primary" disabled={isLoading}>
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
