import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Building2, Users, LayoutGrid, CheckSquare, Activity, Ban, CheckCircle2, Mail, Calendar, RefreshCw, ChevronLeft, ChevronRight, Pencil, Search, Clock, Hourglass, Inbox, Wand2, X, ClipboardList, UserPlus, UserMinus, Sparkles } from 'lucide-react'

// Icono y color por tipo de evento del expediente de la empresa.
const ACTIVITY_STYLE: Record<string, { icon: typeof Activity; color: string }> = {
  company_created: { icon: Sparkles, color: '#2563eb' },
  employee_joined: { icon: UserPlus, color: '#059669' },
  employee_left: { icon: UserMinus, color: '#b45309' },
  work_hour: { icon: Clock, color: '#64748b' },
  follow_up: { icon: ClipboardList, color: '#7c3aed' },
  company_suspended: { icon: Ban, color: '#dc2626' },
  company_reactivated: { icon: CheckCircle2, color: '#059669' },
}
import { useTenantDetail } from '../../hooks'
import { adminService } from '../../services/api'
import { useAuth } from '../../context/AuthContext'
import type { EmployeeSummary } from '../../types'
import Avatar from '../../components/Common/Avatar'
import { Modal, Button } from '../../components/ui'
import { Select } from '../../components/ui/Select'
import { COUNTRY_OPTIONS, getStatesForCountry } from '../../components/Auth/countries'
import { INDUSTRY_OPTIONS } from '../../components/Auth/industries'
import { ArchivedList } from '../../components/Admin/ArchivedList'
import styles from './Tenants.module.css'

const EMP_PER_PAGE = 10

const EMPTY_EDIT_FORM = {
  company_name: '',
  industry: '',
  phone_number: '',
  country: '',
  state: '',
  city: '',
  location: '',
  address: '',
}

function generatePassword(): string {
  // Sin caracteres ambiguos (0/O, 1/l/I) para que sea fácil de dictar.
  const chars = 'ABCDEFGHJKMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789'
  const bytes = crypto.getRandomValues(new Uint8Array(12))
  let pw = Array.from(bytes).map(b => chars[b % chars.length]).join('')
  if (!/\d/.test(pw)) pw = pw.slice(0, -1) + '7'
  if (!/[a-zA-Z]/.test(pw)) pw = pw.slice(0, -1) + 'k'
  return pw
}

