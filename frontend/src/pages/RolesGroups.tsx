import { useCallback, useEffect, useMemo, useState } from 'react'
import { Shield, UsersRound, Plus, Pencil, Trash2, UserPlus, X, Building2 } from 'lucide-react'
import { useAuth } from '../context/AuthContext'
import { useNotification } from '../context/NotificationContext'
import { useConfirm } from '../components/ui/ConfirmProvider'
import { rbacService } from '../services/rbac.service'
import { authService, channelService } from '../services/api'
import { Modal, Button, Skeleton } from '../components/ui'
import { Select } from '../components/ui/Select'
import type { CompanyRole, CompanyGroup, User, PermissionLevel } from '../types'
import styles from './Tenants/Tenants.module.css'

// Módulos de la app sobre los que un rol define permisos. La aplicación de
// estos permisos en cada módulo se conecta gradualmente.
const MODULES: { key: string; label: string }[] = [
  { key: 'tasks', label: 'Tareas' },
  { key: 'hours', label: 'Horas' },
  { key: 'reports', label: 'Reportes' },
  { key: 'chat', label: 'Chat' },
  { key: 'tutorials', label: 'Tutoriales' },
]

const LEVEL_OPTIONS: { value: PermissionLevel; label: string }[] = [
  { value: 'none', label: 'Sin acceso' },
  { value: 'view', label: 'Ver' },
  { value: 'edit', label: 'Ver y editar' },
]

const EMPTY_PERMS = (): Record<string, PermissionLevel> =>
  Object.fromEntries(MODULES.map(m => [m.key, 'none'])) as Record<string, PermissionLevel>

function parsePermissions(raw: string): Record<string, PermissionLevel> {
  const perms = EMPTY_PERMS()
  try {
    const parsed = JSON.parse(raw || '{}')
    for (const m of MODULES) {
      if (parsed[m.key] === 'view' || parsed[m.key] === 'edit') perms[m.key] = parsed[m.key]
    }
  } catch { /* permisos corruptos: tratar como sin acceso */ }
  return perms
}

function permissionSummary(raw: string): string {
  const perms = parsePermissions(raw)
  const granted = MODULES.filter(m => perms[m.key] !== 'none')
  if (granted.length === 0) return 'Sin permisos'
  if (granted.length === MODULES.length) return 'Todos los módulos'
  return granted.map(m => m.label).join(', ')
}

interface MemberCtx {
  kind: 'role' | 'group'
  id: number
  name: string
}

