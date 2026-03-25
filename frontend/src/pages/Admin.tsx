import { useState, useEffect } from 'react'
import { adminService } from '../services/api'
import { useAuth } from '../context/AuthContext'
import type { User } from '../types'
import { AdminDashboard } from '../components/Admin/AdminDashboard'
import { CompanyTable } from '../components/Admin/CompanyTable'
import { UserTable } from '../components/Admin/UserTable'
import { InactiveUserList } from '../components/Admin/InactiveUserList'
import { UserModal } from '../components/Admin/Modals/UserModal'
import './Admin.css'

export default function Admin() {
  useAuth() // initialize auth
  const [metrics, setMetrics] = useState<any | null>(null)
  const [companies, setCompanies] = useState<any[]>([])
  const [employers, setEmployers] = useState<User[]>([])
  const [managers, setManagers] = useState<User[]>([])
  const [inactiveUsers, setInactiveUsers] = useState<any[]>([])
  const [activities, setActivities] = useState<any[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [activeTab, setActiveTab] = useState<'dashboard' | 'companies' | 'users' | 'inactive'>('dashboard')
  const [isLoading, setIsLoading] = useState(true)
  const [showUserModal, setShowUserModal] = useState(false)
  const [editingUser, setEditingUser] = useState<User | null>(null)
  const [showEditModal, setShowEditModal] = useState(false)
  const [newPassword, setNewPassword] = useState('')

  const [editForm, setEditForm] = useState({
    name: '',
    email: '',
    job_title: '',
    phone_number: '',
    country: '',
    city: '',
    is_active: true,
    is_manager: false,
    empleador_id: undefined as number | undefined,
    manager_id: undefined as number | undefined,
  })

  const [userForm, setUserForm] = useState({
    name: '',
    email: '',
    password: '',
    user_type: 'profesional',
    company_name: '',
    job_title: '',
    empleador_id: undefined as number | undefined,
    manager_id: undefined as number | undefined,
    is_manager: false,
  })

  useEffect(() => {
    fetchDashboard()
  }, [])

  const fetchDashboard = async () => {
    setIsLoading(true)
    try {
      const [metricsRes, companiesRes, employersRes, inactiveRes, activityRes, usersRes, managersRes] = await Promise.all([
        adminService.getDashboard(),
        adminService.getCompanies(),
        adminService.getUsers({ user_type: 'empleador' }),
        adminService.getInactiveUsers(7),
        adminService.getRecentActivity(),
        adminService.getUsers({}),
        adminService.getUsers({ is_manager: 'true' }),
      ])
      setMetrics(metricsRes)
      setCompanies(companiesRes || [])
      setEmployers(employersRes?.data || [])
      setManagers(managersRes?.data || [])
      setInactiveUsers(inactiveRes || [])
      setActivities(activityRes || [])
      setUsers(usersRes?.data || [])
    } catch (error) {
      console.error('Error fetching admin data:', error)
    } finally {
      setIsLoading(false)
    }
  }

  const handleCreateUser = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await adminService.createUser(userForm)
      setShowUserModal(false)
      resetForm()
      fetchDashboard()
    } catch (error) { console.error('Error creating user:', error) }
  }

  const handleDeleteUser = async (id: number) => {
    if (!confirm('¿Estás seguro de eliminar este usuario?')) return
    try { await adminService.deleteUser(id); fetchDashboard() } catch (error) { console.error('Error deleting user:', error) }
  }

  const handleToggleUserStatus = async (userData: User) => {
    try { await adminService.updateUser(userData.id, { is_active: !userData.is_active }); fetchDashboard() } catch (error) { console.error('Error toggling user status:', error) }
  }

  const handleEditUser = (userData: User) => {
    setEditingUser(userData)
    setEditForm({
      name: userData.name || '',
      email: userData.email || '',
      job_title: userData.job_title || '',
      phone_number: userData.phone_number || '',
      country: userData.country || '',
      city: userData.city || '',
      is_active: userData.is_active !== false,
      is_manager: userData.is_manager || false,
      empleador_id: userData.empleador_id || undefined,
      manager_id: userData.manager_id || undefined,
    })
    setNewPassword(''); setShowEditModal(true)
  }

  const handleSaveEdit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!editingUser) return
    try { await adminService.updateUser(editingUser.id, editForm); setShowEditModal(false); setEditingUser(null); fetchDashboard() } catch (error) { console.error('Error updating user:', error) }
  }

  const handleResetPassword = async () => {
    if (!editingUser || !newPassword) return
    try { await adminService.resetPassword(editingUser.id, newPassword); alert('Contraseña restablecida exitosamente'); setNewPassword('') } catch (error: any) { alert(error?.response?.data?.error || 'Error al restablecer contraseña') }
  }

  const resetForm = () => {
    setUserForm({
      name: '', email: '', password: '', user_type: 'profesional',
      company_name: '', job_title: '', empleador_id: undefined, manager_id: undefined, is_manager: false,
    })
  }

  const getRoleColor = (userType: string, isManager: boolean, isSuperadmin: boolean) => {
    if (isSuperadmin) return '#8b5cf6'
    if (isManager) return '#f59e0b'
    if (userType === 'empleador') return '#3b82f6'
    return '#10b981'
  }

  const getRoleLabel = (userType: string, isManager: boolean, isSuperadmin: boolean) => {
    if (isSuperadmin) return 'Super Admin'
    if (isManager) return 'Manager'
    if (userType === 'empleador') return 'Empresa'
    return 'Profesional'
  }

  if (isLoading) {
    return (
      <div className="admin-loading">
        <div className="spinner" />
        <p>Cargando panel de administración...</p>
      </div>
    )
  }

  return (
    <div className="admin-page">
      <div className="admin-header">
        <div>
          <h1>Panel de Administración</h1>
          <p className="admin-subtitle">Monitor de Obertrack - Customer Success</p>
        </div>
        <button className="btn-primary" onClick={() => { resetForm(); setShowUserModal(true) }}>
          + Nuevo Usuario
        </button>
      </div>

      <div className="admin-tabs">
        <button className={`tab-btn ${activeTab === 'dashboard' ? 'active' : ''}`} onClick={() => setActiveTab('dashboard')}>📊 Dashboard</button>
        <button className={`tab-btn ${activeTab === 'companies' ? 'active' : ''}`} onClick={() => setActiveTab('companies')}>🏢 Empresas</button>
        <button className={`tab-btn ${activeTab === 'users' ? 'active' : ''}`} onClick={() => setActiveTab('users')}>👥 Usuarios</button>
        <button className={`tab-btn ${activeTab === 'inactive' ? 'active' : ''}`} onClick={() => setActiveTab('inactive')}>
          ⚠️ Inactivos {inactiveUsers.length > 0 && <span className="badge">{inactiveUsers.length}</span>}
        </button>
      </div>

      {activeTab === 'dashboard' && metrics && (
        <AdminDashboard
          metrics={metrics}
          activities={activities}
          onViewInactive={() => setActiveTab('inactive')}
        />
      )}

      {activeTab === 'companies' && <CompanyTable companies={companies} />}

      {activeTab === 'users' && (
        <UserTable
          users={users}
          employers={employers}
          onEdit={handleEditUser}
          onToggleStatus={handleToggleUserStatus}
          onDelete={handleDeleteUser}
          getRoleColor={getRoleColor}
          getRoleLabel={getRoleLabel}
        />
      )}

      {activeTab === 'inactive' && <InactiveUserList inactiveUsers={inactiveUsers} />}

      {showUserModal && (
        <UserModal
          title="Nuevo Usuario"
          mode="create"
          form={userForm}
          setForm={setUserForm}
          employers={employers}
          managers={managers}
          onClose={() => { setShowUserModal(false); resetForm() }}
          onSubmit={handleCreateUser}
        />
      )}

      {showEditModal && editingUser && (
        <UserModal
          title="Editar Usuario"
          mode="edit"
          form={editForm}
          setForm={setEditForm}
          employers={employers}
          managers={managers}
          onClose={() => setShowEditModal(false)}
          onSubmit={handleSaveEdit}
          onResetPassword={handleResetPassword}
          newPassword={newPassword}
          setNewPassword={setNewPassword}
        />
      )}
    </div>
  )
}
