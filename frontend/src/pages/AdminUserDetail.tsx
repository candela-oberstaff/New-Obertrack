import { useState, useEffect, useCallback } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, UserX, Power, KeyRound, Shield, UserCog, Pencil } from 'lucide-react'
import { userService, adminService } from '../services/api'
import { rbacService } from '../services/rbac.service'
import { useAuth } from '../context/AuthContext'
import type { User, CompanyRole, CompanyGroup } from '../types'
import Avatar from '../components/Common/Avatar'
import { Skeleton } from '../components/ui'
import { UserModal } from '../components/Admin/Modals/UserModal'
import styles from './AdminUserDetail.module.css'

// Sin caracteres ambiguos (0/O, 1/l/I) para que sea fácil de dictar.
function generateTempPassword(): string {
  const chars = 'ABCDEFGHJKMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789'
  const bytes = crypto.getRandomValues(new Uint8Array(12))
  let pw = Array.from(bytes).map(b => chars[b % chars.length]).join('')
  if (!/\d/.test(pw)) pw = pw.slice(0, -1) + '7'
  if (!/[a-zA-Z]/.test(pw)) pw = pw.slice(0, -1) + 'k'
  return pw
}

// Tenant al que pertenece el usuario (la cuenta empresa es su propio tenant).
const tenantIdForUser = (u: User) =>
  u.user_type === 'empleador' ? u.id : (u.empleador_id || 0)

