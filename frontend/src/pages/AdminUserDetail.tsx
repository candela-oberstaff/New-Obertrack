import { useState, useEffect, useCallback } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, UserX, Power, KeyRound, Shield, UserCog, Pencil, Building2, Plus, LogOut, FileText, RotateCcw, Users, Eye } from 'lucide-react'
import { userService, adminService, authService } from '../services/api'
import { rbacService } from '../services/rbac.service'
import { useAuth } from '../context/AuthContext'
import { Modal, Button } from '../components/ui'
import { useConfirm } from '../components/ui/ConfirmProvider'
import { Select } from '../components/ui/Select'
import type { User, CompanyRole, CompanyGroup } from '../types'
import Avatar from '../components/Common/Avatar'
import { Skeleton } from '../components/ui'
import { UserModal } from '../components/Admin/Modals/UserModal'
import { ExpedienteModal } from '../components/Admin/ExpedienteModal'
import EmploymentManagersEditor from '../components/Admin/EmploymentManagersEditor'
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
  const confirm = useConfirm()
  // CS entra en modo consulta: sin acciones ni gestión de roles/grupos.
  const canManage = !!viewer?.is_superadmin
  const [user, setUser] = useState<User | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [actionMsg, setActionMsg] = useState<string | null>(null)
  const [actionErr, setActionErr] = useState(false)
  const [empresaName, setEmpresaName] = useState('')
  const [managerName, setManagerName] = useState('')
  // Flag de features: si está activo, se gestiona el CONJUNTO de managers por empleo
  // (multi-manager) en lugar del <select> de manager principal único.
  const [multiManager, setMultiManager] = useState(false)
  // Equipo a cargo (solo cuando el usuario es manager).
  const [managedTeam, setManagedTeam] = useState<any[]>([])
  // Selector inline de manager (asignar/quitar desde el detalle sin recargar).
  const [managerOptions, setManagerOptions] = useState<{ id: number; name: string }[]>([])
  const [managerBusy, setManagerBusy] = useState(false)
  // Reasignar equipo: mueve todos los reportes del manager a otro (o los desasigna).
  const [showReassign, setShowReassign] = useState(false)
  // Listado de profesionales a cargo (modal que bloquea quitar el rol).
  const [teamBlock, setTeamBlock] = useState<any[] | null>(null)
  const [teamCheckBusy, setTeamCheckBusy] = useState(false)
  const [reassignTo, setReassignTo] = useState<number | ''>('')
  const [reassignBusy, setReassignBusy] = useState(false)

  // Membresías (empleos) del usuario: multi-empresa + expediente.
  const [employments, setEmployments] = useState<any[]>([])
  const [showAddEmp, setShowAddEmp] = useState(false)
  const [companies, setCompanies] = useState<{ id: number; name: string }[]>([])
  const [addCompanyId, setAddCompanyId] = useState<number | ''>('')
  const [addJobTitle, setAddJobTitle] = useState('')
  const [addStartReason, setAddStartReason] = useState('')
  const [empBusy, setEmpBusy] = useState(false)
  const [empError, setEmpError] = useState<string | null>(null)
  const [endingEmp, setEndingEmp] = useState<any | null>(null)
  const [endReason, setEndReason] = useState('')
  // Empleo cuyo expediente (resumen + notas + documentos) se está viendo.
  const [expedienteEmp, setExpedienteEmp] = useState<any | null>(null)

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

  const loadEmployments = useCallback(async (u: User) => {
    if (u.user_type !== 'profesional' && u.user_type !== 'customer_success') {
      setEmployments([])
      return
    }
    try {
      setEmployments(await adminService.getUserEmployments(u.id))
    } catch { /* sección opcional */ }
  }, [])

  const load = useCallback(async () => {
    setIsLoading(true)
    try {
      const data = await userService.getById(Number(id))
      setUser(data)
      setError(null)
      loadRBAC(data)
      loadEmployments(data)
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
  }, [id, loadRBAC, loadEmployments])

  useEffect(() => { load() }, [load])

  // Carga el flag de features (multi-manager) una vez al montar.
  useEffect(() => {
    let cancelled = false
    adminService.getFeatures()
      .then(f => { if (!cancelled) setMultiManager(!!f?.multi_manager_reads) })
      .catch(() => { /* sin flag: se mantiene el modo single */ })
    return () => { cancelled = true }
  }, [])

  // Managers candidatos del mismo tenant para el selector inline (solo superadmin).
  // Se cargan para profesionales (selector de manager), customer_success (manager
  // por-empleo) y para cualquier manager (modal "Reasignar equipo").
  useEffect(() => {
    if (!user || !canManage) return
    if (user.user_type !== 'profesional' && user.user_type !== 'customer_success' && !user.is_manager) return
    let cancelled = false
    adminService.getUsers({ limit: 1000 })
      .then((res: any) => {
        if (cancelled) return
        const all: User[] = res?.data || (Array.isArray(res) ? res : [])
        setManagerOptions(
          all
            .filter(u => u.is_manager && u.id !== user.id && tenantIdForUser(u) === tenantIdForUser(user))
            .map(u => ({ id: u.id, name: u.name })),
        )
      })
      .catch(() => { /* selector vacío si falla */ })
    return () => { cancelled = true }
  }, [user?.id, canManage])

  // Profesionales a cargo: solo si el usuario es manager. Usa /reports, que con el
  // flag multi-manager ON resuelve el equipo por la tabla employment_managers.
  useEffect(() => {
    if (!user || !user.is_manager) { setManagedTeam([]); return }
    let cancelled = false
    adminService.getManagerReports(user.id)
      .then((rows: any) => { if (!cancelled) setManagedTeam(Array.isArray(rows) ? rows : []) })
      .catch(() => { if (!cancelled) setManagedTeam([]) })
    return () => { cancelled = true }
  }, [user?.id, user?.is_manager])

  const toggleActive = async () => {
    if (!user) return
    setBusy(true); setActionMsg(null)
    try {
      await adminService.updateUser(user.id, { is_active: !user.is_active })
      await load()
      invalidateAdmin()
    } catch { setActionMsg('No se pudo cambiar el estado.') } finally { setBusy(false) }
  }

  // Promueve (true) o quita el rol de manager (false). Set explícito, no toggle ciego.
  // Al degradar, pide confirmación; el backend además bloquea si aún tiene equipo.
  // Antes de quitar el rol: trae el equipo. Si tiene gente, muestra el listado
  // en un modal (en vez de un número) para que se reasigne primero.
  const handleRemoveManagerClick = async () => {
    if (!user) return
    setTeamCheckBusy(true)
    let reports: any[] = []
    try {
      reports = await adminService.getManagerReports(user.id)
    } catch { /* el backend igual valida; seguimos al flujo normal */ }
    setTeamCheckBusy(false)
    if (Array.isArray(reports) && reports.length > 0) {
      setTeamBlock(reports)
      return
    }
    setManagerRole(false)
  }

  const setManagerRole = async (value: boolean) => {
    if (!user) return
    if (!value) {
      const ok = await confirm({
        title: 'Quitar rol de manager',
        message: `¿Quitar el rol de Manager a ${user.name}? Si tiene profesionales a su cargo deberás reasignar su equipo primero.`,
        confirmLabel: 'Quitar rol',
        variant: 'danger',
      })
      if (!ok) return
    }
    setBusy(true); setActionMsg(null); setActionErr(false)
    try {
      await adminService.updateUser(user.id, { is_manager: value })
      await load()
      invalidateAdmin()
      setActionMsg(value ? 'Usuario promovido a manager.' : 'Rol de manager removido.')
    } catch (err: any) {
      setActionErr(true)
      const base = err?.response?.data?.error ?? (value ? 'No se pudo promover.' : 'No se pudo quitar el rol.')
      // Si el backend bloquea por equipo aún asignado (409), recuerda "Reasignar equipo".
      setActionMsg(err?.response?.status === 409 ? `${base} Usa "Reasignar equipo" primero.` : base)
    } finally { setBusy(false) }
  }

  // Asigna o quita el manager desde el detalle y refleja el cambio en vivo.
  const handleAssignManager = async (value: string) => {
    if (!user) return
    const managerId = value === '' ? null : Number(value)
    setManagerBusy(true); setActionMsg(null); setActionErr(false)
    try {
      await adminService.updateUser(user.id, { manager_id: managerId })
      setUser(prev => (prev ? { ...prev, manager_id: managerId ?? undefined } : prev))
      setManagerName(managerId ? (managerOptions.find(m => m.id === managerId)?.name || '') : '')
      invalidateAdmin()
      setActionMsg(managerId ? 'Manager asignado.' : 'Manager removido.')
    } catch (err: any) {
      setActionErr(true)
      setActionMsg(err?.response?.data?.error ?? 'No se pudo actualizar el manager.')
    } finally { setManagerBusy(false) }
  }

  // Reasigna todo el equipo del manager a otro manager (o lo desasigna si null).
  const submitReassignTeam = async () => {
    if (!user) return
    const newManagerId = reassignTo === '' ? null : Number(reassignTo)
    setReassignBusy(true); setActionMsg(null); setActionErr(false)
    try {
      const data = await userService.reassignTeam(user.id, newManagerId)
      setActionMsg(typeof data?.reassigned === 'number' ? `Equipo reasignado (${data.reassigned}).` : 'Equipo reasignado')
      invalidateAdmin()
      setShowReassign(false)
      setActionErr(false)
      await load() // refresca el detalle (estado del usuario y empleos) sin recargar la página
    } catch (err: any) {
      setActionErr(true)
      setActionMsg(err?.response?.data?.error ?? 'No se pudo reasignar el equipo.')
    } finally { setReassignBusy(false) }
  }

  // Cambia el manager de un empleo concreto (cada empresa puede tener uno distinto).
  const handleEmploymentManager = async (emp: any, value: string) => {
    if (!user) return
    const managerId = value === '' ? null : Number(value)
    setManagerBusy(true); setActionMsg(null); setActionErr(false)
    try {
      await adminService.updateEmploymentManager(user.id, emp.id, managerId)
      await loadEmployments(user)
      setActionMsg('Manager del empleo actualizado')
    } catch (err: any) {
      setActionErr(true)
      setActionMsg(err?.response?.data?.error ?? 'No se pudo actualizar el manager del empleo.')
    } finally { setManagerBusy(false) }
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

  const openAddEmp = async () => {
    setAddCompanyId(''); setAddJobTitle(''); setAddStartReason(''); setEmpError(null)
    setShowAddEmp(true)
    if (companies.length === 0) {
      try { setCompanies(await authService.getPublicCompanies()) } catch { /* picker vacío si falla */ }
    }
  }

  const handleAddEmp = async () => {
    if (!user || !addCompanyId) return
    setEmpBusy(true); setEmpError(null)
    try {
      await adminService.addUserEmployment(user.id, {
        company_id: Number(addCompanyId),
        job_title: addJobTitle || undefined,
        start_reason: addStartReason || undefined,
      })
      setShowAddEmp(false)
      await loadEmployments(user)
      invalidateAdmin()
    } catch (err: any) {
      setEmpError(err?.response?.data?.error ?? 'No se pudo agregar la empresa')
    } finally { setEmpBusy(false) }
  }

  const submitEndEmp = async () => {
    if (!user || !endingEmp) return
    setEmpBusy(true); setEmpError(null)
    try {
      await adminService.endUserEmployment(user.id, endingEmp.id, endReason)
      setEndingEmp(null)
      await loadEmployments(user)
      await load()
      invalidateAdmin()
    } catch (err: any) {
      setEmpError(err?.response?.data?.error ?? 'No se pudo finalizar el empleo.')
    } finally { setEmpBusy(false) }
  }

  const reactivateEmp = async (emp: any) => {
    if (!user) return
    setEmpBusy(true); setEmpError(null)
    try {
      await adminService.reactivateEmployment(user.id, emp.id)
      await loadEmployments(user)
      await load()
      invalidateAdmin()
    } catch (err: any) {
      setEmpError(err?.response?.data?.error ?? 'No se pudo reactivar el empleo.')
    } finally { setEmpBusy(false) }
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

  // Etiqueta del tipo de cuenta. El rol de Manager se muestra en un badge aparte
  // (más abajo), así que aquí NO se colapsa a 'Manager' para no duplicarlo.
  const rol = user.is_superadmin
    ? 'Superadmin'
    : user.user_type === 'empleador'
      ? 'Empresa'
      : user.user_type === 'customer_success'
        ? 'Customer Success'
        : user.user_type === 'profesional'
          ? 'Profesional'
          : user.user_type

  // Asegura que el manager actual aparezca en el selector aunque la lista aún no cargue.
  const managerSelectOptions =
    user.manager_id && !managerOptions.some(m => m.id === user.manager_id)
      ? [{ id: user.manager_id, name: managerName || `#${user.manager_id}` }, ...managerOptions]
      : managerOptions

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
  // Empleo activo del profesional (empresa actual): el campo MANAGER de arriba
  // gestiona el conjunto de managers de ESE empleo cuando el multi-manager está ON.
  const activeEmployment = employments.find((e: any) => e.company_id === user.empleador_id && e.status === 'active')

  let specific: { label: string; value: React.ReactNode }[] = []
  if (user.user_type === 'empleador') {
    specific = [{ label: 'Empresa', value: user.company_name || '—' }]
  } else if (user.user_type === 'profesional') {
    specific = [
      { label: 'Cargo', value: user.job_title || '—' },
      { label: 'Empresa', value: empresaName || '—' },
      {
        label: multiManager && activeEmployment ? 'Managers' : 'Manager',
        value: canManage ? (
          multiManager && activeEmployment ? (
            <EmploymentManagersEditor
              userId={user.id}
              employmentId={activeEmployment.id}
              companyId={activeEmployment.company_id}
              managerOptions={managerOptions}
              onChanged={() => { load(); loadEmployments(user) }}
            />
          ) : (
            <select
              value={user.manager_id ?? ''}
              onChange={e => handleAssignManager(e.target.value)}
              disabled={managerBusy}
              title="Asignar o quitar manager"
              style={{ maxWidth: '100%', fontSize: '0.9rem', padding: '4px 8px', borderRadius: '8px', border: '1px solid var(--border, #cbd5e1)', background: '#fff', color: '#334155', cursor: managerBusy ? 'progress' : 'pointer' }}
            >
              <option value="">Sin asignar</option>
              {managerSelectOptions.map(m => (
                <option key={m.id} value={m.id}>{m.name}</option>
              ))}
            </select>
          )
        ) : (managerName || 'Sin asignar'),
      },
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
        {!user.is_superadmin && (
          <button onClick={() => (user.is_manager ? handleRemoveManagerClick() : setManagerRole(true))} disabled={busy || teamCheckBusy}
            style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', padding: '0.55rem 1rem', borderRadius: '10px', border: '1px solid var(--border, #cbd5e1)', background: 'var(--bg-primary, #fff)', fontWeight: 600, color: user.is_manager ? '#b91c1c' : undefined, cursor: 'pointer', fontSize: '0.85rem' }}>
            <Shield size={15} /> {user.is_manager ? 'Quitar rol de manager' : 'Promover a manager'}
          </button>
        )}
        {user.is_manager && (
          <button onClick={() => { setReassignTo(''); setShowReassign(true) }} disabled={busy}
            style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', padding: '0.55rem 1rem', borderRadius: '10px', border: '1px solid var(--border, #cbd5e1)', background: 'var(--bg-primary, #fff)', fontWeight: 600, cursor: 'pointer', fontSize: '0.85rem' }}>
            <Users size={15} /> Reasignar equipo
          </button>
        )}
      </div>
      )}
      {actionMsg && (
        <div style={{ margin: '0 0 1rem', padding: '0.6rem 0.9rem', borderRadius: '8px', background: actionErr ? 'rgba(220,38,38,0.1)' : 'rgba(16,185,129,0.1)', color: actionErr ? '#dc2626' : '#059669', fontSize: '0.85rem', fontWeight: 600 }}>{actionMsg}</div>
      )}

      <div className={styles.card}>
        <h3>Información</h3>
        <div className={styles.grid}>
          {fields.map(f => (
            <div
              key={f.label}
              className={styles.row}
              // El editor multi-manager (chips) ocupa toda la fila para que los
              // chips fluyan en horizontal y no quede espacio muerto a la derecha.
              style={f.label === 'Managers' ? { gridColumn: '1 / -1' } : undefined}
            >
              <span className={styles.label}>{f.label}</span>
              <span className={styles.value}>{f.value}</span>
            </div>
          ))}
        </div>
      </div>

      {user.is_manager && (
        <div className={styles.card} style={{ marginTop: '1rem' }}>
          <h3 style={{ display: 'flex', alignItems: 'center', gap: '8px', margin: 0 }}>
            <Users size={18} /> Profesionales a cargo
            <span style={{ fontSize: '0.8rem', fontWeight: 600, color: '#94a3b8' }}>({managedTeam.length})</span>
          </h3>
          <p style={{ margin: '6px 0 14px', fontSize: '0.83rem', color: '#94a3b8' }}>
            Profesionales que tienen a {user.name} como manager{multiManager ? ' (cualquiera de sus managers puede aprobar sus horas)' : ''}.
          </p>
          {managedTeam.length === 0 ? (
            <span style={{ fontSize: '0.85rem', color: '#94a3b8' }}>Sin profesionales a cargo.</span>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
              {managedTeam.map((m: any) => (
                <div key={m.id} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', padding: '10px 14px', border: '1px solid var(--border, #e2e8f0)', borderRadius: '10px' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '10px', minWidth: 0 }}>
                    <Avatar src={m.avatar} name={m.name} size="sm" />
                    <div style={{ minWidth: 0 }}>
                      <div style={{ fontWeight: 600, color: '#0f172a' }}>{m.name}</div>
                      <div style={{ fontSize: '0.8rem', color: '#94a3b8', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                        {(m.job_title || 'Profesional')} · {m.email}
                      </div>
                    </div>
                  </div>
                  <button
                    onClick={() => navigate(`/admin/users/${m.id}`)}
                    title="Ver detalle"
                    style={{ display: 'inline-flex', alignItems: 'center', gap: '4px', padding: '5px 10px', borderRadius: '8px', border: '1px solid var(--border, #cbd5e1)', background: '#fff', color: '#334155', fontWeight: 600, cursor: 'pointer', fontSize: '0.8rem', flexShrink: 0 }}
                  >
                    <Eye size={14} /> Ver
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {(user.user_type === 'profesional' || user.user_type === 'customer_success') && (
        <div className={styles.card} style={{ marginTop: '1rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap' }}>
            <h3 style={{ display: 'flex', alignItems: 'center', gap: '8px', margin: 0 }}>
              <Building2 size={18} /> Empresas / Empleos
            </h3>
            {canManage && (
              <Button onClick={openAddEmp} leftIcon={<Plus size={16} />} variant="secondary">Agregar empresa</Button>
            )}
          </div>
          <p style={{ margin: '6px 0 14px', fontSize: '0.83rem', color: '#94a3b8' }}>
            Empresas donde trabaja o trabajó. La activa (en negrita) es donde opera ahora; el resto forma su expediente multi-empresa.
          </p>

          {employments.length === 0 ? (
            <span style={{ fontSize: '0.85rem', color: '#94a3b8' }}>Sin membresías registradas.</span>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
              {employments.map(emp => {
                const isActiveCompany = user.empleador_id === emp.company_id && emp.status === 'active'
                const ended = emp.status === 'ended'
                return (
                  <div key={emp.id} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', padding: '10px 14px', border: '1px solid var(--border, #e2e8f0)', borderRadius: '10px', opacity: ended ? 0.65 : 1 }}>
                    <div>
                      <span style={{ fontWeight: isActiveCompany ? 800 : 600, color: '#0f172a' }}>
                        {emp.company_name}{isActiveCompany && ' · activa'}
                      </span>
                      <div style={{ fontSize: '0.8rem', color: '#94a3b8' }}>
                        {emp.job_title || 'Sin cargo'} · desde {new Date(emp.started_at).toLocaleDateString('es-ES')}
                        {ended && emp.ended_at && ` · hasta ${new Date(emp.ended_at).toLocaleDateString('es-ES')}`}
                        {ended && emp.end_reason ? ` · ${emp.end_reason}` : ''}
                      </div>
                    </div>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                      {canManage && !ended && (
                        multiManager ? (
                          // Modo multi-manager: gestiona el CONJUNTO de managers del empleo.
                          <EmploymentManagersEditor
                            userId={user.id}
                            employmentId={emp.id}
                            companyId={emp.company_id}
                            managerOptions={managerOptions}
                            onChanged={() => loadEmployments(user)}
                          />
                        ) : (() => {
                          // Modo single: <select> del manager principal único.
                          // Si el manager del empleo no está en la lista, lo agregamos como
                          // fallback (con su id) para no parpadear mientras carga.
                          const empMgrId: number | null = emp.manager_id ?? null
                          const empMgrOptions =
                            empMgrId && !managerOptions.some(m => m.id === empMgrId)
                              ? [{ id: empMgrId, name: `#${empMgrId}` }, ...managerOptions]
                              : managerOptions
                          return (
                            <select
                              value={empMgrId ?? ''}
                              onChange={e => handleEmploymentManager(emp, e.target.value)}
                              disabled={managerBusy}
                              title="Manager de este empleo"
                              style={{ maxWidth: '160px', fontSize: '0.78rem', padding: '4px 8px', borderRadius: '8px', border: '1px solid var(--border, #cbd5e1)', background: '#fff', color: '#334155', cursor: managerBusy ? 'progress' : 'pointer' }}
                            >
                              <option value="">Sin manager</option>
                              {empMgrOptions.map(m => (
                                <option key={m.id} value={m.id}>{m.name}</option>
                              ))}
                            </select>
                          )
                        })()
                      )}
                      <span style={{ padding: '3px 10px', borderRadius: 999, fontSize: '0.72rem', fontWeight: 700, background: ended ? 'rgba(100,116,139,0.12)' : 'rgba(16,185,129,0.12)', color: ended ? '#64748b' : '#047857' }}>
                        {ended ? 'Finalizado' : 'Activo'}
                      </span>
                      <button onClick={() => setExpedienteEmp(emp)} title="Ver expediente"
                        style={{ display: 'inline-flex', alignItems: 'center', gap: '4px', padding: '5px 10px', borderRadius: '8px', border: '1px solid var(--border, #cbd5e1)', background: '#fff', color: '#334155', fontWeight: 600, cursor: 'pointer', fontSize: '0.8rem' }}>
                        <FileText size={14} /> Expediente
                      </button>
                      {canManage && !ended && (
                        <button onClick={() => { setEndingEmp(emp); setEndReason(''); setEmpError(null) }} title="Finalizar empleo"
                          style={{ display: 'inline-flex', alignItems: 'center', gap: '4px', padding: '5px 10px', borderRadius: '8px', border: '1px solid var(--border, #cbd5e1)', background: '#fff', color: '#b91c1c', fontWeight: 600, cursor: 'pointer', fontSize: '0.8rem' }}>
                          <LogOut size={14} /> Finalizar
                        </button>
                      )}
                      {canManage && ended && (
                        <button onClick={() => reactivateEmp(emp)} disabled={empBusy} title="Reactivar empleo"
                          style={{ display: 'inline-flex', alignItems: 'center', gap: '4px', padding: '5px 10px', borderRadius: '8px', border: '1px solid var(--border, #cbd5e1)', background: '#fff', color: '#047857', fontWeight: 600, cursor: empBusy ? 'progress' : 'pointer', fontSize: '0.8rem' }}>
                          <RotateCcw size={14} /> Reactivar
                        </button>
                      )}
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      )}

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

      <Modal
        isOpen={showAddEmp}
        onClose={() => setShowAddEmp(false)}
        title="Agregar empresa"
        size="sm"
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowAddEmp(false)} disabled={empBusy}>Cancelar</Button>
            <Button onClick={handleAddEmp} loading={empBusy} disabled={!addCompanyId}>Agregar</Button>
          </>
        }
      >
        <p className={styles.modalHint || ''} style={{ fontSize: '0.85rem', color: '#64748b', marginTop: 0 }}>
          Vincula a {user.name} con otra empresa. Quedará como empleo activo en su expediente.
        </p>
        <div style={{ marginBottom: '14px' }}>
          <label style={{ display: 'block', fontSize: '0.8rem', fontWeight: 600, color: '#334155', marginBottom: '6px' }}>Empresa</label>
          <Select
            fullWidth
            value={addCompanyId}
            onChange={v => setAddCompanyId(v ? Number(v) : '')}
            placeholder="Selecciona una empresa..."
            options={companies
              .filter(c => !employments.some(e => e.company_id === c.id && e.status === 'active'))
              .map(c => ({ value: c.id, label: c.name }))}
          />
        </div>
        <div style={{ marginBottom: '14px' }}>
          <label style={{ display: 'block', fontSize: '0.8rem', fontWeight: 600, color: '#334155', marginBottom: '6px' }}>Cargo en esa empresa</label>
          <input type="text" value={addJobTitle} onChange={e => setAddJobTitle(e.target.value)} placeholder="Ej: Desarrollador Backend"
            style={{ width: '100%', padding: '10px 12px', border: '1px solid #cbd5e1', borderRadius: '10px', fontSize: '14px' }} />
        </div>
        <div>
          <label style={{ display: 'block', fontSize: '0.8rem', fontWeight: 600, color: '#334155', marginBottom: '6px' }}>Motivo de ingreso (opcional)</label>
          <input type="text" value={addStartReason} onChange={e => setAddStartReason(e.target.value)} placeholder="Ej: nuevo proyecto, contrato..."
            style={{ width: '100%', padding: '10px 12px', border: '1px solid #cbd5e1', borderRadius: '10px', fontSize: '14px' }} />
        </div>
        {empError && <p style={{ color: '#dc2626', fontWeight: 600, fontSize: '0.85rem', marginTop: '10px' }}>{empError}</p>}
      </Modal>

      <Modal
        isOpen={!!endingEmp}
        onClose={() => setEndingEmp(null)}
        title="Finalizar empleo"
        size="sm"
        footer={
          <>
            <Button variant="secondary" onClick={() => setEndingEmp(null)} disabled={empBusy}>Cancelar</Button>
            <Button variant="danger" onClick={submitEndEmp} loading={empBusy}>Finalizar empleo</Button>
          </>
        }
      >
        <p style={{ fontSize: '0.85rem', color: '#64748b', marginTop: 0 }}>
          {endingEmp && `Vas a finalizar el empleo de ${user.name} en ${endingEmp.company_name}. Quedará en su expediente como finalizado (los datos históricos se conservan).`}
        </p>
        <label style={{ display: 'block', fontSize: '0.8rem', fontWeight: 600, color: '#334155', marginBottom: '6px' }}>Motivo de salida (opcional)</label>
        <input type="text" value={endReason} onChange={e => setEndReason(e.target.value)} placeholder="Ej: fin de contrato, renuncia..."
          style={{ width: '100%', padding: '10px 12px', border: '1px solid #cbd5e1', borderRadius: '10px', fontSize: '14px' }} />
        {empError && <p style={{ color: '#dc2626', fontWeight: 600, fontSize: '0.85rem', marginTop: '10px' }}>{empError}</p>}
      </Modal>

      <Modal
        isOpen={!!teamBlock}
        onClose={() => setTeamBlock(null)}
        title="No puedes quitar el rol todavía"
        size="sm"
        footer={
          <>
            <Button variant="secondary" onClick={() => setTeamBlock(null)}>Cerrar</Button>
            <Button onClick={() => { setTeamBlock(null); setReassignTo(''); setShowReassign(true) }}>Reasignar equipo</Button>
          </>
        }
      >
        <p style={{ fontSize: '0.85rem', color: '#64748b', marginTop: 0 }}>
          {user.name} tiene {teamBlock?.length} profesional(es) a su cargo. Reasigna su equipo antes de quitarle el rol de Manager.
        </p>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '6px', maxHeight: 300, overflowY: 'auto' }}>
          {teamBlock?.map((m: any) => (
            <div key={m.id} style={{ display: 'flex', alignItems: 'center', gap: '10px', padding: '8px 10px', border: '1px solid var(--border, #e2e8f0)', borderRadius: '10px' }}>
              <Avatar src={m.avatar} name={m.name} size="sm" />
              <div style={{ minWidth: 0 }}>
                <div style={{ fontWeight: 600, color: '#0f172a' }}>{m.name}</div>
                <div style={{ fontSize: '0.78rem', color: '#94a3b8', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                  {(m.job_title || 'Profesional')} · {m.email}
                </div>
              </div>
            </div>
          ))}
        </div>
      </Modal>

      <Modal
        isOpen={showReassign}
        onClose={() => setShowReassign(false)}
        title="Reasignar equipo"
        size="sm"
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowReassign(false)} disabled={reassignBusy}>Cancelar</Button>
            <Button onClick={submitReassignTeam} loading={reassignBusy}>Reasignar</Button>
          </>
        }
      >
        <p style={{ fontSize: '0.85rem', color: '#64748b', marginTop: 0 }}>
          Mueve a todos los profesionales a cargo de {user.name} (en todas las empresas) a otro manager, o desasígnalos.
        </p>
        <label style={{ display: 'block', fontSize: '0.8rem', fontWeight: 600, color: '#334155', marginBottom: '6px' }}>Nuevo manager</label>
        <select
          value={reassignTo}
          onChange={e => setReassignTo(e.target.value === '' ? '' : Number(e.target.value))}
          disabled={reassignBusy}
          style={{ width: '100%', padding: '10px 12px', border: '1px solid #cbd5e1', borderRadius: '10px', fontSize: '14px', background: '#fff', color: '#334155' }}
        >
          <option value="">Sin manager (desasignar)</option>
          {managerOptions.filter(m => m.id !== user.id).map(m => (
            <option key={m.id} value={m.id}>{m.name}</option>
          ))}
        </select>
      </Modal>

      {expedienteEmp && (
        <ExpedienteModal
          userId={user.id}
          employment={expedienteEmp}
          canManage={canManage}
          onClose={() => setExpedienteEmp(null)}
        />
      )}
    </div>
  )
}
