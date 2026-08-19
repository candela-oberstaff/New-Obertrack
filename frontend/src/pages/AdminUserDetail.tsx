import { useState, useEffect, useCallback } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, UserX, Power, KeyRound, Shield, UserCog, Pencil, Building2, Plus, LogOut, FileText, RotateCcw, Users, Eye, MailCheck, GraduationCap } from 'lucide-react'
import { userService, adminService, authService } from '../services/api'
import { rbacService } from '../services/rbac.service'
import { inductionService } from '../services/induction.service'
import { useAuth } from '../context/AuthContext'
import { Modal, Button, RecordPager, DatePicker } from '../components/ui'
import { useConfirm } from '../components/ui/ConfirmProvider'
import { Select } from '../components/ui/Select'
import type { User, CompanyRole, CompanyGroup } from '../types'
import Avatar from '../components/Common/Avatar'
import { Skeleton } from '../components/ui'
import { UserModal } from '../components/Admin/Modals/UserModal'
import { ExpedienteModal } from '../components/Admin/ExpedienteModal'
import { ProfileChangeReviewPanel } from '../components/Admin/ProfileChangeReviewPanel'
import { InductionStatusPanel } from '../components/Admin/InductionStatusPanel'
import EmploymentManagersEditor from '../components/Admin/EmploymentManagersEditor'
import { hierarchyLabel } from '../lib/permissions'
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
  // CS gestiona usuarios como el superadmin. La línea roja (cuentas
  // superadmin) la aplica el backend: a un CS la ficha de un superadmin ni
  // siquiera le carga, así que si llegamos a pintar botones el objetivo es
  // gestionable.
  const canManage = !!viewer?.is_superadmin || viewer?.user_type === 'customer_success'
  const canReviewProfileChange = !!viewer?.is_superadmin ||
    viewer?.user_type === 'customer_success' || viewer?.user_type === 'analista_it'
  const [user, setUser] = useState<User | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [actionMsg, setActionMsg] = useState<string | null>(null)
  const [actionErr, setActionErr] = useState(false)
  // Envío del acceso desde el detalle: el correo del alta masiva a veces no
  // llega y hay que poder reintentarlo persona por persona viendo el motivo
  // exacto del fallo (rechazo del proveedor, correo inválido, etc.).
  const [showAccess, setShowAccess] = useState(false)
  const [accessMode, setAccessMode] = useState<'invite' | 'password'>('password')
  const [accessBusy, setAccessBusy] = useState(false)
  const [accessError, setAccessError] = useState<string | null>(null)
  // Envío de la capacitación (inducción) desde la barra de acciones. El panel de
  // abajo se remonta con esta llave para que refleje el envío sin recargar.
  const [inductionBusy, setInductionBusy] = useState(false)
  const [inductionKey, setInductionKey] = useState(0)
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
  // Id del empleo cuya fecha de ingreso se está guardando (uno por fila) y lo
  // que el usuario acaba de escribir en él. El borrador existe para que el
  // input muestre lo elegido mientras viaja al servidor; al volver se borra, y
  // así una fecha rechazada revierte sola a la que sigue guardada.
  const [startBusy, setStartBusy] = useState<number | null>(null)
  const [startDraft, setStartDraft] = useState<Record<number, string>>({})
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

  // El aviso de la última acción pertenece al usuario sobre el que se hizo: al
  // pasar al siguiente con el paginador se limpia, para no leer "contraseña
  // reseteada" en la ficha de otro.
  useEffect(() => {
    setActionMsg(null)
    setActionErr(false)
  }, [id])

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

  // Corrige la fecha de ingreso de un empleo. El alta la sella con el día en
  // que se cargó al profesional, así que casi nunca es la real: la de quien ya
  // llevaba meses trabajando cuando se le abrió la cuenta, o la que llega mal
  // desde un import. De esta fecha cuelga toda su antigüedad y su expediente.
  const handleEmploymentStart = async (emp: any, value: string) => {
    if (!user || !value) return
    setStartDraft(d => ({ ...d, [emp.id]: value }))
    if (value === String(emp.started_at ?? '').slice(0, 10)) return
    setStartBusy(emp.id); setActionMsg(null); setActionErr(false)
    try {
      await adminService.updateEmploymentStart(user.id, emp.id, value)
      await loadEmployments(user)
      // De esta fecha vive el ranking de antigüedad Y todo lo que la empresa ve
      // de esta persona (su ficha en la plantilla, el vistazo, el expediente).
      // Sin tirar esas cachés, la empresa seguiría enseñando la fecha vieja.
      invalidateAdmin()
      qc.invalidateQueries({ queryKey: ['employee-tracking', user.id] })
      qc.invalidateQueries({ queryKey: ['user-employments', user.id] })
      qc.invalidateQueries({ queryKey: ['tenant-detail', emp.company_id] })
      qc.invalidateQueries({ queryKey: ['expediente', user.id] })
      setActionMsg('Fecha de ingreso actualizada')
    } catch (err: any) {
      setActionErr(true)
      setActionMsg(err?.response?.data?.error ?? 'No se pudo actualizar la fecha de ingreso.')
    } finally {
      setStartBusy(null)
      setStartDraft(d => { const next = { ...d }; delete next[emp.id]; return next })
    }
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

  // Reenvía el acceso a esta persona. No reenvía la clave del import (se guarda
  // hasheada): emite un acceso nuevo, igual que el envío masivo.
  const sendAccess = async () => {
    if (!user) return
    setAccessBusy(true); setAccessError(null)
    try {
      const r = await adminService.sendAccessEmails({ user_ids: [user.id], mode: accessMode })
      if (r.sent > 0) {
        setShowAccess(false)
        setActionErr(false)
        setActionMsg(accessMode === 'invite'
          ? `Enlace para crear la contraseña enviado a ${user.email}. Caduca en 24 horas.`
          : `Contraseña nueva enviada a ${user.email}. La anterior dejó de funcionar.`)
      } else {
        // El backend devuelve el motivo real del fallo (rechazo del proveedor,
        // correo inválido...): mostrarlo es el punto de este botón.
        setAccessError(r.failed?.[0]?.error ?? 'El servidor no pudo enviar el correo.')
      }
    } catch (err: any) {
      const status = err?.response?.status
      setAccessError(
        err?.response?.data?.error
        ?? (status ? `El servidor respondió con un error ${status}.` : 'No hubo respuesta del servidor.')
      )
    } finally { setAccessBusy(false) }
  }

  // Manda (o remanda) el correo de la capacitación. El enlace viejo se invalida
  // siempre: si el correo no llegó, reenviar el mismo token no resuelve nada, y
  // rotarlo evita revivir un enlace que quedó circulando.
  const sendInduction = async () => {
    if (!user) return
    const ok = await confirm({
      title: 'Enviar capacitación',
      message: `Se le enviará a ${user.email} el enlace a la capacitación, con sus intentos en cero. Su acceso queda bloqueado hasta que la apruebe.`,
      confirmLabel: 'Enviar',
    })
    if (!ok) return

    setInductionBusy(true); setActionMsg(null); setActionErr(false)
    try {
      // Con inducción ya registrada se reenvía sobre la misma (conserva el
      // historial de intentos); sin ella hay que emitirla desde cero.
      let registered = true
      try { await inductionService.getUserStatus(user.id) } catch { registered = false }
      if (registered) await inductionService.resetUser(user.id)
      else await inductionService.inviteUser(user.id)
      setActionMsg(`Capacitación enviada a ${user.email}.`)
      setInductionKey(k => k + 1)
      await load()
    } catch (err: any) {
      setActionErr(true)
      // El backend explica por qué no se pudo (inducción apagada, sin
      // cuestionario, ya aprobada): mostrarlo tal cual es el punto del botón.
      setActionMsg(err?.response?.data?.error ?? 'No se pudo enviar la capacitación.')
    } finally { setInductionBusy(false) }
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
      client_since: (user.client_since || '').slice(0, 10),
      empleador_id: user.empleador_id || '',
      manager_id: user.manager_id || '',
      is_active: user.is_active,
      is_manager: user.is_manager,
      is_supervisor: !!user.is_supervisor,
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

  // Fields common to every user type. El cargo lo tiene cualquier cuenta, no
  // solo los profesionales: en una empresa es el puesto de quien la gestiona, y
  // además alimenta la variable {{cargo}} de los correos.
  const common: { label: string; value: React.ReactNode }[] = [
    { label: 'Cargo', value: user.job_title || '—' },
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
  // Tope del selector de fecha de ingreso.
  //
  // La incorporación PUEDE ser futura: se contrata a alguien hoy para que
  // empiece la semana que viene, y antes el campo obligaba a esperar al día de
  // la incorporación para poder cargarla. El tope de dos años es el mismo que
  // aplica el backend, y está solo para atajar el año tecleado de más.
  const maxStartISO = (() => {
    const d = new Date()
    d.setFullYear(d.getFullYear() + 2)
    return d.toISOString().slice(0, 10)
  })()

  let specific: { label: string; value: React.ReactNode }[] = []
  if (user.user_type === 'empleador') {
    specific = [
      { label: 'Empresa', value: user.company_name || '—' },
      // El alta corregida a mano manda sobre "Registrado", que es solo cuándo
      // se creó la cuenta en Obertrack.
      {
        label: 'Alta como cliente',
        value: user.client_since
          ? new Date(user.client_since).toLocaleDateString('es-ES')
          : (user.created_at ? `${new Date(user.created_at).toLocaleDateString('es-ES')} (creación de la cuenta)` : '—'),
      },
    ]
  } else if (user.user_type === 'profesional' || user.user_type === 'customer_success') {
    // Customer Success también trabaja: se vincula a una empresa y tiene manager
    // que le aprueba las horas, sin perder por eso su alcance de gestión. Antes
    // esta rama era solo para profesionales, así que a un CS no había manera de
    // asignarle manager desde la ficha aunque el backend ya lo admitía.
    specific = [
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
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', marginBottom: '24px' }}>
        <button className={styles.backBtn} style={{ marginBottom: 0 }} onClick={() => navigate('/admin')}>
          <ArrowLeft size={18} /> Usuarios
        </button>
        <RecordPager
          scope="admin-users"
          currentId={Number(id)}
          toPath={uid => `/admin/users/${uid}`}
          noun="usuario"
        />
      </div>

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
            {!user.is_superadmin && hierarchyLabel(user) && (
              <span className={`${styles.tag} ${styles.tagManager}`}>{hierarchyLabel(user)}</span>
            )}
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
        {!!user.email && (
          <button onClick={() => { setAccessError(null); setShowAccess(true) }} disabled={busy}
            style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', padding: '0.55rem 1rem', borderRadius: '10px', border: '1px solid var(--border, #cbd5e1)', background: 'var(--bg-primary, #fff)', fontWeight: 600, cursor: 'pointer', fontSize: '0.85rem' }}>
            <MailCheck size={15} /> Enviar acceso por correo
          </button>
        )}
        {user.user_type === 'profesional' && !!user.email && (
          <button onClick={sendInduction} disabled={busy || inductionBusy}
            style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', padding: '0.55rem 1rem', borderRadius: '10px', border: '1px solid var(--border, #cbd5e1)', background: 'var(--bg-primary, #fff)', fontWeight: 600, cursor: 'pointer', fontSize: '0.85rem' }}>
            <GraduationCap size={15} /> Enviar capacitación
          </button>
        )}
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

      {user.user_type === 'profesional' && (
        <ProfileChangeReviewPanel
          userId={user.id}
          user={user}
          canReview={canReviewProfileChange}
          onApplied={load}
        />
      )}

      {/* Inducción: el estado si pasó por ella, o la acción de enviársela si no
          (alta manual / importación). Ambas las permite el backend a superadmin
          y customer success. */}
      {user.user_type === 'profesional' && (
        <InductionStatusPanel
          key={inductionKey}
          userId={user.id}
          isProfessional
          canReset={!!viewer?.is_superadmin || viewer?.user_type === 'customer_success'}
        />
      )}

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
                      {/* 10px y no 5: el campo de fecha es una caja con borde,
                          y pegado al texto parecía continuación de la frase en
                          vez de algo que se puede tocar. */}
                      <div style={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: '10px', fontSize: '0.8rem', color: '#94a3b8' }}>
                        <span>{emp.job_title || 'Sin cargo'} · desde</span>
                        {/* La fecha de ingreso se edita aquí y no en el modal
                            del usuario: es de ESTE empleo, y quien trabaja en
                            dos empresas tiene una distinta en cada una. */}
                        {canManage ? (
                          <DatePicker
                            compact
                            value={startDraft[emp.id] ?? String(emp.started_at ?? '').slice(0, 10)}
                            max={ended && emp.ended_at ? String(emp.ended_at).slice(0, 10) : maxStartISO}
                            disabled={startBusy === emp.id}
                            onChange={v => handleEmploymentStart(emp, v)}
                            title="Fecha de ingreso"
                            ariaLabel="Fecha de ingreso"
                          />
                        ) : (
                          <span>{new Date(emp.started_at).toLocaleDateString('es-ES')}</span>
                        )}
                        {ended && emp.ended_at && <span>· hasta {new Date(emp.ended_at).toLocaleDateString('es-ES')}</span>}
                        {ended && emp.end_reason ? <span>· {emp.end_reason}</span> : null}
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
        isOpen={showAccess}
        onClose={() => setShowAccess(false)}
        title="Enviar acceso por correo"
        size="sm"
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowAccess(false)} disabled={accessBusy}>Cancelar</Button>
            <Button onClick={sendAccess} loading={accessBusy}>Enviar</Button>
          </>
        }
      >
        <p style={{ fontSize: '0.85rem', color: '#64748b', marginTop: 0 }}>
          Se enviará a <strong>{user.email}</strong>. La contraseña generada al importar no se puede reenviar (se guarda cifrada), así que esto emite un acceso nuevo.
        </p>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', marginBottom: '10px' }}>
          {([
            { value: 'password', title: 'Contraseña temporal', hint: 'Genera una clave nueva y la manda en el correo. La anterior deja de servir.' },
            { value: 'invite', title: 'Enlace para crear su contraseña', hint: 'No viaja ninguna clave. El enlace caduca en 24 horas.' },
          ] as const).map(opt => (
            <label key={opt.value}
              style={{ display: 'flex', gap: '10px', alignItems: 'flex-start', padding: '10px 12px', borderRadius: '10px', cursor: 'pointer', border: `1px solid ${accessMode === opt.value ? '#7c3aed' : 'var(--border, #e2e8f0)'}`, background: accessMode === opt.value ? '#f5f3ff' : 'transparent' }}>
              <input type="radio" name="access-mode" value={opt.value} checked={accessMode === opt.value}
                onChange={() => setAccessMode(opt.value)} disabled={accessBusy} style={{ marginTop: '3px' }} />
              <span>
                <span style={{ display: 'block', fontWeight: 600, fontSize: '0.85rem', color: '#334155' }}>{opt.title}</span>
                <span style={{ display: 'block', fontSize: '0.78rem', color: '#94a3b8' }}>{opt.hint}</span>
              </span>
            </label>
          ))}
        </div>
        {accessError && <p style={{ color: '#dc2626', fontWeight: 600, fontSize: '0.85rem', margin: 0 }}>{accessError}</p>}
      </Modal>

      <Modal
        isOpen={showAddEmp}
        isDirty={addCompanyId !== '' || addJobTitle.trim() !== '' || addStartReason.trim() !== ''}
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
        isDirty={endReason.trim() !== ''}
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
        isDirty={reassignTo !== ''}
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
