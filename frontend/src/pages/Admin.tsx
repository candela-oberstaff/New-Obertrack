import { useState } from 'react'
import { useAdmin } from '../hooks'
import {
  Users,
  Building2,
  Activity,
  BarChart3,
  Search,
  X,
  Check,
  Trash2,
  RefreshCw,
  Shield
} from 'lucide-react'
import Avatar from '../components/Common/Avatar'
import styles from '../components/Admin/Admin.module.css'

export default function Admin() {
  const {
    stats,
    users,
    companies,
    recentActivity,
    isLoading,
    activeTab,
    setActiveTab,
    deleteUser,
    toggleUserStatus,
    resetUserPassword,
    promoteToManager,
  } = useAdmin()

  const [searchQuery, setSearchQuery] = useState('')
  const [showDeleteModal, setShowDeleteModal] = useState(false)
  const [userToDelete, setUserToDelete] = useState<any>(null)

  const filteredUsers = Array.isArray(users) 
    ? users.filter((u: any) =>
        u.name?.toLowerCase().includes(searchQuery.toLowerCase()) ||
        u.email?.toLowerCase().includes(searchQuery.toLowerCase())
      )
    : []

  const tabs = [
    { id: 'dashboard', label: 'Dashboard', icon: BarChart3 },
    { id: 'users', label: 'Usuarios', icon: Users },
    { id: 'companies', label: 'Empresas', icon: Building2 },
    { id: 'activity', label: 'Actividad', icon: Activity },
  ]

  const handleDeleteUser = async () => {
    if (!userToDelete) return
    await deleteUser(userToDelete.id)
    setShowDeleteModal(false)
    setUserToDelete(null)
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
      <div className={styles['admin-header']}>
        <h1>Panel de Administración</h1>
        <p>Gestiona usuarios, empresas y actividad</p>
      </div>

      <div className={styles['admin-tabs']}>
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

      <div className={styles['admin-content']}>
        {activeTab === 'dashboard' && (
          <div className={styles['dashboard-tab']}>
            <div className={styles['stats-grid']}>
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

            <div className={styles['recent-activity-section']}>
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
                          {activity.user} • {new Date(activity.created_at).toLocaleString('es-ES')}
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
              <div className={styles['search-box']}>
                <Search size={18} />
                <input
                  type="text"
                  placeholder="Buscar usuarios..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                />
              </div>
            </div>

            <div className={styles['users-table']}>
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
                        <div className={styles['action-buttons']}>
                          <button
                            className={styles['btn-icon']}
                            onClick={() => toggleUserStatus(u.id)}
                            title={u.is_active ? 'Desactivar' : 'Activar'}
                          >
                            {u.is_active ? <X size={16} /> : <Check size={16} />}
                          </button>
                          <button
                            className={styles['btn-icon']}
                            onClick={() => resetUserPassword(u.id, 'temporary123')}
                            title="Resetear contraseña"
                          >
                            <RefreshCw size={16} />
                          </button>
                          {!u.is_manager && !u.is_superadmin && (
                            <button
                              className={styles['btn-icon']}
                              onClick={() => promoteToManager(u.id)}
                              title="Promover a Manager"
                            >
                              <Shield size={16} />
                            </button>
                          )}
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

        {activeTab === 'companies' && (
          <div className={styles['companies-tab']}>
            {companies.length === 0 ? (
              <div className={styles['empty-state']}>
                <Building2 size={40} />
                <p>No hay empresas registradas</p>
              </div>
            ) : (
              <div className={styles['companies-grid']}>
                {companies.map((company: any, index: number) => (
                  <div key={company.id || `company-${index}`} className={styles['company-card']}>
                    <div className={styles['company-header']}>
                      <Avatar 
                        src={company.logo} 
                        name={company.name} 
                        size="md" 
                      />
                      <div className={styles['company-info']}>
                        <h4>{company.name}</h4>
                        <p>{company.email}</p>
                      </div>
                    </div>
                    <div className={styles['company-stats']}>
                      <div className={styles['company-stat']}>
                        <span className={styles['stat-value']}>
                          {users.filter((u: any) => u.empleador_id === company.id).length}
                        </span>
                        <span className={styles['stat-label']}>Empleados</span>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
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
              <div className={styles['activity-list-full']}>
                {recentActivity.map((activity: any, index: number) => {
                  const date = activity.created_at ? new Date(activity.created_at) : null;
                  const isValidDate = date && !isNaN(date.getTime());
                  
                  return (
                    <div key={activity.id || `activity-full-${index}`} className={styles['activity-item-full']}>
                      <div className={styles['activity-icon']}>
                        <Activity size={20} />
                      </div>
                      <div className={styles['activity-details']}>
                        <p className={styles['activity-desc']}>{activity.description || 'Sin descripción'}</p>
                        <span className={styles['activity-meta']}>
                          <strong>{activity.user || 'Sistema'}</strong> • {isValidDate ? date.toLocaleString('es-ES') : 'Fecha no disponible'}
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
            <p className={styles['warning-text']}>Esta acción no se puede deshacer.</p>
            <div className={styles['modal-actions']}>
              <button onClick={() => setShowDeleteModal(false)}>Cancelar</button>
              <button className={styles['btn-danger']} onClick={handleDeleteUser}>Eliminar</button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
