import { useState, useEffect } from 'react'
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
  Eye,
  UserPlus,
  X,
  ChevronLeft,
  ChevronRight
} from 'lucide-react'
import Avatar from '../components/Common/Avatar'
import { UserModal } from '../components/Admin/Modals/UserModal'
import { Select } from '../components/ui/Select'
import { Skeleton } from '../components/ui'
import { ActivityFeed } from '../components/Admin/ActivityFeed'
import { authService } from '../services/api'
import { useAuth } from '../context/AuthContext'
import styles from '../components/Admin/Admin.module.css'

const EMPTY_CREATE_FORM = {
  name: '',
  email: '',
  password: '',
  userType: 'profesional',
  companyName: '',
  industry: '',
  selectedCompanyId: '' as number | '',
  phoneNumber: '',
  country: '',
  province: '',
  city: '',
  location: '',
  address: '',
  jobTitle: '',
}

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
    createUser,
    deleteUser,
    updateUser,
  } = useAdmin()

  const navigate = useNavigate()
  const { user: viewer } = useAuth()
  // CS (manager y analista) entran en modo consulta: sin crear/editar/eliminar.
  const canManage = !!viewer?.is_superadmin
  const [searchQuery, setSearchQuery] = useState('')
  const [roleFilter, setRoleFilter] = useState('')
  const [companyFilter, setCompanyFilter] = useState<number | ''>('')
  const [usersPage, setUsersPage] = useState(1)
  const [showDeleteModal, setShowDeleteModal] = useState(false)
  const [userToDelete, setUserToDelete] = useState<any>(null)
  const [showEditModal, setShowEditModal] = useState(false)
  const [editId, setEditId] = useState<number | null>(null)
  const [editForm, setEditForm] = useState<any>({})

  const [showCreateModal, setShowCreateModal] = useState(false)
  const [createForm, setCreateForm] = useState({ ...EMPTY_CREATE_FORM })
  const [createError, setCreateError] = useState('')
  const [createLoading, setCreateLoading] = useState(false)
  const [publicCompanies, setPublicCompanies] = useState<{ id: number; name: string }[]>([])

  useEffect(() => {
    if (showCreateModal && publicCompanies.length === 0) {
      authService.getPublicCompanies()
        .then(data => setPublicCompanies(data))
        .catch(() => {})
    }
  }, [showCreateModal])

  const openCreateModal = () => {
    setCreateForm({ ...EMPTY_CREATE_FORM })
    setCreateError('')
    setShowCreateModal(true)
  }

  const handleCreateUser = async (e: React.FormEvent) => {
    e.preventDefault()
    setCreateError('')

    const { name, email, password, userType, companyName, industry, selectedCompanyId, phoneNumber, country, province, city, location, address, jobTitle } = createForm

    if (userType === 'profesional') {
      if (!selectedCompanyId) { setCreateError('Debes seleccionar una empresa'); return }
      if (!phoneNumber.trim()) { setCreateError('El teléfono es obligatorio'); return }
      if (!country.trim()) { setCreateError('El país es obligatorio'); return }
      if (!jobTitle.trim()) { setCreateError('El rol o cargo es obligatorio'); return }
    }
    if (userType === 'empleador') {
      if (!companyName.trim()) { setCreateError('El nombre de la empresa es obligatorio'); return }
      if (!phoneNumber.trim()) { setCreateError('El teléfono es obligatorio'); return }
      if (!country.trim()) { setCreateError('El país es obligatorio'); return }
      if (!industry.trim()) { setCreateError('El rubro o industria es obligatorio'); return }
    }    setCreateLoading(true)
    try {
      await createUser({
        name,
        email,
        password,
        user_type: userType,
        company_name: userType === 'empleador' ? companyName : undefined,
        industry: userType === 'empleador' ? industry : undefined,
        empleador_id:
          userType === 'profesional' || userType === 'customer_success'
            ? (selectedCompanyId as number) || undefined
            : undefined,
        phone_number: phoneNumber || undefined,
        country: country || undefined,
        state: province || undefined,
        city: city || undefined,
        location: location || undefined,
        address: userType === 'empleador' ? address : undefined,
        job_title: userType === 'profesional' ? jobTitle : undefined,
      })
      setShowCreateModal(false)
    } catch (err: any) {
      setCreateError(err?.response?.data?.error || err?.response?.data?.message || err?.message || 'Error al crear el usuario')
    } finally {
      setCreateLoading(false)
    }
  }

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

  const USERS_PER_PAGE = 10

  const filteredUsers = (Array.isArray(users) ? users : [])
    .filter((u: any) => {
      const q = searchQuery.trim().toLowerCase()
      if (q && !(u.name?.toLowerCase().includes(q) || u.email?.toLowerCase().includes(q))) return false
      if (roleFilter && u.user_type !== roleFilter) return false
      // Una empresa "incluye" a su propia cuenta empleador y a sus vinculados.
      if (companyFilter !== '' && u.empleador_id !== companyFilter && u.id !== companyFilter) return false
      return true
    })
    .sort((a: any, b: any) => (a.name || '').localeCompare(b.name || '', 'es', { sensitivity: 'base' }))

  const totalUserPages = Math.max(1, Math.ceil(filteredUsers.length / USERS_PER_PAGE))
  const currentUsersPage = Math.min(usersPage, totalUserPages)
  const paginatedUsers = filteredUsers.slice((currentUsersPage - 1) * USERS_PER_PAGE, currentUsersPage * USERS_PER_PAGE)

  useEffect(() => {
    setUsersPage(1)
  }, [searchQuery, roleFilter, companyFilter])

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
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem', padding: '1.5rem' }}>
          <Skeleton height={48} width={280} radius={12} />
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: '1rem' }}>
            {Array.from({ length: 4 }).map((_, i) => <Skeleton key={i} height={96} radius={16} />)}
          </div>
          <Skeleton height={420} radius={16} />
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
                <ActivityFeed items={recentActivity.slice(0, 10)} />
              )}
            </div>
          </div>
        )}

        {activeTab === 'users' && (
          <div className={styles['users-tab']}>
            <div className={styles['tab-header']} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '16px', flexWrap: 'wrap' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'wrap', flex: 1 }}>
                <div className={styles['search-box']} data-tour="admin-search">
                  <Search size={18} />
                  <input
                    type="text"
                    placeholder="Buscar usuarios..."
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                  />
                </div>
                <div style={{ minWidth: 190 }} data-tour="admin-role-filter">
                  <Select
                    fullWidth
                    clearable
                    placeholder="Todos los roles"
                    value={roleFilter}
                    onChange={v => setRoleFilter(v ? String(v) : '')}
                    options={[
                      { value: 'profesional', label: 'Profesional' },
                      { value: 'empleador', label: 'Empresa' },
                      { value: 'customer_success', label: 'Customer Success' },
                      { value: 'analista_it', label: 'Analista de IT' },
                      { value: 'superadmin', label: 'Superadmin' },
                    ]}
                  />
                </div>
                <div style={{ minWidth: 210 }} data-tour="admin-company-filter">
                  <Select
                    fullWidth
                    clearable
                    placeholder="Todas las empresas"
                    value={companyFilter}
                    onChange={v => setCompanyFilter(v ? Number(v) : '')}
                    options={employers.map((emp: any) => ({ value: emp.id, label: emp.company_name || emp.name }))}
                  />
                </div>
                {(searchQuery.trim() || roleFilter || companyFilter !== '') && (
                  <button
                    type="button"
                    onClick={() => { setSearchQuery(''); setRoleFilter(''); setCompanyFilter('') }}
                    style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '9px 14px', border: '1px solid #e2e8f0', borderRadius: '10px', background: 'transparent', color: '#64748b', fontSize: '13px', fontWeight: 600, cursor: 'pointer', whiteSpace: 'nowrap' }}
                    title="Quitar todos los filtros"
                  >
                    <X size={14} /> Limpiar filtros
                  </button>
                )}
              </div>
              {canManage && (
              <button
                onClick={openCreateModal}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '8px',
                  padding: '10px 20px',
                  background: 'linear-gradient(135deg, var(--primary), var(--primary-dark, #1d4ed8))',
                  color: '#fff',
                  border: 'none',
                  borderRadius: '12px',
                  fontWeight: 700,
                  fontSize: '14px',
                  cursor: 'pointer',
                  boxShadow: '0 4px 12px rgba(59,130,246,0.3)',
                  transition: 'all 0.2s',
                  whiteSpace: 'nowrap',
                }}
                onMouseOver={e => (e.currentTarget.style.transform = 'translateY(-1px)')}
                onMouseOut={e => (e.currentTarget.style.transform = 'translateY(0)')}
              >
                <UserPlus size={16} />
                Crear Usuario
              </button>
              )}
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
                  {paginatedUsers.length === 0 && (
                    <tr>
                      <td colSpan={5} style={{ textAlign: 'center', padding: '32px 16px', color: '#94a3b8' }}>
                        No se encontraron usuarios con los filtros aplicados.
                      </td>
                    </tr>
                  )}
                  {paginatedUsers.map((u: any, index: number) => (
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
                          {canManage && (
                            <>
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
                            </>
                          )}
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {filteredUsers.length > 0 && (
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', marginTop: '14px' }} data-tour="admin-users-pagination">
                <span style={{ fontSize: '13px', color: '#64748b' }}>
                  Mostrando {(currentUsersPage - 1) * USERS_PER_PAGE + 1}–{Math.min(currentUsersPage * USERS_PER_PAGE, filteredUsers.length)} de {filteredUsers.length} usuarios
                </span>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <button
                    type="button"
                    className={styles['btn-icon']}
                    onClick={() => setUsersPage(p => Math.max(1, p - 1))}
                    disabled={currentUsersPage <= 1}
                    style={{ opacity: currentUsersPage <= 1 ? 0.4 : 1, cursor: currentUsersPage <= 1 ? 'not-allowed' : 'pointer' }}
                    title="Página anterior"
                  >
                    <ChevronLeft size={16} />
                  </button>
                  <span style={{ fontSize: '13px', fontWeight: 600, color: '#334155', whiteSpace: 'nowrap' }}>
                    Página {currentUsersPage} de {totalUserPages}
                  </span>
                  <button
                    type="button"
                    className={styles['btn-icon']}
                    onClick={() => setUsersPage(p => Math.min(totalUserPages, p + 1))}
                    disabled={currentUsersPage >= totalUserPages}
                    style={{ opacity: currentUsersPage >= totalUserPages ? 0.4 : 1, cursor: currentUsersPage >= totalUserPages ? 'not-allowed' : 'pointer' }}
                    title="Página siguiente"
                  >
                    <ChevronRight size={16} />
                  </button>
                </div>
              </div>
            )}
          </div>
        )}

        {activeTab === 'activity' && (
          <div className={styles['activity-tab']} data-tour="admin-activity-list">
            {recentActivity.length === 0 ? (
              <div className={styles['empty-state']}>
                <Activity size={40} />
                <p>No hay actividad registrada</p>
              </div>
            ) : (
              <ActivityFeed items={recentActivity} />
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

      {/* ===== MODAL CREAR USUARIO ===== */}
      {showCreateModal && (
        <div
          className={styles['modal-overlay']}
          onClick={() => setShowCreateModal(false)}
          style={{ position: 'fixed', inset: 0, zIndex: 1000, display: 'flex', alignItems: 'center', justifyContent: 'center' }}
        >
          <div
            onClick={e => e.stopPropagation()}
            style={{
              background: '#fff',
              borderRadius: '20px',
              padding: '32px',
              width: '100%',
              maxWidth: '560px',
              maxHeight: '90vh',
              overflowY: 'auto',
              boxShadow: '0 25px 50px -12px rgba(0,0,0,0.25)',
              border: '1px solid rgba(0,0,0,0.1)',
            }}
          >
            {/* Header */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '24px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                <div style={{ width: 40, height: 40, borderRadius: 12, background: 'linear-gradient(135deg, var(--primary), var(--primary-dark, #1d4ed8))', display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#fff' }}>
                  <UserPlus size={20} />
                </div>
                <div>
                  <h2 style={{ margin: 0, fontSize: '20px', fontWeight: 800, color: '#0f172a' }}>Crear Usuario</h2>
                  <p style={{ margin: 0, fontSize: '13px', color: '#64748b' }}>Completa todos los campos requeridos</p>
                </div>
              </div>
              <button
                onClick={() => setShowCreateModal(false)}
                style={{ background: 'none', border: 'none', cursor: 'pointer', color: '#94a3b8', padding: 4 }}
              >
                <X size={22} />
              </button>
            </div>

            {createError && (
              <div style={{ background: 'rgba(239, 68, 68, 0.12)', border: '1px solid rgba(239, 68, 68, 0.3)', color: '#dc2626', padding: '11px 14px', borderRadius: '10px', marginBottom: '18px', fontSize: '13px', fontWeight: 500 }}>
                {createError}
              </div>
            )}

            <form onSubmit={handleCreateUser}>
              {/* Nombre */}
              <div className={styles['form-group']}>
                <label style={{ display: 'block', marginBottom: '7px', fontWeight: 600, fontSize: '13px', color: '#334155' }}>
                  {createForm.userType === 'empleador' ? 'Nombre del dueño / Administrador' : 'Nombre completo'}
                </label>
                <input
                  type="text"
                  placeholder={createForm.userType === 'empleador' ? 'Ej: Juan Pérez' : 'Juan Pérez'}
                  value={createForm.name}
                  onChange={e => setCreateForm(f => ({ ...f, name: e.target.value }))}
                  required
                  style={{ width: '100%', padding: '11px 14px', border: '1px solid #cbd5e1', borderRadius: '10px', fontSize: '14px', color: '#0f172a', background: '#f8fafc' }}
                />
              </div>

              {/* Email */}
              <div className={styles['form-group']} style={{ marginTop: '16px' }}>
                <label style={{ display: 'block', marginBottom: '7px', fontWeight: 600, fontSize: '13px', color: '#334155' }}>Email</label>
                <input
                  type="email"
                  placeholder="juan@ejemplo.com"
                  value={createForm.email}
                  onChange={e => setCreateForm(f => ({ ...f, email: e.target.value }))}
                  required
                  style={{ width: '100%', padding: '11px 14px', border: '1px solid #cbd5e1', borderRadius: '10px', fontSize: '14px', color: '#0f172a', background: '#f8fafc' }}
                />
              </div>

              {/* Contraseña */}
              <div className={styles['form-group']} style={{ marginTop: '16px' }}>
                <label style={{ display: 'block', marginBottom: '7px', fontWeight: 600, fontSize: '13px', color: '#334155' }}>Contraseña</label>
                <input
                  type="password"
                  placeholder="Min. 8 caracteres con letras y números"
                  value={createForm.password}
                  onChange={e => setCreateForm(f => ({ ...f, password: e.target.value }))}
                  required
                  minLength={8}
                  style={{ width: '100%', padding: '11px 14px', border: '1px solid #cbd5e1', borderRadius: '10px', fontSize: '14px', color: '#0f172a', background: '#f8fafc' }}
                />
              </div>

              {/* Tipo de usuario */}
              <div className={styles['form-group']} style={{ marginTop: '16px' }}>
                <label style={{ display: 'block', marginBottom: '7px', fontWeight: 600, fontSize: '13px', color: '#334155' }}>Tipo de usuario</label>
                <Select
                  fullWidth
                  value={createForm.userType}
                  onChange={v => setCreateForm(f => ({ ...f, userType: String(v), companyName: '', industry: '', selectedCompanyId: '', phoneNumber: '', country: '', province: '', city: '', location: '', address: '', jobTitle: '' }))}
                  options={[
                    { value: 'profesional', label: 'Profesional (presta servicios)' },
                    { value: 'empleador', label: 'Empresa' },
                    { value: 'customer_success', label: 'Customer Success (Gestión de soporte)' },
                    { value: 'analista_it', label: 'Analista de IT (Soporte técnico)' },
                    { value: 'superadmin', label: 'Super Administrador (Control Total)' },
                  ]}
                />
              </div>

              {/* Campos de Profesional */}
              {createForm.userType === 'profesional' && (
                <div style={{ marginTop: '16px', display: 'flex', flexDirection: 'column', gap: '16px' }}>
                  <div>
                    <label style={{ display: 'block', marginBottom: '7px', fontWeight: 600, fontSize: '13px', color: '#334155' }}>Rol / Cargo (Ej: Desarrollador Backend...)</label>
                    <input
                      type="text"
                      placeholder="Ej: Desarrollador Fullstack"
                      value={createForm.jobTitle}
                      onChange={e => setCreateForm(f => ({ ...f, jobTitle: e.target.value }))}
                      required
                      style={{ width: '100%', padding: '11px 14px', border: '1px solid #cbd5e1', borderRadius: '10px', fontSize: '14px', color: '#0f172a', background: '#f8fafc' }}
                    />
                  </div>

                  <div>
                    <label style={{ display: 'block', marginBottom: '7px', fontWeight: 600, fontSize: '13px', color: '#334155' }}>Teléfono de contacto</label>
                    <input
                      type="tel"
                      placeholder="Ej: +34 600 000 000"
                      value={createForm.phoneNumber}
                      onChange={e => setCreateForm(f => ({ ...f, phoneNumber: e.target.value }))}
                      required
                      style={{ width: '100%', padding: '11px 14px', border: '1px solid #cbd5e1', borderRadius: '10px', fontSize: '14px', color: '#0f172a', background: '#f8fafc' }}
                    />
                  </div>

                  <div>
                    <label style={{ display: 'block', marginBottom: '7px', fontWeight: 600, fontSize: '13px', color: '#334155' }}>País</label>
                    <Select
                      fullWidth
                      value={createForm.country}
                      onChange={v => setCreateForm(f => ({ ...f, country: String(v), province: '' }))}
                      placeholder="Selecciona un país..."
                      options={[
                        { value: 'Argentina', label: 'Argentina' },
                        { value: 'España', label: 'España' },
                        { value: 'México', label: 'México' },
                        { value: 'Colombia', label: 'Colombia' },
                        { value: 'Chile', label: 'Chile' },
                        { value: 'Uruguay', label: 'Uruguay' },
                      ]}
                    />
                  </div>

                  <div>
                    <label style={{ display: 'block', marginBottom: '7px', fontWeight: 600, fontSize: '13px', color: '#334155' }}>Empresa a la que perteneces</label>
                    <Select
                      fullWidth
                      value={createForm.selectedCompanyId}
                      onChange={v => setCreateForm(f => ({ ...f, selectedCompanyId: Number(v) || '' }))}
                      placeholder="Selecciona una empresa..."
                      options={publicCompanies.map(c => ({ value: c.id, label: c.name }))}
                    />
                  </div>
                </div>
              )}

              {/* Campos de Empleador */}
              {createForm.userType === 'empleador' && (
                <div style={{ marginTop: '16px', display: 'flex', flexDirection: 'column', gap: '16px' }}>
                  <div>
                    <label style={{ display: 'block', marginBottom: '7px', fontWeight: 600, fontSize: '13px', color: '#334155' }}>Nombre de tu empresa</label>
                    <input
                      type="text"
                      placeholder="Mi Empresa S.A."
                      value={createForm.companyName}
                      onChange={e => setCreateForm(f => ({ ...f, companyName: e.target.value }))}
                      required
                      style={{ width: '100%', padding: '11px 14px', border: '1px solid #cbd5e1', borderRadius: '10px', fontSize: '14px', color: '#0f172a', background: '#f8fafc' }}
                    />
                  </div>

                  <div>
                    <label style={{ display: 'block', marginBottom: '7px', fontWeight: 600, fontSize: '13px', color: '#334155' }}>Rubro o industria</label>
                    <input
                      type="text"
                      placeholder="Ej: Tecnología, Marketing..."
                      value={createForm.industry}
                      onChange={e => setCreateForm(f => ({ ...f, industry: e.target.value }))}
                      required
                      style={{ width: '100%', padding: '11px 14px', border: '1px solid #cbd5e1', borderRadius: '10px', fontSize: '14px', color: '#0f172a', background: '#f8fafc' }}
                    />
                  </div>

                  <div>
                    <label style={{ display: 'block', marginBottom: '7px', fontWeight: 600, fontSize: '13px', color: '#334155' }}>Teléfono de contacto de la empresa</label>
                    <input
                      type="tel"
                      placeholder="Ej: +34 600 000 000"
                      value={createForm.phoneNumber}
                      onChange={e => setCreateForm(f => ({ ...f, phoneNumber: e.target.value }))}
                      required
                      style={{ width: '100%', padding: '11px 14px', border: '1px solid #cbd5e1', borderRadius: '10px', fontSize: '14px', color: '#0f172a', background: '#f8fafc' }}
                    />
                  </div>

                  <div>
                    <label style={{ display: 'block', marginBottom: '7px', fontWeight: 600, fontSize: '13px', color: '#334155' }}>País</label>
                    <Select
                      fullWidth
                      value={createForm.country}
                      onChange={v => setCreateForm(f => ({ ...f, country: String(v), province: '' }))}
                      placeholder="Selecciona un país..."
                      options={[
                        { value: 'Argentina', label: 'Argentina' },
                        { value: 'España', label: 'España' },
                        { value: 'México', label: 'México' },
                        { value: 'Colombia', label: 'Colombia' },
                        { value: 'Chile', label: 'Chile' },
                        { value: 'Uruguay', label: 'Uruguay' },
                      ]}
                    />
                  </div>
                </div>
              )}

              {/* Ciudad y ubicación (opcionales) para profesional y empresa */}
              {(createForm.userType === 'profesional' || createForm.userType === 'empleador') && (
                <div style={{ marginTop: '16px', display: 'flex', flexDirection: 'column', gap: '16px' }}>
                  <div>
                    <label style={{ display: 'block', marginBottom: '7px', fontWeight: 600, fontSize: '13px', color: '#334155' }}>Ciudad (opcional)</label>
                    <input
                      type="text"
                      placeholder="Ej: Buenos Aires"
                      value={createForm.city}
                      onChange={e => setCreateForm(f => ({ ...f, city: e.target.value }))}
                      style={{ width: '100%', padding: '11px 14px', border: '1px solid #cbd5e1', borderRadius: '10px', fontSize: '14px', color: '#0f172a', background: '#f8fafc' }}
                    />
                  </div>
                  <div>
                    <label style={{ display: 'block', marginBottom: '7px', fontWeight: 600, fontSize: '13px', color: '#334155' }}>Ubicación (opcional)</label>
                    <input
                      type="text"
                      placeholder="Ej: Ciudad, provincia o región"
                      value={createForm.location}
                      onChange={e => setCreateForm(f => ({ ...f, location: e.target.value }))}
                      style={{ width: '100%', padding: '11px 14px', border: '1px solid #cbd5e1', borderRadius: '10px', fontSize: '14px', color: '#0f172a', background: '#f8fafc' }}
                    />
                  </div>
                </div>
              )}

              {/* Campos de Customer Success */}
              {createForm.userType === 'customer_success' && (
                <div className={styles['form-group']} style={{ marginTop: '16px' }}>
                  <label style={{ display: 'block', marginBottom: '7px', fontWeight: 600, fontSize: '13px', color: '#334155' }}>Empresa asignada (opcional)</label>
                  <Select
                    fullWidth
                    clearable
                    value={createForm.selectedCompanyId}
                    onChange={v => setCreateForm(f => ({ ...f, selectedCompanyId: Number(v) || '' }))}
                    placeholder="Selecciona una empresa..."
                    options={publicCompanies.map(c => ({ value: c.id, label: c.name }))}
                  />
                  <p style={{ margin: '6px 0 0', fontSize: '12px', color: '#94a3b8' }}>
                    Vincula esta cuenta de soporte a una empresa concreta, o déjala vacía para soporte global.
                  </p>
                </div>
              )}

              {/* Botones */}
              <div style={{ display: 'flex', gap: '12px', marginTop: '24px' }}>
                <button
                  type="button"
                  onClick={() => setShowCreateModal(false)}
                  style={{ flex: 1, padding: '12px', borderRadius: '12px', border: '1px solid #e2e8f0', background: '#f8fafc', fontWeight: 600, cursor: 'pointer', color: '#475569' }}
                >
                  Cancelar
                </button>
                <button
                  type="submit"
                  disabled={createLoading}
                  style={{ flex: 1, padding: '12px', borderRadius: '12px', border: 'none', background: 'linear-gradient(135deg, var(--primary), var(--primary-dark, #1d4ed8))', color: '#fff', fontWeight: 700, cursor: createLoading ? 'not-allowed' : 'pointer', opacity: createLoading ? 0.7 : 1 }}
                >
                  {createLoading ? 'Creando...' : 'Crear Usuario'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}    </div>
  )
}
