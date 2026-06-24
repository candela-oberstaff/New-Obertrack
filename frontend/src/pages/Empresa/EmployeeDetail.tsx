import { useState, useEffect, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, UserX, Shield, Users, Eye, FileText, Building2, Pencil, Trash2, KeyRound, Copy, Check } from 'lucide-react'
import { userService, employerService } from '../../services/api'
import { useAuth } from '../../context/AuthContext'
import { useConfirm } from '../../components/ui/ConfirmProvider'
import { Modal, Button, Skeleton } from '../../components/ui'
import { Select } from '../../components/ui/Select'
import Avatar from '../../components/Common/Avatar'
import { ExpedienteModal } from '../../components/Admin/ExpedienteModal'
import EmploymentManagersEditor from '../../components/Admin/EmploymentManagersEditor'
import { UserModal } from '../../components/Admin/Modals/UserModal'
import type { User } from '../../types'
import styles from '../AdminUserDetail.module.css'

// Empleo resuelto del profesional dentro de la empresa del empleador.
// El backend (EmploymentView) trae .id y .company_id en el primer nivel.
type ActiveEmployment = { id: number; company_id: number } & Record<string, unknown>

// Detalle de un empleado para el EMPLEADOR. Espejo ACOTADO de AdminUserDetail:
// el empleador puede ver/gestionar a SU profesional dentro de SU empresa, sin las
// acciones de superadmin (editar, resetear contraseña, RBAC).
export default function EmployeeDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { user: viewer } = useAuth()
  const confirm = useConfirm()
  // Siempre es su empleado (la lista /users/employees ya está acotada a su empresa).
  const canManage = true

  const [user, setUser] = useState<User | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [actionMsg, setActionMsg] = useState<string | null>(null)
  const [actionErr, setActionErr] = useState(false)

  // Flag multi-manager: si está ON y hay empleo activo, se gestiona el CONJUNTO
  // de managers del empleo (chips) en vez del manager principal de solo lectura.
  const [multiManager, setMultiManager] = useState(false)
  // Empleo activo del profesional en MI empresa (null si no tiene -> aviso suave).
  const [activeEmployment, setActiveEmployment] = useState<ActiveEmployment | null>(null)
  const [noEmployment, setNoEmployment] = useState(false)
  // Managers candidatos del tenant (derivados de /users/employees).
  const [managerOptions, setManagerOptions] = useState<{ id: number; name: string }[]>([])
  // Equipo a cargo (solo si el usuario es manager).
  const [managedTeam, setManagedTeam] = useState<any[]>([])

  // Bloqueo al degradar un manager con equipo (reasignar antes de quitar el rol).
  const [teamBlock, setTeamBlock] = useState<any[] | null>(null)
  const [teamCheckBusy, setTeamCheckBusy] = useState(false)
  const [reassignTo, setReassignTo] = useState<number | ''>('')
  const [reassignBusy, setReassignBusy] = useState(false)

  // Expediente abierto.
  const [showExpediente, setShowExpediente] = useState(false)

  const [showEdit, setShowEdit] = useState(false)
  const [editForm, setEditForm] = useState<any>({
    name: '', email: '', password: '', user_type: 'profesional', company_name: '',
    empleador_id: undefined, manager_id: undefined, job_title: '',
    phone_number: '', country: '', city: '', location: '',
    is_manager: false, is_active: true,
  })
  const [editBusy, setEditBusy] = useState(false)
  const [editErr, setEditErr] = useState<string | null>(null)

  const [resetPwd, setResetPwd] = useState<string | null>(null)
  const [resetBusy, setResetBusy] = useState(false)
  const [resetCopied, setResetCopied] = useState(false)

  const openEdit = () => {
    if (!user) return
    setEditForm({
      name: user.name || '',
      email: user.email || '',
      password: '',
      user_type: 'profesional',
      company_name: '',
      empleador_id: user.empleador_id,
      manager_id: user.manager_id ?? undefined,
      job_title: user.job_title || '',
      phone_number: user.phone_number || '',
      country: user.country || '',
      city: user.city || '',
      location: user.location || '',
      is_manager: !!user.is_manager,
      is_active: user.is_active !== false,
    })
    setEditErr(null)
    setShowEdit(true)
  }

  const submitEdit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!id) return
    setEditBusy(true); setEditErr(null)
    try {
      const payload: Parameters<typeof employerService.updateEmployee>[1] = {
        name: editForm.name.trim(),
        email: editForm.email.trim(),
        job_title: (editForm.job_title || '').trim(),
        phone_number: (editForm.phone_number || '').trim(),
        country: (editForm.country || '').trim(),
        city: (editForm.city || '').trim(),
        location: (editForm.location || '').trim(),
        is_manager: !!editForm.is_manager,
        is_active: !!editForm.is_active,
      }
      const newMgr = editForm.manager_id ? Number(editForm.manager_id) : 0
      const curMgr = user?.manager_id ? Number(user.manager_id) : 0
      if (newMgr !== curMgr) payload.manager_id = newMgr

      await employerService.updateEmployee(Number(id), payload)
      setShowEdit(false)
      await load()
    } catch (err: any) {
      setEditErr(err?.response?.data?.error ?? 'No se pudieron guardar los cambios.')
    } finally {
      setEditBusy(false)
    }
  }

  const handleResetPassword = async () => {
    if (!user) return
    const ok = await confirm({
      title: 'Restablecer contraseña',
      message: `Se generará una contraseña temporal para ${user.name}. Tendrás que compartírsela para que vuelva a ingresar. ¿Continuar?`,
      confirmLabel: 'Restablecer',
    })
    if (!ok) return
    setResetBusy(true); setResetCopied(false)
    try {
      const { temp_password } = await employerService.resetEmployeePassword(user.id)
      setResetPwd(temp_password)
    } catch (err: any) {
      setActionErr(true)
      setActionMsg(err?.response?.data?.error ?? 'No se pudo restablecer la contraseña.')
    } finally {
      setResetBusy(false)
    }
  }

  const copyResetPwd = async () => {
    if (!resetPwd) return
    try {
      await navigator.clipboard.writeText(resetPwd)
      setResetCopied(true)
      setTimeout(() => setResetCopied(false), 2000)
    } catch { }
  }

  // Elimina (soft delete) al profesional. 409 si es manager con equipo a cargo.
  const handleDelete = async () => {
    if (!user) return
    const ok = await confirm({
      title: 'Eliminar profesional',
      message: `¿Eliminar a ${user.name}? Esta acción no se puede deshacer.`,
      confirmLabel: 'Eliminar',
      variant: 'danger',
    })
    if (!ok) return
    setBusy(true); setActionMsg(null); setActionErr(false)
    try {
      await employerService.deleteEmployee(Number(id))
      navigate('/empresa')
    } catch (err: any) {
      setActionErr(true)
      setActionMsg(err?.response?.data?.error ?? 'No se pudo eliminar el profesional.')
    } finally {
      setBusy(false)
    }
  }

  const loadManagedTeam = useCallback(async (u: User) => {
    if (!u.is_manager) { setManagedTeam([]); return }
    try {
      const rows = await employerService.getManagerReports(u.id)
      setManagedTeam(Array.isArray(rows) ? rows : [])
    } catch { setManagedTeam([]) }
  }, [])

  const load = useCallback(async () => {
    if (!id) return
    setIsLoading(true)
    try {
      const data = await userService.getById(Number(id))
      setUser(data)
      setError(null)
      loadManagedTeam(data)
    } catch {
      setError('No se pudo cargar el empleado')
    } finally {
      setIsLoading(false)
    }
  }, [id, loadManagedTeam])

  useEffect(() => { load() }, [load])

  // Flag de features (una vez al montar).
  useEffect(() => {
    let cancelled = false
    employerService.getFeatures()
      .then(f => { if (!cancelled) setMultiManager(!!f?.multi_manager_reads) })
      .catch(() => { /* sin flag: modo single */ })
    return () => { cancelled = true }
  }, [])

  // Empleo activo en MI empresa. Si falla, no tiene empleo aquí: aviso suave y
  // se deshabilita la gestión de managers / el expediente.
  useEffect(() => {
    if (!id) return
    let cancelled = false
    setActiveEmployment(null); setNoEmployment(false)
    employerService.getEmployerEmployment(Number(id))
      .then(emp => {
        if (cancelled) return
        setActiveEmployment({ ...emp, id: Number(emp.id), company_id: Number(emp.company_id) })
      })
      .catch(() => { if (!cancelled) setNoEmployment(true) })
    return () => { cancelled = true }
  }, [id])

  // Managers candidatos del tenant para el editor multi-manager y la reasignación.
  useEffect(() => {
    if (!id) return
    let cancelled = false
    userService.getEmployees()
      .then(list => {
        if (cancelled) return
        setManagerOptions(
          (list || [])
            .filter(u => u.is_manager && u.id !== Number(id))
            .map(u => ({ id: u.id, name: u.name })),
        )
      })
      .catch(() => { /* selector vacío si falla */ })
    return () => { cancelled = true }
  }, [id])

  // Promueve (true) o quita el rol de manager (false).
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
      await userService.promoteToManager(user.id, value)
      await load()
      setActionMsg(value ? 'Empleado promovido a manager.' : 'Rol de manager removido.')
    } catch (err: any) {
      setActionErr(true)
      const base = err?.response?.data?.error ?? (value ? 'No se pudo promover.' : 'No se pudo quitar el rol.')
      setActionMsg(err?.response?.status === 409 ? `${base} Reasigna su equipo primero.` : base)
    } finally { setBusy(false) }
  }

  // Antes de quitar el rol: pre-chequea el equipo. Si tiene gente a cargo, abre el
  // modal de bloqueo (lista + reasignar) en vez de degradar directo.
  const handleRemoveManagerClick = async () => {
    if (!user) return
    setTeamCheckBusy(true)
    let reports: any[] = []
    try {
      reports = await employerService.getManagerReports(user.id)
    } catch { /* el backend igual valida con 409 */ }
    setTeamCheckBusy(false)
    if (Array.isArray(reports) && reports.length > 0) {
      setReassignTo(''); setTeamBlock(reports)
      return
    }
    setManagerRole(false)
  }

  // Reasigna todo el equipo del manager a otro (o lo desasigna) antes de degradar.
  const submitReassign = async () => {
    if (!user) return
    setReassignBusy(true); setActionMsg(null); setActionErr(false)
    try {
      const data = await userService.reassignTeam(user.id, reassignTo === '' ? null : Number(reassignTo))
      setTeamBlock(null)
      setActionMsg(typeof data?.reassigned === 'number' ? `Equipo reasignado (${data.reassigned}).` : 'Equipo reasignado.')
      await load()
    } catch (err: any) {
      setActionErr(true)
      setActionMsg(err?.response?.data?.error ?? 'No se pudo reasignar el equipo.')
    } finally { setReassignBusy(false) }
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
        <button className={styles.backBtn} onClick={() => navigate('/empresa')}>
          <ArrowLeft size={18} /> Volver
        </button>
        <div className={styles.empty}>
          <UserX size={40} />
          <p>{error || 'Empleado no encontrado'}</p>
        </div>
      </div>
    )
  }

  // Empresa del empleador (su tenant). Cae al company_name del propio empleado.
  const empresaName = viewer?.company_name || user.company_name || '—'

  // Manager principal (solo lectura) cuando el multi-manager está OFF o no hay empleo.
  const primaryManagerName =
    user.manager_id != null
      ? (managerOptions.find(m => m.id === user.manager_id)?.name || `#${user.manager_id}`)
      : 'Sin asignar'

  const managersField = (multiManager && activeEmployment) ? (
    <EmploymentManagersEditor
      mode="employer"
      userId={Number(id)}
      employmentId={activeEmployment.id}
      companyId={activeEmployment.company_id}
      managerOptions={managerOptions}
      onChanged={load}
    />
  ) : (
    <span>{primaryManagerName}</span>
  )

  // Campos de la card "Información". El campo Managers ocupa toda la fila.
  const fields: { label: string; value: React.ReactNode; fullWidth?: boolean }[] = [
    { label: 'Cargo', value: user.job_title || '—' },
    { label: 'Empresa', value: empresaName },
    { label: 'Teléfono', value: user.phone_number || '—' },
    { label: 'País', value: user.country || '—' },
    { label: 'Managers', value: managersField, fullWidth: true },
  ]

  return (
    <div className={styles.page}>
      <button className={styles.backBtn} onClick={() => navigate('/empresa')}>
        <ArrowLeft size={18} /> Empleados
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
            <span className={styles.tag}>Profesional</span>
            {user.is_manager && <span className={`${styles.tag} ${styles.tagManager}`}>Manager</span>}
          </div>
        </div>
      </div>

      {canManage && (
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.5rem', margin: '0 0 1rem' }}>
          <button
            onClick={() => (user.is_manager ? handleRemoveManagerClick() : setManagerRole(true))}
            disabled={busy || teamCheckBusy}
            style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', padding: '0.55rem 1rem', borderRadius: '10px', border: '1px solid var(--border, #cbd5e1)', background: 'var(--bg-primary, #fff)', fontWeight: 600, color: user.is_manager ? '#b91c1c' : undefined, cursor: 'pointer', fontSize: '0.85rem' }}
          >
            <Shield size={15} /> {user.is_manager ? 'Quitar rol de manager' : 'Promover a manager'}
          </button>
          <button
            onClick={openEdit}
            disabled={busy}
            style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', padding: '0.55rem 1rem', borderRadius: '10px', border: '1px solid var(--border, #cbd5e1)', background: 'var(--bg-primary, #fff)', fontWeight: 600, cursor: 'pointer', fontSize: '0.85rem' }}
          >
            <Pencil size={15} /> Editar
          </button>
          {activeEmployment && (
            <button
              onClick={() => setShowExpediente(true)}
              disabled={busy}
              style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', padding: '0.55rem 1rem', borderRadius: '10px', border: '1px solid var(--border, #cbd5e1)', background: 'var(--bg-primary, #fff)', fontWeight: 600, cursor: 'pointer', fontSize: '0.85rem' }}
            >
              <FileText size={15} /> Ver expediente
            </button>
          )}
          <button
            onClick={handleResetPassword}
            disabled={busy || resetBusy}
            style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', padding: '0.55rem 1rem', borderRadius: '10px', border: '1px solid var(--border, #cbd5e1)', background: 'var(--bg-primary, #fff)', fontWeight: 600, cursor: 'pointer', fontSize: '0.85rem' }}
          >
            <KeyRound size={15} /> {resetBusy ? 'Generando...' : 'Restablecer contraseña'}
          </button>
          <button
            onClick={handleDelete}
            disabled={busy}
            style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', padding: '0.55rem 1rem', borderRadius: '10px', border: '1px solid #fecaca', background: 'var(--bg-primary, #fff)', fontWeight: 600, color: '#b91c1c', cursor: 'pointer', fontSize: '0.85rem' }}
          >
            <Trash2 size={15} /> Eliminar
          </button>
        </div>
      )}

      {noEmployment && (
        <div style={{ margin: '0 0 1rem', padding: '0.6rem 0.9rem', borderRadius: '8px', background: 'rgba(245,158,11,0.12)', color: '#b45309', fontSize: '0.85rem', fontWeight: 600, display: 'flex', alignItems: 'center', gap: '8px' }}>
          <Building2 size={16} /> Este profesional no tiene un empleo activo en tu empresa.
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
              style={f.fullWidth ? { gridColumn: '1 / -1' } : undefined}
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
            Profesionales que tienen a {user.name} como manager.
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
                    onClick={() => navigate(`/empresa/employees/${m.id}`)}
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

      {teamBlock && (
        <Modal isOpen onClose={() => setTeamBlock(null)} size="md" title="No puedes quitar el rol todavía">
          <p style={{ fontSize: '0.85rem', color: '#475569', marginTop: 0 }}>
            <strong>{user.name}</strong> tiene {teamBlock.length} profesional(es) a su cargo. Reasigna su equipo antes de quitarle el rol de Manager.
          </p>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '6px', maxHeight: 220, overflowY: 'auto', marginBottom: '14px' }}>
            {teamBlock.map((m: any) => (
              <div key={m.id} style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '6px 8px', border: '1px solid #e2e8f0', borderRadius: '8px' }}>
                <Avatar src={m.avatar} name={m.name} size="sm" />
                <div style={{ minWidth: 0 }}>
                  <div style={{ fontWeight: 600, fontSize: '0.82rem', color: '#0f172a' }}>{m.name}</div>
                  <div style={{ fontSize: '0.72rem', color: '#94a3b8', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{(m.job_title || 'Profesional')} · {m.email}</div>
                </div>
              </div>
            ))}
          </div>
          <label style={{ display: 'block', fontSize: '0.78rem', fontWeight: 600, color: '#334155', marginBottom: '6px' }}>Reasignar todo el equipo a:</label>
          <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
            <div style={{ flex: 1 }}>
              <Select
                fullWidth
                clearable
                placeholder="Sin manager (desasignar)"
                value={reassignTo}
                onChange={v => setReassignTo(v === '' || v == null ? '' : Number(v))}
                options={managerOptions.filter(m => m.id !== user.id).map(m => ({ value: m.id, label: m.name }))}
              />
            </div>
            <Button onClick={submitReassign} loading={reassignBusy}>Reasignar</Button>
          </div>
        </Modal>
      )}

      {showEdit && (
        <UserModal
          title="Editar profesional"
          mode="edit"
          employerMode
          form={editForm}
          setForm={setEditForm}
          employers={[]}
          managers={managerOptions as unknown as User[]}
          onClose={() => { if (!editBusy) setShowEdit(false) }}
          onSubmit={submitEdit}
          submitLabel="Guardar cambios"
          busy={editBusy}
          error={editErr}
        />
      )}

      {resetPwd && (
        <Modal
          isOpen
          onClose={() => setResetPwd(null)}
          title="Contraseña restablecida"
          size="md"
          footer={<Button onClick={() => setResetPwd(null)}>Listo</Button>}
        >
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.85rem' }}>
            <div style={{ padding: '0.7rem 0.9rem', borderRadius: 10, background: 'rgba(16,185,129,0.1)', color: '#059669', fontSize: '0.88rem', fontWeight: 600 }}>
              Contraseña temporal generada para {user?.name}.
            </div>
            <div>
              <label style={{ display: 'block', fontSize: '0.8rem', fontWeight: 600, color: '#334155', marginBottom: '6px' }}>Contraseña temporal</label>
              <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
                <code style={{ flex: 1, padding: '0.6rem 0.75rem', borderRadius: 8, border: '1px solid #e2e8f0', background: '#f8fafc', fontSize: '0.95rem', fontFamily: 'monospace', userSelect: 'all', wordBreak: 'break-all' }}>{resetPwd}</code>
                <Button type="button" variant="secondary" onClick={copyResetPwd}>
                  {resetCopied ? <Check size={15} /> : <Copy size={15} />} {resetCopied ? 'Copiada' : 'Copiar'}
                </Button>
              </div>
            </div>
            <p style={{ margin: 0, fontSize: '0.8rem', color: '#b45309', fontWeight: 600 }}>
              Compártela por un canal seguro; no se vuelve a mostrar.
            </p>
          </div>
        </Modal>
      )}

      {showExpediente && activeEmployment && (
        <ExpedienteModal
          userId={Number(id)}
          employment={activeEmployment}
          canManage
          employerMode
          onClose={() => setShowExpediente(false)}
        />
      )}
    </div>
  )
}