export default function RolesGroups() {
  const { user } = useAuth()
  const { success, error } = useNotification()
  const confirm = useConfirm()
  const isSuperadmin = !!user?.is_superadmin

  // Superadmin: misma empresa preferida que el chat.
  const [selectedCompanyId, setSelectedCompanyIdState] = useState<number | null>(() => {
    const stored = localStorage.getItem('preferred_company_id')
    return stored ? Number(stored) : null
  })
  const setSelectedCompanyId = (id: number | null) => {
    if (id) localStorage.setItem('preferred_company_id', String(id))
    setSelectedCompanyIdState(id)
  }
  const [companies, setCompanies] = useState<{ id: number; name: string }[]>([])

  // El backend resuelve el tenant de las cuentas empresa; company_id solo viaja para superadmin.
  const companyScope = isSuperadmin ? selectedCompanyId : undefined
  const scopeReady = !isSuperadmin || !!selectedCompanyId

  const [tab, setTab] = useState<'roles' | 'grupos'>('roles')
  const [roles, setRoles] = useState<CompanyRole[]>([])
  const [groups, setGroups] = useState<CompanyGroup[]>([])
  const [tenantUsers, setTenantUsers] = useState<User[]>([])
  const [isLoading, setIsLoading] = useState(false)

  // Modal de rol (crear/editar)
  const [showRoleModal, setShowRoleModal] = useState(false)
  const [editingRole, setEditingRole] = useState<CompanyRole | null>(null)
  const [roleName, setRoleName] = useState('')
  const [roleDescription, setRoleDescription] = useState('')
  const [rolePerms, setRolePerms] = useState<Record<string, PermissionLevel>>(EMPTY_PERMS())

  // Modal de grupo (crear/editar)
  const [showGroupModal, setShowGroupModal] = useState(false)
  const [editingGroup, setEditingGroup] = useState<CompanyGroup | null>(null)
  const [groupName, setGroupName] = useState('')
  const [groupDescription, setGroupDescription] = useState('')

  // Modal de miembros/usuarios asignados (compartido entre roles y grupos)
  const [memberCtx, setMemberCtx] = useState<MemberCtx | null>(null)
  const [assigned, setAssigned] = useState<User[]>([])
  const [pickedIds, setPickedIds] = useState<Set<number>>(new Set())
  const [memberSearch, setMemberSearch] = useState('')
  const [memberBusy, setMemberBusy] = useState(false)

  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (isSuperadmin && companies.length === 0) {
      authService.getPublicCompanies().then(setCompanies).catch(() => {})
    }
  }, [isSuperadmin, companies.length])

  const loadAll = useCallback(async () => {
    if (!scopeReady) {
      setRoles([]); setGroups([]); setTenantUsers([])
      return
    }
    setIsLoading(true)
    try {
      const [rolesData, groupsData, usersData] = await Promise.all([
        rbacService.listRoles(companyScope),
        rbacService.listGroups(companyScope),
        channelService.getAllUsers(companyScope).catch(() => [] as User[]),
      ])
      setRoles(rolesData)
      setGroups(groupsData)
      setTenantUsers(usersData)
    } catch {
      error('No se pudieron cargar los roles y grupos')
    } finally {
      setIsLoading(false)
    }
  }, [scopeReady, companyScope, error])

  useEffect(() => { loadAll() }, [loadAll])

  // ── Roles ──────────────────────────────────────────────────────────────────

  const openCreateRole = () => {
    setEditingRole(null)
    setRoleName('')
    setRoleDescription('')
    setRolePerms(EMPTY_PERMS())
    setShowRoleModal(true)
  }

  const openEditRole = (role: CompanyRole) => {
    setEditingRole(role)
    setRoleName(role.name)
    setRoleDescription(role.description || '')
    setRolePerms(parsePermissions(role.permissions))
    setShowRoleModal(true)
  }

  const handleSaveRole = async () => {
    if (!roleName.trim()) { error('El nombre del rol es obligatorio'); return }
    setSaving(true)
    try {
      const payload = {
        name: roleName,
        description: roleDescription,
        permissions: JSON.stringify(rolePerms),
      }
      if (editingRole) {
        await rbacService.updateRole(editingRole.id, payload, companyScope)
        success('Rol actualizado')
      } else {
        await rbacService.createRole(payload, companyScope)
        success('Rol creado')
      }
      setShowRoleModal(false)
      await loadAll()
    } catch (err: any) {
      error(err?.response?.data?.error || 'No se pudo guardar el rol')
    } finally {
      setSaving(false)
    }
  }

  const handleDeleteRole = async (role: CompanyRole) => {
    const ok = await confirm({
      title: 'Eliminar rol',
      message: `¿Eliminar el rol "${role.name}"? Se quitará de todos los usuarios que lo tengan asignado.`,
      confirmLabel: 'Eliminar',
      variant: 'danger',
    })
    if (!ok) return
    try {
      await rbacService.deleteRole(role.id, companyScope)
      success('Rol eliminado')
      await loadAll()
    } catch (err: any) {
      error(err?.response?.data?.error || 'No se pudo eliminar el rol')
    }
  }

  // ── Grupos ─────────────────────────────────────────────────────────────────

  const openCreateGroup = () => {
    setEditingGroup(null)
    setGroupName('')
    setGroupDescription('')
    setShowGroupModal(true)
  }

  const openEditGroup = (group: CompanyGroup) => {
    setEditingGroup(group)
    setGroupName(group.name)
    setGroupDescription(group.description || '')
    setShowGroupModal(true)
  }

  const handleSaveGroup = async () => {
    if (!groupName.trim()) { error('El nombre del grupo es obligatorio'); return }
    setSaving(true)
    try {
      const payload = { name: groupName, description: groupDescription }
      if (editingGroup) {
        await rbacService.updateGroup(editingGroup.id, payload, companyScope)
        success('Grupo actualizado')
      } else {
        await rbacService.createGroup(payload, companyScope)
        success('Grupo creado')
      }
      setShowGroupModal(false)
      await loadAll()
    } catch (err: any) {
      error(err?.response?.data?.error || 'No se pudo guardar el grupo')
    } finally {
      setSaving(false)
    }
  }

  const handleDeleteGroup = async (group: CompanyGroup) => {
    const ok = await confirm({
      title: 'Eliminar grupo',
      message: `¿Eliminar el grupo "${group.name}"? Sus miembros no se eliminan, solo la agrupación.`,
      confirmLabel: 'Eliminar',
      variant: 'danger',
    })
    if (!ok) return
    try {
      await rbacService.deleteGroup(group.id, companyScope)
      success('Grupo eliminado')
      await loadAll()
    } catch (err: any) {
      error(err?.response?.data?.error || 'No se pudo eliminar el grupo')
    }
  }

  // ── Miembros / usuarios asignados ──────────────────────────────────────────

  const openMembers = async (ctx: MemberCtx) => {
    setMemberCtx(ctx)
    setPickedIds(new Set())
    setMemberSearch('')
    setAssigned([])
    try {
      const users = ctx.kind === 'role'
        ? await rbacService.getRoleUsers(ctx.id, companyScope)
        : await rbacService.getGroupMembers(ctx.id, companyScope)
      setAssigned(users)
    } catch {
      error('No se pudieron cargar los usuarios asignados')
    }
  }

  const availableUsers = useMemo(() => {
    const assignedIds = new Set(assigned.map(u => u.id))
    const q = memberSearch.trim().toLowerCase()
    return tenantUsers
      .filter(u => !assignedIds.has(u.id))
      .filter(u => !q || u.name?.toLowerCase().includes(q) || u.email?.toLowerCase().includes(q))
      .sort((a, b) => (a.name || '').localeCompare(b.name || '', 'es', { sensitivity: 'base' }))
  }, [tenantUsers, assigned, memberSearch])

  const togglePicked = (userId: number) => {
    setPickedIds(prev => {
      const next = new Set(prev)
      if (next.has(userId)) next.delete(userId)
      else next.add(userId)
      return next
    })
  }

  const allVisiblePicked = availableUsers.length > 0 && availableUsers.every(u => pickedIds.has(u.id))
  const toggleAllVisible = () => {
    setPickedIds(prev => {
      const next = new Set(prev)
      if (allVisiblePicked) availableUsers.forEach(u => next.delete(u.id))
      else availableUsers.forEach(u => next.add(u.id))
      return next
    })
  }

  const handleAddMembers = async () => {
    if (!memberCtx || pickedIds.size === 0) return
    setMemberBusy(true)
    try {
      const ids = Array.from(pickedIds)
      if (memberCtx.kind === 'role') {
        await rbacService.assignRole(memberCtx.id, ids, companyScope)
      } else {
        await rbacService.addGroupMembers(memberCtx.id, ids, companyScope)
      }
      success(ids.length === 1 ? 'Usuario agregado' : `${ids.length} usuarios agregados`)
      await openMembers(memberCtx)
      await loadAll()
    } catch (err: any) {
      error(err?.response?.data?.error || 'No se pudieron agregar los usuarios')
    } finally {
      setMemberBusy(false)
    }
  }

  const handleRemoveMember = async (userId: number) => {
    if (!memberCtx) return
    setMemberBusy(true)
    try {
      if (memberCtx.kind === 'role') {
        await rbacService.unassignRole(memberCtx.id, userId, companyScope)
      } else {
        await rbacService.removeGroupMember(memberCtx.id, userId, companyScope)
      }
      setAssigned(prev => prev.filter(u => u.id !== userId))
      await loadAll()
    } catch (err: any) {
      error(err?.response?.data?.error || 'No se pudo quitar el usuario')
    } finally {
      setMemberBusy(false)
    }
  }

  // ── Render ─────────────────────────────────────────────────────────────────

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1>Roles y Grupos</h1>
          <p>Define roles con permisos por módulo y organiza a tu equipo en grupos</p>
        </div>
        {scopeReady && (
          tab === 'roles' ? (
            <Button onClick={openCreateRole} leftIcon={<Plus size={18} />}>Nuevo rol</Button>
          ) : (
            <Button onClick={openCreateGroup} leftIcon={<Plus size={18} />}>Nuevo grupo</Button>
          )
        )}
      </div>

      {isSuperadmin && (
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: 20, flexWrap: 'wrap' }}>
          <div style={{ minWidth: 240 }}>
            <Select
              fullWidth
              placeholder="Selecciona una empresa..."
              value={selectedCompanyId ?? ''}
              onChange={v => setSelectedCompanyId(v ? Number(v) : null)}
              options={companies.map(c => ({ value: c.id, label: c.name }))}
            />
          </div>
          <span style={{ fontSize: '13px', color: '#64748b' }}>
            Los roles y grupos pertenecen a cada empresa.
          </span>
        </div>
      )}

      <div className={styles.subTabs}>
        <button className={tab === 'roles' ? styles.subTabActive : styles.subTab} onClick={() => setTab('roles')}>
          Roles ({roles.length})
        </button>
        <button className={tab === 'grupos' ? styles.subTabActive : styles.subTab} onClick={() => setTab('grupos')}>
          Grupos ({groups.length})
        </button>
      </div>

      {!scopeReady ? (
        <div className={styles.empty}>
          <Building2 size={40} />
          <p>Selecciona una empresa para gestionar sus roles y grupos</p>
        </div>
      ) : isLoading ? (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.6rem' }}>
          {Array.from({ length: 4 }).map((_, i) => <Skeleton key={i} height={56} radius={12} />)}
        </div>
      ) : tab === 'roles' ? (
        roles.length === 0 ? (
          <div className={styles.empty}>
            <Shield size={40} />
            <p>Aún no hay roles. Crea el primero para definir permisos por módulo.</p>
          </div>
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>Rol</th>
                  <th>Permisos</th>
                  <th>Usuarios</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {roles.map(role => (
                  <tr key={role.id} className={styles.row}>
                    <td>
                      <div className={styles.ownerCell}>
                        <span>{role.name}</span>
                        {role.description && <small>{role.description}</small>}
                      </div>
                    </td>
                    <td>{permissionSummary(role.permissions)}</td>
                    <td>{role.user_count}</td>
                    <td>
                      <div className={styles.rowActions}>
                        <button className={styles.iconBtn} onClick={() => openMembers({ kind: 'role', id: role.id, name: role.name })} title="Usuarios con este rol">
                          <UserPlus size={16} />
                        </button>
                        <button className={styles.iconBtn} onClick={() => openEditRole(role)} title="Editar rol">
                          <Pencil size={16} />
                        </button>
                        <button className={`${styles.iconBtn} ${styles.danger}`} onClick={() => handleDeleteRole(role)} title="Eliminar rol">
                          <Trash2 size={16} />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )
      ) : (
        groups.length === 0 ? (
          <div className={styles.empty}>
            <UsersRound size={40} />
            <p>Aún no hay grupos. Crea el primero para organizar a tu equipo.</p>
          </div>
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>Grupo</th>
                  <th>Miembros</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {groups.map(group => (
                  <tr key={group.id} className={styles.row}>
                    <td>
                      <div className={styles.ownerCell}>
                        <span>{group.name}</span>
                        {group.description && <small>{group.description}</small>}
                      </div>
                    </td>
                    <td>{group.member_count}</td>
                    <td>
                      <div className={styles.rowActions}>
                        <button className={styles.iconBtn} onClick={() => openMembers({ kind: 'group', id: group.id, name: group.name })} title="Miembros del grupo">
                          <UserPlus size={16} />
                        </button>
                        <button className={styles.iconBtn} onClick={() => openEditGroup(group)} title="Editar grupo">
                          <Pencil size={16} />
                        </button>
                        <button className={`${styles.iconBtn} ${styles.danger}`} onClick={() => handleDeleteGroup(group)} title="Eliminar grupo">
                          <Trash2 size={16} />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )
      )}

      {/* Modal de rol */}
      <Modal
        isOpen={showRoleModal}
        onClose={() => setShowRoleModal(false)}
        title={editingRole ? 'Editar rol' : 'Nuevo rol'}
        size="md"
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowRoleModal(false)} disabled={saving}>Cancelar</Button>
            <Button onClick={handleSaveRole} loading={saving}>{editingRole ? 'Guardar cambios' : 'Crear rol'}</Button>
          </>
        }
      >
        <div className={styles.field}>
          <label>Nombre del rol</label>
          <input value={roleName} onChange={e => setRoleName(e.target.value)} placeholder="Ej: Supervisor RRHH" autoFocus />
        </div>
        <div className={styles.field}>
          <label>Descripción</label>
          <input value={roleDescription} onChange={e => setRoleDescription(e.target.value)} placeholder="Qué hace este rol (opcional)" />
        </div>
        <div className={styles.field}>
          <label>Permisos por módulo</label>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', marginTop: '4px' }}>
            {MODULES.map(m => (
              <div key={m.key} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px' }}>
                <span style={{ fontSize: '14px', color: '#0f172a' }}>{m.label}</span>
                <div style={{ minWidth: 170 }}>
                  <Select
                    fullWidth
                    value={rolePerms[m.key]}
                    onChange={v => setRolePerms(prev => ({ ...prev, [m.key]: (v as PermissionLevel) || 'none' }))}
                    options={LEVEL_OPTIONS}
                  />
                </div>
              </div>
            ))}
          </div>
          <p className={styles.modalHint} style={{ marginTop: '10px' }}>
            Los permisos definen qué podrá ver o editar cada usuario con este rol. Su aplicación en cada módulo se irá conectando progresivamente.
          </p>
        </div>
      </Modal>

      {/* Modal de grupo */}
      <Modal
        isOpen={showGroupModal}
        onClose={() => setShowGroupModal(false)}
        title={editingGroup ? 'Editar grupo' : 'Nuevo grupo'}
        size="sm"
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowGroupModal(false)} disabled={saving}>Cancelar</Button>
            <Button onClick={handleSaveGroup} loading={saving}>{editingGroup ? 'Guardar cambios' : 'Crear grupo'}</Button>
          </>
        }
      >
        <div className={styles.field}>
          <label>Nombre del grupo</label>
          <input value={groupName} onChange={e => setGroupName(e.target.value)} placeholder="Ej: Equipo Backend" autoFocus />
        </div>
        <div className={styles.field}>
          <label>Descripción</label>
          <input value={groupDescription} onChange={e => setGroupDescription(e.target.value)} placeholder="Para qué es este grupo (opcional)" />
        </div>
      </Modal>

      {/* Modal de miembros / usuarios asignados */}
      <Modal
        isOpen={!!memberCtx}
        onClose={() => setMemberCtx(null)}
        title={memberCtx ? (memberCtx.kind === 'role' ? `Usuarios con el rol — ${memberCtx.name}` : `Miembros — ${memberCtx.name}`) : ''}
        size="md"
        footer={<Button variant="secondary" onClick={() => setMemberCtx(null)}>Cerrar</Button>}
      >
        <div style={{ marginBottom: '16px' }}>
          <div style={{ display: 'flex', gap: '8px', marginBottom: '10px' }}>
            <input
              type="search"
              value={memberSearch}
              onChange={e => setMemberSearch(e.target.value)}
              placeholder="Buscar usuario por nombre o correo..."
              style={{ flex: 1, padding: '9px 12px', border: '1px solid var(--glass-border, #e2e8f0)', borderRadius: '10px', fontSize: '14px', outline: 'none' }}
            />
            <Button onClick={handleAddMembers} disabled={pickedIds.size === 0} loading={memberBusy} leftIcon={<UserPlus size={16} />}>
              Agregar {pickedIds.size > 0 ? `(${pickedIds.size})` : ''}
            </Button>
          </div>

          {availableUsers.length === 0 ? (
            <p className={styles.modalHint}>
              {memberSearch.trim() ? 'Sin usuarios disponibles que coincidan.' : 'Todos los usuarios de la empresa ya están agregados.'}
            </p>
          ) : (
            <div style={{ border: '1px solid var(--glass-border, #e2e8f0)', borderRadius: '10px', maxHeight: 220, overflowY: 'auto' }}>
              <label style={{ display: 'flex', alignItems: 'center', gap: '10px', padding: '8px 12px', borderBottom: '1px solid var(--glass-border, #e2e8f0)', cursor: 'pointer', fontSize: '13px', fontWeight: 600, color: '#475569', position: 'sticky', top: 0, background: '#f8fafc' }}>
                <input type="checkbox" checked={allVisiblePicked} onChange={toggleAllVisible} />
                Seleccionar todos ({availableUsers.length})
              </label>
              {availableUsers.map(u => (
                <label key={u.id} style={{ display: 'flex', alignItems: 'center', gap: '10px', padding: '8px 12px', cursor: 'pointer' }}>
                  <input type="checkbox" checked={pickedIds.has(u.id)} onChange={() => togglePicked(u.id)} />
                  <div className={styles.ownerCell}>
                    <span>{u.name}</span>
                    <small>{u.email}</small>
                  </div>
                </label>
              ))}
            </div>
          )}
        </div>

        {assigned.length === 0 ? (
          <p className={styles.modalHint}>
            {memberCtx?.kind === 'role' ? 'Nadie tiene este rol todavía.' : 'Este grupo no tiene miembros todavía.'}
          </p>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
            {assigned.map(u => (
              <div key={u.id} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '10px', padding: '8px 12px', border: '1px solid var(--glass-border, #e2e8f0)', borderRadius: '10px' }}>
                <div className={styles.ownerCell}>
                  <span>{u.name}</span>
                  <small>{u.email}</small>
                </div>
                <button
                  className={`${styles.iconBtn} ${styles.danger}`}
                  onClick={() => handleRemoveMember(u.id)}
                  disabled={memberBusy}
                  title="Quitar"
                >
                  <X size={15} />
                </button>
              </div>
            ))}
          </div>
        )}
      </Modal>
    </div>
  )
}
