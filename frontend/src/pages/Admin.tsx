import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAdmin } from '../hooks'
import {
  Users,
  Building2,
  Activity,
  BarChart3,
  AlertTriangle,
  CalendarX,
  CheckCircle2,
  Clock,
  Search,
  Trash2,
  Pencil,
  Eye
} from 'lucide-react'
import Avatar from '../components/Common/Avatar'
import { UserModal } from '../components/Admin/Modals/UserModal'
import { Select } from '../components/ui/Select'
import styles from '../components/Admin/Admin.module.css'

export default function Admin() {
  const {
    stats,
    users,
    inactiveUsers,
    recentActivity,
    absenceReport,
    isLoading,
    activeTab,
    setActiveTab,
    deleteUser,
    updateUser,
  } = useAdmin()

  const navigate = useNavigate()
  const [searchQuery, setSearchQuery] = useState('')
  const [showDeleteModal, setShowDeleteModal] = useState(false)
  const [userToDelete, setUserToDelete] = useState<any>(null)
  const [showEditModal, setShowEditModal] = useState(false)
  const [editId, setEditId] = useState<number | null>(null)
  const [editForm, setEditForm] = useState<any>({})

  const employers = Array.isArray(users) ? users.filter((u: any) => u.user_type === 'empleador') : []
  const managers = Array.isArray(users) ? users.filter((u: any) => u.is_manager) : []

  const openEdit = (u: any) => {
    setEditId(u.id)
    setEditForm({
      name: u.name || '',
      email: u.email || '',
      user_type: u.user_type || '',
      job_title: u.job_title || '',
      phone_number: u.phone_number || '',
      country: u.country || '',
      city: u.city || '',
      location: u.location || '',
      company_name: u.company_name || '',
      empleador_id: u.empleador_id || '',
      manager_id: u.manager_id || '',
      is_active: u.is_active,
      is_manager: u.is_manager,
    })
    setShowEditModal(true)
  }

  const [editError, setEditError] = useState<string | null>(null)

  const handleEditSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (editId == null) return
    setEditError(null)
    // Sanitize FK ids: send a positive number or null — never "" (which the
    // backend's *uint binding rejects with 400, making the whole save fail).
    const payload = {
      ...editForm,
      empleador_id: editForm.empleador_id ? Number(editForm.empleador_id) : null,
      manager_id: editForm.manager_id ? Number(editForm.manager_id) : null,
    }
    try {
      await updateUser(editId, payload)
      setShowEditModal(false)
      setEditId(null)
    } catch (err: any) {
      setEditError(err?.response?.data?.error ?? 'No se pudieron guardar los cambios.')
    }
  }
  const absenceItems = absenceReport?.items || []
  const topInactiveUsers = Array.isArray(inactiveUsers) ? inactiveUsers.slice(0, 5) : []

  const filteredUsers = Array.isArray(users) 
    ? users.filter((u: any) =>
        u.name?.toLowerCase().includes(searchQuery.toLowerCase()) ||
        u.email?.toLowerCase().includes(searchQuery.toLowerCase())
      )
    : []

  const tabs = [
    { id: 'dashboard', label: 'Dashboard', icon: BarChart3 },
    { id: 'users', label: 'Usuarios', icon: Users },
    { id: 'activity', label: 'Actividad', icon: Activity },
  ]

  const [deleteError, setDeleteError] = useState<string | null>(null)
  const [deleting, setDeleting] = useState(false)

  const handleDeleteUser = async () => {
    if (!userToDelete) return
    setDeleting(true)
    setDeleteError(null)
    try {
      await deleteUser(userToDelete.id)
      setShowDeleteModal(false)
      setUserToDelete(null)
    } catch (err: any) {
      setDeleteError(err?.response?.data?.error ?? 'No se pudo eliminar el usuario.')
    } finally {
      setDeleting(false)
    }
  }

  const formatActivityDate = (value?: string) => {
    if (!value) return 'Fecha no disponible'
    const date = new Date(value)
    return Number.isNaN(date.getTime()) ? 'Fecha no disponible' : date.toLocaleString('es-ES')
  }

  const formatShortDate = (value?: string) => {
    if (!value) return 'Sin fecha'
    const date = new Date(value)
    return Number.isNaN(date.getTime()) ? 'Sin fecha' : date.toLocaleDateString('es-ES', { day: '2-digit', month: 'short' })
  }

  const getAbsenceStatus = (item: any) => {
    if (item.rejected) return { label: 'Rechazada', className: 'danger' }
    if (item.approved) return { label: 'Aprobada', className: 'success' }
    return { label: 'Pendiente', className: 'warning' }
  }

  if (isLoading) {
    return (
      <div className={styles['admin-page']}>
        <div className={styles['admin-loading']}>
          <div className={styles['spinner']} />
          <p>Cargando panel de administración...</p>
        </div>
      </div>
    )
  }

  return (
    <div className={styles['admin-page']}>
      <div className={styles['admin-header']} data-tour="admin-header">
        <h1>Panel de Administración</h1>
        <p>Gestiona usuarios y actividad</p>
      </div>

      <div className={styles['admin-tabs']} data-tour="admin-tabs">
        {tabs.map(tab => (
          <button
            key={tab.id}
            className={`${styles['tab-btn']} ${activeTab === tab.id ? styles['active'] : ''}`}
            onClick={() => setActiveTab(tab.id)}
          >
            <tab.icon size={18} />
            {tab.label}
          </button>
        ))}
      </div>

      <div className={styles['mobile-tabs']}>
        <Select
          fullWidth
          value={activeTab}
          onChange={(v) => setActiveTab(String(v))}
          options={tabs.map(tab => ({ value: tab.id, label: tab.label }))}
        />
      </div>

      <div className={styles['admin-content']}>
        {activeTab === 'dashboard' && (
          <div className={styles['dashboard-tab']}>
            <div className={styles['stats-grid']} data-tour="admin-stats">
              <div className={styles['stat-card']}>
                <div className={styles['stat-icon']} style={{ background: 'linear-gradient(135deg, var(--primary), var(--primary-dark))' }}>
                  <Users size={26} />
                </div>
                <div className={styles['stat-info']}>
                  <span className={styles['stat-value']}>{stats?.totalUsers || 0}</span>
                  <span className={styles['stat-label']}>Total Usuarios</span>
                </div>
              </div>
              <div className={styles['stat-card']}>
                <div className={styles['stat-icon']} style={{ background: 'linear-gradient(135deg, #10b981, #059669)' }}>
                  <Users size={26} />
                </div>
                <div className={styles['stat-info']}>
                  <span className={styles['stat-value']}>{stats?.activeUsers || 0}</span>
                  <span className={styles['stat-label']}>Usuarios Activos</span>
                </div>
              </div>
              <div className={styles['stat-card']}>
                <div className={styles['stat-icon']} style={{ background: 'linear-gradient(135deg, #f59e0b, #d97706)' }}>
                  <Building2 size={26} />
                </div>
                <div className={styles['stat-info']}>
                  <span className={styles['stat-value']}>{stats?.totalBoards || 0}</span>
                  <span className={styles['stat-label']}>Tableros</span>
                </div>
              </div>
              <div className={styles['stat-card']}>
                <div className={styles['stat-icon']} style={{ background: 'linear-gradient(135deg, #8b5cf6, #7c3aed)' }}>
                  <Activity size={26} />
                </div>
                <div className={styles['stat-info']}>
                  <span className={styles['stat-value']}>{stats?.totalTasks || 0}</span>
                  <span className={styles['stat-label']}>Tareas</span>
                </div>
              </div>
            </div>

            <div className={styles['operations-grid']}>
              <section className={styles['activity-status-card']}>
                <div className={styles['section-heading']}>
                  <div>
                    <h3>Actividad del equipo</h3>
                    <p>Marcas de actividad e inactividad para seguimiento diario.</p>
                  </div>
                </div>

                <div className={styles['status-metrics']}>
                  <div className={`${styles['status-metric']} ${styles['success']}`}>
                    <div className={styles['status-icon']}><CheckCircle2 size={18} /></div>
                    <div>
                      <strong>{stats?.activeToday || 0}</strong>
                      <span>Activos hoy</span>
                    </div>
                  </div>
                  <div className={`${styles['status-metric']} ${styles['warning']}`}>
                    <div className={styles['status-icon']}><Clock size={18} /></div>
                    <div>
                      <strong>{topInactiveUsers.length || stats?.inactiveWarning || 0}</strong>
                      <span>Sin actividad +7d</span>
                    </div>
                  </div>
                  <div className={`${styles['status-metric']} ${styles['danger']}`}>
                    <div className={styles['status-icon']}><CalendarX size={18} /></div>
                    <div>
                      <strong>{absenceReport?.total_absences || 0}</strong>
                      <span>Ausencias del mes</span>
                    </div>
                  </div>
                </div>

                <div className={styles['watch-list']}>
                  <div className={styles['watch-list-header']}>
                    <span>Profesionales sin actividad reciente</span>
                  </div>
                  {topInactiveUsers.length === 0 ? (
                    <p className={styles['empty-message']}>No hay alertas de inactividad</p>
                  ) : (
                    topInactiveUsers.map((user: any) => (
                      <div key={user.id} className={styles['watch-row']}>
                        <Avatar src={user.avatar} name={user.name} size="sm" />
                        <div>
                          <strong>{user.name}</strong>
                          <span>{user.company || 'Sin empresa'}</span>
                        </div>
                        <span className={styles['days-badge']}>{user.days_inactive || 0}d</span>
                      </div>
                    ))
                  )}
                </div>
              </section>

              <section className={styles['absence-report-card']}>
                <div className={styles['section-heading']}>
                  <div>
                    <h3>Reporte de ausencias</h3>
                    <p>Resumen mensual con horas ausentes y registros pendientes.</p>
                  </div>
                  <AlertTriangle size={20} />
                </div>

                <div className={styles['absence-summary']}>
                  <div>
                    <span>Total ausencias</span>
                    <strong>{absenceReport?.total_absences || 0}</strong>
                  </div>
                  <div>
                    <span>Horas ausentes</span>
                    <strong>{(absenceReport?.absence_hours || 0).toFixed(1)}h</strong>
                  </div>
                  <div>
                    <span>Pendientes</span>
                    <strong>{absenceReport?.pending_review || 0}</strong>
                  </div>
                </div>

                {absenceReport?.reasons?.length ? (
                  <div className={styles['reason-cloud']}>
                    {absenceReport.reasons.map((reason: any) => (
                      <span key={reason.reason}>{reason.reason} ({reason.count})</span>
                    ))}
                  </div>
                ) : null}

                <div className={styles['absence-list']}>
                  {absenceItems.length === 0 ? (
                    <p className={styles['empty-message']}>No hay ausencias registradas este mes</p>
                  ) : (
                    absenceItems.slice(0, 5).map((item: any) => {
                      const status = getAbsenceStatus(item)
                      return (
                        <div key={item.id} className={styles['absence-row']}>
                          <div>
                            <strong>{item.user}</strong>
                            <span>{item.company} - {formatShortDate(item.work_date)} - {(item.absence_hours || 0).toFixed(1)}h</span>
                            <small>{item.absence_reason || 'Sin motivo'}</small>
                          </div>
                          <span className={`${styles['pill']} ${styles[status.className]}`}>{status.label}</span>
                        </div>
                      )
                    })
                  )}
                </div>
              </section>
            </div>

            <div className={styles['recent-activity-section']} data-tour="admin-recent-activity">
              <h3>Actividad Reciente</h3>
              {recentActivity.length === 0 ? (
                <p className={styles['empty-message']}>No hay actividad reciente</p>
              ) : (
                <div className={styles['activity-list']}>
                  {recentActivity.slice(0, 10).map((activity: any, index: number) => (
                    <div key={activity.id || `activity-${index}`} className={styles['activity-item']}>
                      <div className={styles['activity-icon']}>
                        <Activity size={16} />
                      </div>
                      <div className={styles['activity-content']}>
                        <p>{activity.description}</p>
                        <span className={styles['activity-meta']}>
                          {activity.user || 'Sistema'} - {formatActivityDate(activity.created_at)}
                        </span>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        )}

        {activeTab === 'users' && (
          <div className={styles['users-tab']}>
            <div className={styles['tab-header']}>
              <div className={styles['search-box']} data-tour="admin-search">
                <Search size={18} />
                <input
                  type="text"
                  placeholder="Buscar usuarios..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                />
              </div>
            </div>

            <div className={styles['users-table']} data-tour="admin-users-table">
              <table>
                <thead>
                  <tr>
                    <th>Usuario</th>
                    <th>Email</th>
                    <th>Tipo</th>
                    <th>Estado</th>
                    <th>Acciones</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredUsers.map((u: any, index: number) => (
                    <tr key={u.id || `user-${index}`}>
                      <td>
                        <div className={styles['user-cell']}>
                          <Avatar 
                            src={u.avatar} 
                            name={u.name} 
                            size="sm" 
                          />
                          <span>{u.name}</span>
                        </div>
                      </td>
                      <td>{u.email}</td>
                      <td>
                        <span className={`${styles['badge']} ${styles[u.user_type] || ''}`}>
                          {u.user_type}
                        </span>
                      </td>
                      <td>
                        <span className={`${styles['status-badge']} ${u.is_active ? styles['active'] : styles['inactive']}`}>
                          {u.is_active ? 'Activo' : 'Inactivo'}
                        </span>
                      </td>
                      <td>
                        <div className={styles['action-buttons']} data-tour="admin-user-actions">
                          <button
                            className={styles['btn-icon']}
                            onClick={() => navigate(`/admin/users/${u.id}`)}
                            title="Ver detalles"
                          >
                            <Eye size={16} />
                          </button>
                          <button
                            className={styles['btn-icon']}
                            onClick={() => openEdit(u)}
                            title="Editar"
                          >
                            <Pencil size={16} />
                          </button>
                          <button
                            className={`${styles['btn-icon']} ${styles['danger']}`}
                            onClick={() => {
                              setUserToDelete(u)
                              setShowDeleteModal(true)
                            }}
                            title="Eliminar"
                          >
                            <Trash2 size={16} />
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}

        {activeTab === 'activity' && (
          <div className={styles['activity-tab']}>
            {recentActivity.length === 0 ? (
              <div className={styles['empty-state']}>
                <Activity size={40} />
                <p>No hay actividad registrada</p>
              </div>
            ) : (
              <div className={styles['activity-list-full']} data-tour="admin-activity-list">
                {recentActivity.map((activity: any, index: number) => {
                  return (
                    <div key={activity.id || `activity-full-${index}`} className={styles['activity-item-full']}>
                      <div className={styles['activity-icon']}>
                        <Activity size={20} />
                      </div>
                      <div className={styles['activity-details']}>
                        <p className={styles['activity-desc']}>{activity.description || 'Sin descripción'}</p>
                        <span className={styles['activity-meta']}>
                          <strong>{activity.user || 'Sistema'}</strong> - {formatActivityDate(activity.created_at)}
                        </span>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        )}
      </div>

      {showDeleteModal && userToDelete && (
        <div className={styles['modal-overlay']} onClick={() => setShowDeleteModal(false)}>
          <div className={styles['modal']} onClick={(e) => e.stopPropagation()}>
            <h2>Confirmar Eliminación</h2>
            <p>¿Estás seguro de eliminar al usuario <strong>{userToDelete.name}</strong>?</p>
            <p className={styles['warning-text']}>El usuario dejará de aparecer y no podrá iniciar sesión (sus registros se conservan).</p>
            {deleteError && (
              <p style={{ color: '#dc2626', fontWeight: 600, padding: '0 1.5rem', margin: '0 0 0.5rem' }}>{deleteError}</p>
            )}
            <div className={styles['modal-actions']}>
              <button className={styles['btn-secondary']} onClick={() => setShowDeleteModal(false)} disabled={deleting}>Cancelar</button>
              <button className={styles['btn-danger']} onClick={handleDeleteUser} disabled={deleting}>{deleting ? 'Eliminando…' : 'Eliminar'}</button>
            </div>
          </div>
        </div>
      )}

      {showEditModal && (
        <UserModal
          title="Editar usuario"
          mode="edit"
          form={editForm}
          setForm={setEditForm}
          employers={employers}
          managers={managers}
          onClose={() => setShowEditModal(false)}
          onSubmit={handleEditSubmit}
          error={editError}
        />
      )}

    </div>
  )
}
