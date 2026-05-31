import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Building2, Users, Search, Plus, Ban, CheckCircle2, ChevronRight, X } from 'lucide-react'
import { useTenants } from '../../hooks'
import { adminService } from '../../services/api'
import type { Tenant, User } from '../../types'
import Avatar from '../../components/Common/Avatar'
import styles from './Tenants.module.css'

export default function TenantsList() {
  const navigate = useNavigate()
  const { tenants, isLoading, error, createTenant, suspendTenant, activateTenant } = useTenants()

  const [search, setSearch] = useState('')
  const [showCreate, setShowCreate] = useState(false)
  const [companyName, setCompanyName] = useState('')
  const [responsableQuery, setResponsableQuery] = useState('')
  const [selectedUser, setSelectedUser] = useState<User | null>(null)
  const [users, setUsers] = useState<User[]>([])
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  const filtered = tenants.filter(t =>
    t.company_name?.toLowerCase().includes(search.toLowerCase()) ||
    t.owner_email?.toLowerCase().includes(search.toLowerCase())
  )

  const activeCount = tenants.filter(t => t.is_active).length
  const totalUsers = tenants.reduce((sum, t) => sum + (t.user_count || 0), 0)

  useEffect(() => {
    if (!showCreate) return
    let active = true
    adminService.getUsers({ limit: 1000 })
      .then(res => {
        const arr = res?.data || (Array.isArray(res) ? res : [])
        if (active) setUsers(arr)
      })
      .catch(() => {})
    return () => { active = false }
  }, [showCreate])

  const suggestions = responsableQuery.trim().length === 0
    ? []
    : users.filter(u =>
        !u.is_superadmin &&
        u.user_type !== 'empleador' &&
        (u.name?.toLowerCase().includes(responsableQuery.toLowerCase()) ||
         u.email?.toLowerCase().includes(responsableQuery.toLowerCase()))
      ).slice(0, 6)

  const closeCreate = () => {
    setShowCreate(false)
    setCompanyName('')
    setResponsableQuery('')
    setSelectedUser(null)
    setFormError(null)
  }

  const handleCreate = async () => {
    if (!selectedUser) return
    setSubmitting(true)
    setFormError(null)
    try {
      await createTenant({ company_name: companyName, user_id: selectedUser.id })
      closeCreate()
    } catch (err: any) {
      setFormError(err?.response?.data?.error || 'No se pudo crear la empresa')
    } finally {
      setSubmitting(false)
    }
  }

  const handleToggle = async (e: React.MouseEvent, t: Tenant) => {
    e.stopPropagation()
    if (t.is_active) {
      await suspendTenant(t.id)
    } else {
      await activateTenant(t.id)
    }
  }

  if (isLoading) {
    return (
      <div className={styles.page}>
        <div className={styles.loading}>
          <div className={styles.spinner} />
          <p>Cargando empresas...</p>
        </div>
      </div>
    )
  }

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1>Empresas</h1>
          <p>Gestiona los clientes de la plataforma Oberstaff</p>
        </div>
        <button className={styles.primaryBtn} onClick={() => setShowCreate(true)}>
          <Plus size={18} />
          Nueva empresa
        </button>
      </div>

      <div className={styles.kpis}>
        <div className={styles.kpiCard}>
          <div className={styles.kpiIcon} style={{ background: 'linear-gradient(135deg, var(--primary), var(--primary-dark))' }}>
            <Building2 size={24} />
          </div>
          <div>
            <span className={styles.kpiValue}>{tenants.length}</span>
            <span className={styles.kpiLabel}>Empresas</span>
          </div>
        </div>
        <div className={styles.kpiCard}>
          <div className={styles.kpiIcon} style={{ background: 'linear-gradient(135deg, #10b981, #059669)' }}>
            <CheckCircle2 size={24} />
          </div>
          <div>
            <span className={styles.kpiValue}>{activeCount}</span>
            <span className={styles.kpiLabel}>Activas</span>
          </div>
        </div>
        <div className={styles.kpiCard}>
          <div className={styles.kpiIcon} style={{ background: 'linear-gradient(135deg, #ef4444, #b91c1c)' }}>
            <Ban size={24} />
          </div>
          <div>
            <span className={styles.kpiValue}>{tenants.length - activeCount}</span>
            <span className={styles.kpiLabel}>Suspendidas</span>
          </div>
        </div>
        <div className={styles.kpiCard}>
          <div className={styles.kpiIcon} style={{ background: 'linear-gradient(135deg, #8b5cf6, #7c3aed)' }}>
            <Users size={24} />
          </div>
          <div>
            <span className={styles.kpiValue}>{totalUsers}</span>
            <span className={styles.kpiLabel}>Usuarios totales</span>
          </div>
        </div>
      </div>

      <div className={styles.searchBox}>
        <Search size={18} />
        <input
          type="text"
          placeholder="Buscar empresa o correo..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </div>

      {error && <p className={styles.errorMsg}>{error}</p>}

      {filtered.length === 0 ? (
        <div className={styles.empty}>
          <Building2 size={40} />
          <p>No hay empresas que coincidan</p>
        </div>
      ) : (
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>Empresa</th>
                <th>Responsable</th>
                <th>Usuarios</th>
                <th>Tableros</th>
                <th>Tareas</th>
                <th>Estado</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {filtered.map(t => (
                <tr key={t.id} className={styles.row} onClick={() => navigate(`/admin/tenants/${t.id}`)}>
                  <td>
                    <div className={styles.companyCell}>
                      <div className={styles.companyLogo}>{t.company_name?.charAt(0).toUpperCase() || '?'}</div>
                      <span>{t.company_name}</span>
                    </div>
                  </td>
                  <td>
                    <div className={styles.ownerCell}>
                      <span>{t.owner_name}</span>
                      <small>{t.owner_email}</small>
                    </div>
                  </td>
                  <td>{t.user_count}</td>
                  <td>{t.board_count}</td>
                  <td>{t.task_count}</td>
                  <td>
                    <span className={`${styles.badge} ${t.is_active ? styles.badgeActive : styles.badgeSuspended}`}>
                      {t.is_active ? 'Activa' : 'Suspendida'}
                    </span>
                  </td>
                  <td>
                    <div className={styles.rowActions}>
                      <button
                        className={`${styles.iconBtn} ${t.is_active ? styles.danger : styles.success}`}
                        onClick={(e) => handleToggle(e, t)}
                        title={t.is_active ? 'Suspender empresa' : 'Reactivar empresa'}
                      >
                        {t.is_active ? <Ban size={16} /> : <CheckCircle2 size={16} />}
                      </button>
                      <ChevronRight size={18} className={styles.chevron} />
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showCreate && (
        <div className={styles.modalOverlay} onClick={closeCreate}>
          <div className={styles.modal} onClick={(e) => e.stopPropagation()}>
            <div className={styles.modalHeader}>
              <h2>Nueva empresa</h2>
              <button className={styles.closeBtn} onClick={closeCreate}><X size={20} /></button>
            </div>
            <p className={styles.modalHint}>Selecciona un usuario existente como responsable (empleador) de la empresa.</p>
            <div className={styles.field}>
              <label>Nombre de la empresa</label>
              <input value={companyName} onChange={(e) => setCompanyName(e.target.value)} placeholder="Acme S.A." />
            </div>
            <div className={styles.field}>
              <label>Responsable</label>
              {selectedUser ? (
                <div className={styles.selectedChip}>
                  <Avatar src={selectedUser.avatar} name={selectedUser.name} size="sm" />
                  <div className={styles.ownerCell}>
                    <span>{selectedUser.name}</span>
                    <small>{selectedUser.email}</small>
                  </div>
                  <button className={styles.chipClear} onClick={() => { setSelectedUser(null); setResponsableQuery('') }}><X size={16} /></button>
                </div>
              ) : (
                <div className={styles.suggestWrap}>
                  <div className={styles.searchBox} style={{ margin: 0, maxWidth: 'none' }}>
                    <Search size={18} />
                    <input
                      type="text"
                      value={responsableQuery}
                      onChange={(e) => setResponsableQuery(e.target.value)}
                      placeholder="Busca por nombre o correo..."
                    />
                  </div>
                  {suggestions.length > 0 && (
                    <div className={styles.suggestBox}>
                      {suggestions.map(u => (
                        <button key={u.id} className={styles.suggestItem} onClick={() => { setSelectedUser(u); setResponsableQuery('') }}>
                          <Avatar src={u.avatar} name={u.name} size="sm" />
                          <div className={styles.ownerCell}>
                            <span>{u.name}</span>
                            <small>{u.email} · {u.is_manager ? 'manager' : u.user_type}</small>
                          </div>
                        </button>
                      ))}
                    </div>
                  )}
                  {responsableQuery.trim().length > 0 && suggestions.length === 0 && (
                    <p className={styles.suggestEmpty}>Sin usuarios que coincidan (se excluyen superadmins y empresas).</p>
                  )}
                </div>
              )}
            </div>
            {formError && <p className={styles.errorMsg}>{formError}</p>}
            <div className={styles.modalActions}>
              <button className={styles.secondaryBtn} onClick={closeCreate} disabled={submitting}>Cancelar</button>
              <button
                className={styles.primaryBtn}
                onClick={handleCreate}
                disabled={submitting || !companyName || !selectedUser}
              >
                {submitting ? 'Creando...' : 'Crear empresa'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