export default function AdminUserDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const qc = useQueryClient()
  // Invalida las vistas del panel admin (lista de usuarios, dashboard, etc.)
  // para que reflejen los cambios hechos desde el detalle (no comparten caché).
  const invalidateAdmin = useCallback(() => qc.invalidateQueries({ queryKey: ['admin'] }), [qc])
  const { user: viewer } = useAuth()
  // CS entra en modo consulta: sin acciones ni gestión de roles/grupos.
  const canManage = !!viewer?.is_superadmin
  const [user, setUser] = useState<User | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [actionMsg, setActionMsg] = useState<string | null>(null)
  const [empresaName, setEmpresaName] = useState('')
  const [managerName, setManagerName] = useState('')

  // Edición del usuario desde el detalle (reutiliza el modal del panel).
  const [showEdit, setShowEdit] = useState(false)
  const [editForm, setEditForm] = useState<any>({})
  const [editError, setEditError] = useState<string | null>(null)
  const [editPeople, setEditPeople] = useState<{ employers: User[]; managers: User[] }>({ employers: [], managers: [] })

  // Roles y grupos del usuario (solo miembros de un tenant: profesional / CS con empresa)
  const [tenantRoles, setTenantRoles] = useState<CompanyRole[]>([])
  const [tenantGroups, setTenantGroups] = useState<CompanyGroup[]>([])
  const [userRoleIds, setUserRoleIds] = useState<Set<number>>(new Set())
  const [userGroupIds, setUserGroupIds] = useState<Set<number>>(new Set())
  const [rbacBusy, setRbacBusy] = useState(false)

  const loadRBAC = useCallback(async (u: User) => {
    const tid = tenantIdForUser(u)
    if (!tid || u.is_superadmin || u.user_type === 'empleador') return
    try {
      const [rolesList, groupsList, mine] = await Promise.all([
        rbacService.listRoles(tid),
        rbacService.listGroups(tid),
        rbacService.getUserRBAC(u.id, tid),
      ])
      setTenantRoles(rolesList)
      setTenantGroups(groupsList)
      setUserRoleIds(new Set(mine.roles.map(r => r.id)))
      setUserGroupIds(new Set(mine.groups.map(g => g.id)))
    } catch { /* sección opcional: si falla, simplemente no se muestra */ }
  }, [])

  const load = useCallback(async () => {
    setIsLoading(true)
    try {
      const data = await userService.getById(Number(id))
      setUser(data)
      setError(null)
      loadRBAC(data)
      // Resolve related names (employer / manager) for professionals and customer success.
      setEmpresaName(''); setManagerName('')
      if (data.user_type === 'profesional' || data.user_type === 'customer_success') {
        if (data.empleador_id) {
          userService.getById(data.empleador_id).then(e => setEmpresaName(e?.company_name || e?.name || '')).catch(() => {})
        }
        if (data.manager_id) {
          userService.getById(data.manager_id).then(m => setManagerName(m?.name || '')).catch(() => {})
        }
      }
    } catch {
      setError('No se pudo cargar el usuario')
    } finally {
      setIsLoading(false)
    }
  }, [id, loadRBAC])

  useEffect(() => { load() }, [load])

  const toggleActive = async () => {
    if (!user) return
    setBusy(true); setActionMsg(null)
    try {
      await adminService.updateUser(user.id, { is_active: !user.is_active })
      await load()
      invalidateAdmin()
    } catch { setActionMsg('No se pudo cambiar el estado.') } finally { setBusy(false) }
  }

  const promote = async () => {
    if (!user) return
    setBusy(true); setActionMsg(null)
    try {
      await adminService.updateUser(user.id, { is_manager: true })
      await load()
      invalidateAdmin()
    } catch { setActionMsg('No se pudo promover.') } finally { setBusy(false) }
  }

  const resetPass = async () => {
    if (!user) return
    setBusy(true); setActionMsg(null)
    try {
      const temp = generateTempPassword()
      await adminService.resetPassword(user.id, temp)
      setActionMsg(`Contraseña reseteada. Temporal: ${temp} — compártela por un canal seguro, no volverá a mostrarse.`)
    } catch { setActionMsg('No se pudo resetear la contraseña.') } finally { setBusy(false) }
  }

  const openEdit = async () => {
    if (!user) return
    setEditForm({
      name: user.name || '',
      email: user.email || '',
      user_type: user.user_type || '',
      job_title: user.job_title || '',
      phone_number: user.phone_number || '',
      country: user.country || '',
      state: user.state || '',
      city: user.city || '',
      location: user.location || '',
      company_name: user.company_name || '',
      empleador_id: user.empleador_id || '',
      manager_id: user.manager_id || '',
      is_active: user.is_active,
      is_manager: user.is_manager,
    })
    setEditError(null)
    setShowEdit(true)
    // Carga diferida de empresas y managers para los selectores del modal.
    try {
      const res: any = await adminService.getUsers({ limit: 1000 })
      const all: User[] = res?.data || (Array.isArray(res) ? res : [])
      setEditPeople({
        employers: all.filter(u => u.user_type === 'empleador'),
        managers: all.filter(u => u.is_manager),
      })
    } catch { /* selectores vacíos si falla */ }
  }

  const handleEditSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!user) return
    setEditError(null)
    // Sanitiza FKs: número positivo o null (nunca "" — el backend lo rechaza).
    const payload = {
      ...editForm,
      empleador_id: editForm.empleador_id ? Number(editForm.empleador_id) : null,
      manager_id: editForm.manager_id ? Number(editForm.manager_id) : null,
    }
    try {
      await adminService.updateUser(user.id, payload)
      setShowEdit(false)
      await load()
      invalidateAdmin()
    } catch (err: any) {
      setEditError(err?.response?.data?.error ?? 'No se pudieron guardar los cambios.')
    }
  }

  if (isLoading) {
    return (
      <div className={styles.page}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem', padding: '1.5rem' }}>
          <Skeleton height={120} radius={16} />
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '1rem' }}>
            {Array.from({ length: 3 }).map((_, i) => <Skeleton key={i} height={140} radius={16} />)}
          </div>
        </div>
      </div>
    )
  }

  if (error || !user) {
    return (
      <div className={styles.page}>
        <button className={styles.backBtn} onClick={() => navigate('/admin')}>
          <ArrowLeft size={18} /> Volver
        </button>
        <div className={styles.empty}>
          <UserX size={40} />
          <p>{error || 'Usuario no encontrado'}</p>
        </div>
      </div>
    )
  }

  const rol = user.is_superadmin ? 'Superadmin' : user.is_manager ? 'Manager' : user.user_type

  const rbacApplicable = canManage && !!tenantIdForUser(user) && !user.is_superadmin && user.user_type !== 'empleador'

  const toggleRole = async (roleId: number, has: boolean) => {
    const tid = tenantIdForUser(user)
    setRbacBusy(true)
    try {
      if (has) await rbacService.unassignRole(roleId, user.id, tid)
      else await rbacService.assignRole(roleId, [user.id], tid)
      setUserRoleIds(prev => {
        const next = new Set(prev)
        if (has) next.delete(roleId)
        else next.add(roleId)
        return next
      })
    } catch {
      setActionMsg('No se pudo actualizar el rol.')
    } finally {
      setRbacBusy(false)
    }
  }

  const toggleGroup = async (groupId: number, has: boolean) => {
    const tid = tenantIdForUser(user)
    setRbacBusy(true)
    try {
      if (has) await rbacService.removeGroupMember(groupId, user.id, tid)
      else await rbacService.addGroupMembers(groupId, [user.id], tid)
      setUserGroupIds(prev => {
        const next = new Set(prev)
        if (has) next.delete(groupId)
        else next.add(groupId)
        return next
      })
    } catch {
      setActionMsg('No se pudo actualizar el grupo.')
    } finally {
      setRbacBusy(false)
    }
  }

  const chipStyle = (selected: boolean): React.CSSProperties => ({
    display: 'inline-flex',
    alignItems: 'center',
    gap: '6px',
    padding: '6px 14px',
    borderRadius: '999px',
    border: selected ? '1px solid var(--primary, #cc33cc)' : '1px solid var(--border, #cbd5e1)',
    background: selected ? 'rgba(204, 51, 204, 0.1)' : 'var(--bg-primary, #fff)',
    color: selected ? 'var(--primary, #cc33cc)' : '#475569',
    fontSize: '0.83rem',
    fontWeight: 600,
    cursor: rbacBusy ? 'wait' : 'pointer',
    opacity: rbacBusy ? 0.6 : 1,
  })

  // Fields common to every user type.
  const common: { label: string; value: React.ReactNode }[] = [
    { label: 'Teléfono', value: user.phone_number || '—' },
    { label: 'País', value: user.country || '—' },
    { label: 'Estado / Provincia', value: user.state || '—' },
    { label: 'Ciudad', value: user.city || '—' },
    { label: 'Ubicación', value: user.location || '—' },
    { label: 'Registrado', value: user.created_at ? new Date(user.created_at).toLocaleString('es-ES') : '—' },
    { label: 'Actualizado', value: user.updated_at ? new Date(user.updated_at).toLocaleString('es-ES') : '—' },
  ]

  // Type-specific fields: only show what's relevant for each user type.
  let specific: { label: string; value: React.ReactNode }[] = []
  if (user.user_type === 'empleador') {
    specific = [{ label: 'Empresa', value: user.company_name || '—' }]
  } else if (user.user_type === 'profesional') {
    specific = [
      { label: 'Cargo', value: user.job_title || '—' },
      { label: 'Empresa', value: empresaName || '—' },
      { label: 'Manager', value: managerName || 'Sin asignar' },
    ]
  } else if (user.user_type === 'customer_success') {
    specific = [{ label: 'Empresa asignada', value: empresaName || 'Soporte global' }]
  }

  const fields = [...specific, ...common]

  return (
    <div className={styles.page}>
      <button className={styles.backBtn} onClick={() => navigate('/admin')}>
        <ArrowLeft size={18} /> Usuarios
      </button>

      <div className={styles.header}>
        <Avatar src={user.avatar} name={user.name} size="xl" />
        <div>
          <div className={styles.titleRow}>
            <h1>{user.name}</h1>
            <span className={`${styles.status} ${user.is_active ? styles.active : styles.inactive}`}>
              {user.is_active ? 'Activo' : 'Inactivo'}
            </span>
          </div>
          <p className={styles.email}>{user.email}</p>
          <div className={styles.tags}>
            <span className={`${styles.tag} ${user.is_superadmin ? styles.tagAdmin : ''}`}>{rol}</span>
            {user.is_manager && !user.is_superadmin && <span className={`${styles.tag} ${styles.tagManager}`}>Manager</span>}
          </div>
        </div>
      </div>

      {canManage && (
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.5rem', margin: '0 0 1rem' }}>
        <button onClick={openEdit} disabled={busy}
          style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', padding: '0.55rem 1rem', borderRadius: '10px', border: 'none', background: 'var(--primary, #cc33cc)', color: '#fff', fontWeight: 600, cursor: 'pointer', fontSize: '0.85rem' }}>
          <Pencil size={15} /> Editar
        </button>
        <button onClick={toggleActive} disabled={busy}
          style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', padding: '0.55rem 1rem', borderRadius: '10px', border: '1px solid var(--border, #cbd5e1)', background: 'var(--bg-primary, #fff)', fontWeight: 600, cursor: 'pointer', fontSize: '0.85rem' }}>
          <Power size={15} /> {user.is_active ? 'Desactivar' : 'Activar'}
        </button>
        <button onClick={resetPass} disabled={busy}
          style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', padding: '0.55rem 1rem', borderRadius: '10px', border: '1px solid var(--border, #cbd5e1)', background: 'var(--bg-primary, #fff)', fontWeight: 600, cursor: 'pointer', fontSize: '0.85rem' }}>
          <KeyRound size={15} /> Resetear contraseña
        </button>
        {!user.is_manager && !user.is_superadmin && (
          <button onClick={promote} disabled={busy}
            style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', padding: '0.55rem 1rem', borderRadius: '10px', border: '1px solid var(--border, #cbd5e1)', background: 'var(--bg-primary, #fff)', fontWeight: 600, cursor: 'pointer', fontSize: '0.85rem' }}>
            <Shield size={15} /> Promover a manager
          </button>
        )}
      </div>
      )}
      {actionMsg && (
        <div style={{ margin: '0 0 1rem', padding: '0.6rem 0.9rem', borderRadius: '8px', background: 'rgba(16,185,129,0.1)', color: '#059669', fontSize: '0.85rem', fontWeight: 600 }}>{actionMsg}</div>
      )}

      <div className={styles.card}>
        <h3>Información</h3>
        <div className={styles.grid}>
          {fields.map(f => (
            <div key={f.label} className={styles.row}>
              <span className={styles.label}>{f.label}</span>
              <span className={styles.value}>{f.value}</span>
            </div>
          ))}
        </div>
      </div>

      {rbacApplicable && (
        <div className={styles.card} style={{ marginTop: '1rem' }}>
          <h3 style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <UserCog size={18} /> Roles y grupos
          </h3>
          <p style={{ margin: '4px 0 14px', fontSize: '0.83rem', color: '#94a3b8' }}>
            Haz clic para asignar o quitar. Los roles definen permisos por módulo; sin roles el usuario conserva el acceso normal de su tipo de cuenta.
          </p>

          <div style={{ marginBottom: '14px' }}>
            <span style={{ display: 'block', fontSize: '0.72rem', fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.04em', color: '#64748b', marginBottom: '8px' }}>Roles</span>
            {tenantRoles.length === 0 ? (
              <span style={{ fontSize: '0.85rem', color: '#94a3b8' }}>
                Esta empresa no tiene roles definidos — créalos en <a href="/roles-grupos">Roles y Grupos</a>.
              </span>
            ) : (
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
                {tenantRoles.map(role => {
                  const has = userRoleIds.has(role.id)
                  return (
                    <button key={role.id} type="button" style={chipStyle(has)} onClick={() => toggleRole(role.id, has)} disabled={rbacBusy} title={role.description || role.name}>
                      {has ? '✓ ' : '+ '}{role.name}
                    </button>
                  )
                })}
              </div>
            )}
          </div>

          <div>
            <span style={{ display: 'block', fontSize: '0.72rem', fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.04em', color: '#64748b', marginBottom: '8px' }}>Grupos</span>
            {tenantGroups.length === 0 ? (
              <span style={{ fontSize: '0.85rem', color: '#94a3b8' }}>
                Esta empresa no tiene grupos — créalos en <a href="/roles-grupos">Roles y Grupos</a>.
              </span>
            ) : (
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
                {tenantGroups.map(group => {
                  const has = userGroupIds.has(group.id)
                  return (
                    <button key={group.id} type="button" style={chipStyle(has)} onClick={() => toggleGroup(group.id, has)} disabled={rbacBusy} title={group.description || group.name}>
                      {has ? '✓ ' : '+ '}{group.name}
                    </button>
                  )
                })}
              </div>
            )}
          </div>
        </div>
      )}

      {showEdit && (
        <UserModal
          title="Editar usuario"
          mode="edit"
          form={editForm}
          setForm={setEditForm}
          employers={editPeople.employers}
          managers={editPeople.managers}
          onClose={() => setShowEdit(false)}
          onSubmit={handleEditSubmit}
          error={editError}
        />
      )}
    </div>
  )
}