export default function TenantDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { user: viewer } = useAuth()
  // CS entra en modo consulta: sin editar, suspender ni tocar Profesionales.
  const canManage = !!viewer?.is_superadmin
  const tenantId = Number(id)
  const { tenant, employees, activity, isLoading, error, refresh, suspendTenant, activateTenant, toggleEmployeeStatus, resetEmployeePassword } = useTenantDetail(tenantId)

  const [tab, setTab] = useState<'resumen' | 'usuarios' | 'actividad' | 'archivados'>('resumen')

  // Archivados de esta empresa (bajas + cuentas desactivadas).
  const [archived, setArchived] = useState<any[]>([])
  const loadArchived = async () => {
    try { setArchived(await adminService.getTenantArchived(tenantId)) } catch { /* noop */ }
  }
  useEffect(() => { if (tab === 'archivados') loadArchived() }, [tab]) // eslint-disable-line react-hooks/exhaustive-deps

  // Edición de la empresa
  const [showEdit, setShowEdit] = useState(false)
  const [editForm, setEditForm] = useState({ ...EMPTY_EDIT_FORM })
  const [editSaving, setEditSaving] = useState(false)
  const [editError, setEditError] = useState<string | null>(null)

  // Reset de contraseña de empleado
  const [resetTarget, setResetTarget] = useState<EmployeeSummary | null>(null)
  const [resetPassword, setResetPassword] = useState('')
  const [resetSaving, setResetSaving] = useState(false)
  const [resetError, setResetError] = useState<string | null>(null)

  // Búsqueda, filtros y paginación de Profesionales
  const [empSearch, setEmpSearch] = useState('')
  const [empRole, setEmpRole] = useState('')
  const [empStatus, setEmpStatus] = useState('')
  const [empPage, setEmpPage] = useState(1)

  useEffect(() => {
    setEmpPage(1)
  }, [empSearch, empRole, empStatus])

  const empHasFilters = !!(empSearch.trim() || empRole || empStatus)
  const clearEmpFilters = () => {
    setEmpSearch('')
    setEmpRole('')
    setEmpStatus('')
  }

  const empFiltered = employees
    .filter(emp => {
      const q = empSearch.trim().toLowerCase()
      if (q && !(emp.name?.toLowerCase().includes(q) || emp.email?.toLowerCase().includes(q))) return false
      if (empRole && emp.user_type !== empRole) return false
      if (empStatus === 'active' && !emp.is_active) return false
      if (empStatus === 'inactive' && emp.is_active) return false
      return true
    })
    .sort((a, b) => (a.name || '').localeCompare(b.name || '', 'es', { sensitivity: 'base' }))

  const empTotalPages = Math.max(1, Math.ceil(empFiltered.length / EMP_PER_PAGE))
  const empCurrentPage = Math.min(empPage, empTotalPages)
  const empPaginated = empFiltered.slice((empCurrentPage - 1) * EMP_PER_PAGE, empCurrentPage * EMP_PER_PAGE)

  const handleToggleEmployee = async (e: React.MouseEvent, emp: EmployeeSummary) => {
    e.stopPropagation()
    await toggleEmployeeStatus(emp)
  }

  const openReset = (e: React.MouseEvent, emp: EmployeeSummary) => {
    e.stopPropagation()
    setResetTarget(emp)
    setResetPassword('')
    setResetError(null)
  }

  const handleResetSubmit = async () => {
    if (!resetTarget) return
    if (resetPassword.length < 8 || !/\d/.test(resetPassword) || !/[a-zA-Z]/.test(resetPassword)) {
      setResetError('La contraseña debe tener al menos 8 caracteres con letras y números.')
      return
    }
    setResetSaving(true)
    setResetError(null)
    try {
      await resetEmployeePassword(resetTarget.id, resetPassword)
      setResetTarget(null)
    } catch (err: any) {
      setResetError(err?.response?.data?.error || 'No se pudo resetear la contraseña')
    } finally {
      setResetSaving(false)
    }
  }

  const openEdit = () => {
    if (!tenant) return
    setEditForm({
      company_name: tenant.company_name || '',
      industry: tenant.industry || '',
      phone_number: tenant.phone_number || '',
      country: tenant.country || '',
      state: tenant.state || '',
      city: tenant.city || '',
      location: tenant.location || '',
      address: tenant.address || '',
    })
    setEditError(null)
    setShowEdit(true)
  }

  const handleEditSubmit = async () => {
    if (!editForm.company_name.trim()) {
      setEditError('El nombre de la empresa es obligatorio')
      return
    }
    setEditSaving(true)
    setEditError(null)
    try {
      await adminService.updateUser(tenantId, editForm)
      await refresh()
      setShowEdit(false)
    } catch (err: any) {
      setEditError(err?.response?.data?.error || 'No se pudieron guardar los cambios')
    } finally {
      setEditSaving(false)
    }
  }

  if (isLoading) {
    return (
      <div className={styles.page}>
        <div className={styles.loading}>
          <div className={styles.spinner} />
          <p>Cargando empresa...</p>
        </div>
      </div>
    )
  }

  if (error || !tenant) {
    return (
      <div className={styles.page}>
        <button className={styles.backBtn} onClick={() => navigate('/admin/tenants')}>
          <ArrowLeft size={18} /> Volver
        </button>
        <div className={styles.empty}>
          <Building2 size={40} />
          <p>{error || 'Empresa no encontrada'}</p>
        </div>
      </div>
    )
  }

  const createdLabel = tenant.created_at ? new Date(tenant.created_at).toLocaleDateString('es-ES') : '-'
  const editStates = getStatesForCountry(editForm.country)

  // Iconos en estilo "suave": fondo pastel + icono del mismo tono (igual que las
  // tarjetas del panel admin), en vez de gradientes saturados.
  const kpis = [
    { value: tenant.user_count, label: 'Usuarios', icon: <Users size={24} />, bg: '#faf5ff', color: 'var(--primary)' },
    { value: tenant.board_count, label: 'Tableros', icon: <LayoutGrid size={24} />, bg: '#fffbeb', color: '#f59e0b' },
    { value: tenant.task_count, label: 'Tareas', icon: <CheckSquare size={24} />, bg: '#f5f3ff', color: '#8b5cf6' },
    { value: `${(tenant.hours_this_month ?? 0).toFixed(1)} h`, label: 'Horas este mes', icon: <Clock size={24} />, bg: '#ecfdf5', color: '#10b981' },
    { value: `${(tenant.pending_hours ?? 0).toFixed(1)} h`, label: 'Horas por aprobar', icon: <Hourglass size={24} />, bg: '#fff7ed', color: '#f97316' },
    { value: tenant.open_tickets ?? 0, label: 'Tickets abiertos', icon: <Inbox size={24} />, bg: '#fef2f2', color: '#ef4444' },
  ]

  const infoFields = [
    { label: 'Rubro', value: tenant.industry },
    { label: 'País', value: tenant.country },
    { label: 'Estado / Provincia', value: tenant.state },
    { label: 'Ciudad', value: tenant.city },
    { label: 'Ubicación', value: tenant.location },
    { label: 'Dirección', value: tenant.address },
    { label: 'Teléfono', value: tenant.phone_number },
  ]

  return (
    <div className={styles.page}>
      <button className={styles.backBtn} onClick={() => navigate('/admin/tenants')}>
        <ArrowLeft size={18} /> Empresas
      </button>

      <div className={styles.detailHeader}>
        <div className={styles.detailIdentity}>
          <div className={styles.companyLogoLg}>{tenant.company_name?.charAt(0).toUpperCase() || '?'}</div>
          <div>
            <div className={styles.detailTitleRow}>
              <h1>{tenant.company_name}</h1>
              <span className={`${styles.badge} ${tenant.is_active ? styles.badgeActive : styles.badgeSuspended}`}>
                {tenant.is_active ? 'Activa' : 'Suspendida'}
              </span>
            </div>
            <div className={styles.detailMeta}>
              <span><Mail size={14} /> {tenant.owner_name} · {tenant.owner_email}</span>
              <span><Calendar size={14} /> Alta: {createdLabel}</span>
            </div>
          </div>
        </div>
        {canManage && (
          <div style={{ display: 'flex', gap: '10px', flexWrap: 'wrap' }}>
            <Button variant="secondary" onClick={openEdit} leftIcon={<Pencil size={16} />}>
              Editar
            </Button>
            {tenant.is_active ? (
              <button className={`${styles.dangerBtn}`} onClick={suspendTenant}>
                <Ban size={18} /> Suspender acceso
              </button>
            ) : (
              <button className={`${styles.successBtn}`} onClick={activateTenant}>
                <CheckCircle2 size={18} /> Reactivar acceso
              </button>
            )}
          </div>
        )}
      </div>

      <div className={styles.subTabs}>
        <button className={tab === 'resumen' ? styles.subTabActive : styles.subTab} onClick={() => setTab('resumen')}>Resumen</button>
        <button className={tab === 'usuarios' ? styles.subTabActive : styles.subTab} onClick={() => setTab('usuarios')}>Profesionales ({employees.length})</button>
        <button className={tab === 'actividad' ? styles.subTabActive : styles.subTab} onClick={() => setTab('actividad')}>Expediente</button>
        <button className={tab === 'archivados' ? styles.subTabActive : styles.subTab} onClick={() => setTab('archivados')}>Archivados</button>
      </div>

      {tab === 'resumen' && (
        <>
          <div className={styles.kpis}>
            {kpis.map(kpi => (
              <div key={kpi.label} className={styles.kpiCard}>
                <div className={styles.kpiIcon} style={{ background: kpi.bg, color: kpi.color }}>
                  {kpi.icon}
                </div>
                <div>
                  <span className={styles.kpiValue}>{kpi.value}</span>
                  <span className={styles.kpiLabel}>{kpi.label}</span>
                </div>
              </div>
            ))}
          </div>

          <div className={styles.infoCard}>
            <h2>Información</h2>
            <div className={styles.infoGrid}>
              {infoFields.map(f => (
                <div key={f.label}>
                  <span className={styles.infoLabel}>{f.label}</span>
                  <span className={styles.infoValue}>{f.value?.trim() || '—'}</span>
                </div>
              ))}
            </div>
          </div>
        </>
      )}

      {tab === 'usuarios' && (
        employees.length === 0 ? (
          <div className={styles.empty}><Users size={40} /><p>Esta empresa no tiene Profesionales</p></div>
        ) : (
          <>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'wrap', marginBottom: 20 }}>
              <div className={styles.searchBox} style={{ margin: 0 }}>
                <Search size={18} />
                <input
                  type="text"
                  placeholder="Buscar empleado o correo..."
                  value={empSearch}
                  onChange={(e) => setEmpSearch(e.target.value)}
                />
              </div>
              <div style={{ minWidth: 190 }}>
                <Select
                  fullWidth
                  clearable
                  placeholder="Todos los roles"
                  value={empRole}
                  onChange={v => setEmpRole(v ? String(v) : '')}
                  options={[
                    { value: 'profesional', label: 'Profesional' },
                    { value: 'empleador', label: 'Empresa' },
                    { value: 'customer_success', label: 'Customer Success' },
                  ]}
                />
              </div>
              <div style={{ minWidth: 170 }}>
                <Select
                  fullWidth
                  clearable
                  placeholder="Todos los estados"
                  value={empStatus}
                  onChange={v => setEmpStatus(v ? String(v) : '')}
                  options={[
                    { value: 'active', label: 'Activos' },
                    { value: 'inactive', label: 'Inactivos' },
                  ]}
                />
              </div>
              {empHasFilters && (
                <button
                  type="button"
                  onClick={clearEmpFilters}
                  style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '9px 14px', border: '1px solid var(--glass-border)', borderRadius: '10px', background: 'transparent', color: '#64748b', fontSize: '13px', fontWeight: 600, cursor: 'pointer', whiteSpace: 'nowrap' }}
                  title="Quitar todos los filtros"
                >
                  <X size={14} /> Limpiar filtros
                </button>
              )}
            </div>

            {empFiltered.length === 0 ? (
              <div className={styles.empty}><Users size={40} /><p>Sin Profesionales que coincidan</p></div>
            ) : (
              <div className={styles.tableWrap}>
                <table className={styles.table}>
                  <thead>
                    <tr>
                      <th>Empleado</th>
                      <th>Horas (mes)</th>
                      <th>Tareas</th>
                      <th>Últ. actividad</th>
                      <th>Estado</th>
                      <th></th>
                    </tr>
                  </thead>
                  <tbody>
                    {empPaginated.map(emp => {
                      const last = emp.last_active ? new Date(emp.last_active) : null
                      const lastValid = last && !isNaN(last.getTime())
                      return (
                        <tr key={emp.id} className={styles.row} onClick={() => navigate(`/admin/tenants/${tenantId}/employees/${emp.id}`)}>
                          <td>
                            <div className={styles.companyCell}>
                              <Avatar src={emp.avatar} name={emp.name} size="sm" />
                              <div className={styles.ownerCell}>
                                <span>{emp.name}</span>
                                <small>{emp.email} · {emp.is_manager ? 'manager' : emp.user_type}</small>
                              </div>
                            </div>
                          </td>
                          <td>{emp.hours_this_month?.toFixed(1) ?? '0.0'} h</td>
                          <td>{emp.tasks_completed}/{emp.tasks_assigned}</td>
                          <td>{lastValid ? last!.toLocaleDateString('es-ES') : '—'}</td>
                          <td>
                            <span className={`${styles.badge} ${emp.is_active ? styles.badgeActive : styles.badgeSuspended}`}>
                              {emp.is_active ? 'Activo' : 'Inactivo'}
                            </span>
                          </td>
                          <td>
                            <div className={styles.rowActions}>
                              {canManage && (
                                <>
                                  <button
                                    className={`${styles.iconBtn} ${emp.is_active ? styles.danger : styles.success}`}
                                    onClick={(e) => handleToggleEmployee(e, emp)}
                                    title={emp.is_active ? 'Desactivar empleado' : 'Activar empleado'}
                                  >
                                    {emp.is_active ? <Ban size={16} /> : <CheckCircle2 size={16} />}
                                  </button>
                                  <button
                                    className={styles.iconBtn}
                                    onClick={(e) => openReset(e, emp)}
                                    title="Resetear contraseña"
                                  >
                                    <RefreshCw size={16} />
                                  </button>
                                </>
                              )}
                              <ChevronRight size={18} className={styles.chevron} />
                            </div>
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>

                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', padding: '14px 16px' }}>
                  <span style={{ fontSize: '13px', color: '#64748b' }}>
                    Mostrando {(empCurrentPage - 1) * EMP_PER_PAGE + 1}–{Math.min(empCurrentPage * EMP_PER_PAGE, empFiltered.length)} de {empFiltered.length} Profesionales
                  </span>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <button
                      type="button"
                      className={styles.iconBtn}
                      onClick={() => setEmpPage(p => Math.max(1, p - 1))}
                      disabled={empCurrentPage <= 1}
                      style={{ opacity: empCurrentPage <= 1 ? 0.4 : 1, cursor: empCurrentPage <= 1 ? 'not-allowed' : 'pointer' }}
                      title="Página anterior"
                    >
                      <ChevronLeft size={16} />
                    </button>
                    <span style={{ fontSize: '13px', fontWeight: 600, color: '#334155', whiteSpace: 'nowrap' }}>
                      Página {empCurrentPage} de {empTotalPages}
                    </span>
                    <button
                      type="button"
                      className={styles.iconBtn}
                      onClick={() => setEmpPage(p => Math.min(empTotalPages, p + 1))}
                      disabled={empCurrentPage >= empTotalPages}
                      style={{ opacity: empCurrentPage >= empTotalPages ? 0.4 : 1, cursor: empCurrentPage >= empTotalPages ? 'not-allowed' : 'pointer' }}
                      title="Página siguiente"
                    >
                      <ChevronRight size={16} />
                    </button>
                  </div>
                </div>
              </div>
            )}
          </>
        )
      )}

      {tab === 'actividad' && (
        activity.length === 0 ? (
          <div className={styles.empty}><Activity size={40} /><p>Sin movimientos en el expediente</p></div>
        ) : (
          <div className={styles.activityList}>
            {activity.map((a, i) => {
              const date = a.timestamp ? new Date(a.timestamp) : null
              const valid = date && !isNaN(date.getTime())
              const st = ACTIVITY_STYLE[a.type] || { icon: Activity, color: undefined as unknown as string }
              const Icon = st.icon
              return (
                <div key={`act-${i}`} className={styles.activityItem}>
                  <div className={styles.activityIcon} style={st.color ? { color: st.color } : undefined}>
                    <Icon size={16} />
                  </div>
                  <div>
                    <p>{a.details}</p>
                    <span className={styles.activityMeta}>
                      <strong>{a.user || 'Sistema'}</strong> · {valid ? date.toLocaleString('es-ES') : 'Fecha no disponible'}
                    </span>
                  </div>
                </div>
              )
            })}
          </div>
        )
      )}

      {tab === 'archivados' && (
        <ArchivedList entries={archived} showCompany={false} />
      )}

      <Modal
        isOpen={showEdit}
        onClose={() => setShowEdit(false)}
        title="Editar empresa"
        size="md"
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowEdit(false)} disabled={editSaving}>Cancelar</Button>
            <Button onClick={handleEditSubmit} loading={editSaving}>Guardar cambios</Button>
          </>
        }
      >
        <div className={styles.field}>
          <label>Nombre de la empresa</label>
          <input
            value={editForm.company_name}
            onChange={(e) => setEditForm(f => ({ ...f, company_name: e.target.value }))}
            placeholder="Acme S.A."
          />
        </div>
        <div className={styles.field}>
          <label>Rubro o industria</label>
          <Select
            fullWidth
            clearable
            value={editForm.industry}
            onChange={v => setEditForm(f => ({ ...f, industry: v ? String(v) : '' }))}
            placeholder="Selecciona un rubro..."
            options={INDUSTRY_OPTIONS}
          />
        </div>
        <div className={styles.field}>
          <label>Teléfono</label>
          <input
            value={editForm.phone_number}
            onChange={(e) => setEditForm(f => ({ ...f, phone_number: e.target.value }))}
            placeholder="Ej: +34 600 000 000"
          />
        </div>
        <div className={styles.field}>
          <label>País</label>
          <Select
            fullWidth
            clearable
            value={editForm.country}
            onChange={v => setEditForm(f => ({ ...f, country: v ? String(v) : '', state: '' }))}
            placeholder="Selecciona un país..."
            options={COUNTRY_OPTIONS}
          />
        </div>
        {editStates.length > 0 && (
          <div className={styles.field}>
            <label>Estado / Provincia</label>
            <Select
              fullWidth
              clearable
              value={editForm.state}
              onChange={v => setEditForm(f => ({ ...f, state: v ? String(v) : '' }))}
              placeholder="Selecciona un estado..."
              options={editStates}
            />
          </div>
        )}
        <div className={styles.field}>
          <label>Ciudad</label>
          <input
            value={editForm.city}
            onChange={(e) => setEditForm(f => ({ ...f, city: e.target.value }))}
            placeholder="Ej: Buenos Aires"
          />
        </div>
        <div className={styles.field}>
          <label>Ubicación</label>
          <input
            value={editForm.location}
            onChange={(e) => setEditForm(f => ({ ...f, location: e.target.value }))}
            placeholder="Ej: Ciudad, provincia o región"
          />
        </div>
        <div className={styles.field}>
          <label>Dirección</label>
          <input
            value={editForm.address}
            onChange={(e) => setEditForm(f => ({ ...f, address: e.target.value }))}
            placeholder="Ej: Calle, número, piso..."
          />
        </div>
        {editError && <p className={styles.errorMsg}>{editError}</p>}
      </Modal>

      <Modal
        isOpen={!!resetTarget}
        onClose={() => setResetTarget(null)}
        title={`Resetear contraseña${resetTarget ? ` — ${resetTarget.name}` : ''}`}
        size="sm"
        footer={
          <>
            <Button variant="secondary" onClick={() => setResetTarget(null)} disabled={resetSaving}>Cancelar</Button>
            <Button onClick={handleResetSubmit} loading={resetSaving} disabled={!resetPassword}>Resetear</Button>
          </>
        }
      >
        <p className={styles.modalHint}>
          Define una contraseña nueva o genera una aleatoria. Compártela con el empleado por un canal seguro: no volverá a mostrarse.
        </p>
        <div className={styles.field}>
          <label>Nueva contraseña</label>
          <div style={{ display: 'flex', gap: '8px' }}>
            <input
              type="text"
              value={resetPassword}
              onChange={(e) => setResetPassword(e.target.value)}
              placeholder="Mín. 8 caracteres con letras y números"
              style={{ flex: 1 }}
            />
            <Button variant="secondary" onClick={() => setResetPassword(generatePassword())} leftIcon={<Wand2 size={16} />}>
              Generar
            </Button>
          </div>
        </div>
        {resetError && <p className={styles.errorMsg}>{resetError}</p>}
      </Modal>
    </div>
  )
}
