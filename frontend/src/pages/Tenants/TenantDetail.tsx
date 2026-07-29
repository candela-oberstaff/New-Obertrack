import { useState, useEffect, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Building2, Users, LayoutGrid, CheckSquare, Activity, Ban, CheckCircle2, Mail, Calendar, RefreshCw, ChevronLeft, ChevronRight, Pencil, Search, Clock, Hourglass, Inbox, Wand2, X, ClipboardList, UserPlus, UserMinus, Sparkles, MessageSquare, CreditCard, Link, ShieldCheck, StickyNote, Plus, Trash2, Pin, PinOff } from 'lucide-react'

// Icono y color por tipo de evento del expediente de la empresa.
const ACTIVITY_STYLE: Record<string, { icon: typeof Activity; color: string }> = {
  company_created: { icon: Sparkles, color: '#2563eb' },
  employee_joined: { icon: UserPlus, color: '#059669' },
  employee_left: { icon: UserMinus, color: '#b45309' },
  work_hour: { icon: Clock, color: '#64748b' },
  follow_up: { icon: ClipboardList, color: '#7c3aed' },
  company_suspended: { icon: Ban, color: '#dc2626' },
  company_reactivated: { icon: CheckCircle2, color: '#059669' },
  company_note: { icon: StickyNote, color: '#0891b2' },
}

// Etiqueta corta del movimiento, para leer el expediente de un vistazo sin
// tener que deducir el tipo por el icono.
const ACTIVITY_LABEL: Record<string, string> = {
  company_created: 'Alta',
  employee_joined: 'Incorporación',
  employee_left: 'Baja',
  work_hour: 'Jornada',
  follow_up: 'Gestión',
  company_suspended: 'Suspensión',
  company_reactivated: 'Reactivación',
  company_note: 'Nota',
}

const ACTIVITY_PER_PAGE = 20
const NOTE_MAX_LENGTH = 2000

import { useTenantDetail, useTenantActivity, ACTIVITY_CATEGORIES } from '../../hooks'
import { groupByDay } from './activityGrouping'
import { adminService } from '../../services/api'
import { useAuth } from '../../context/AuthContext'
import type { EmployeeSummary } from '../../types'
import Avatar from '../../components/Common/Avatar'
import { Modal, Button, RecordPager, Skeleton } from '../../components/ui'
import { setRecordNav } from '../../lib/recordNav'
import { Select } from '../../components/ui/Select'
import { COUNTRY_OPTIONS, getStatesForCountry } from '../../components/Auth/countries'
import { INDUSTRY_OPTIONS } from '../../components/Auth/industries'
import { ArchivedList } from '../../components/Admin/ArchivedList'
import { useConfirm } from '../../components/ui/ConfirmProvider'
import { emailService } from '../../services/emailService'
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
  // CS entra en modo consulta: sin editar, suspender ni tocar profesionales.
  const canManage = !!viewer?.is_superadmin
  const tenantId = Number(id)
  const { tenant, employees, isLoading, error, refresh, suspendTenant, activateTenant, toggleEmployeeStatus, resetEmployeePassword } = useTenantDetail(tenantId)
  const confirm = useConfirm()

  // Expediente: filtros (categoría y persona) + paginación, todo de servidor.
  const [actCategory, setActCategory] = useState('')
  const [actPerson, setActPerson] = useState(0)
  const [actPage, setActPage] = useState(1)
  const {
    activity,
    total: actTotal,
    counts: actCounts,
    people: actPeople,
    pinnedNotes,
    isLoading: actLoading,
    isFetching: actFetching,
    error: actError,
    addNote,
    updateNote,
    setNotePinned,
    deleteNote,
  } = useTenantActivity(tenantId, actCategory, actPerson, actPage, ACTIVITY_PER_PAGE)

  // Últimas notas para el Resumen. Consulta propia (categoría "note", 3 por
  // página) para que no dependa de en qué página o filtro esté el Expediente;
  // React Query reutiliza el resto de consultas del hook, que comparten clave.
  const { activity: latestNotes } = useTenantActivity(tenantId, 'note', 0, 1, 3)

  const actTotalPages = Math.max(1, Math.ceil(actTotal / ACTIVITY_PER_PAGE))
  const [noteOpen, setNoteOpen] = useState(false)
  const [noteText, setNoteText] = useState('')
  const [noteSaving, setNoteSaving] = useState(false)
  const [noteError, setNoteError] = useState<string | null>(null)
  // Id de la nota que se está corrigiendo; null = se está escribiendo una nueva.
  const [noteEditingId, setNoteEditingId] = useState<number | null>(null)

  const [tab, setTab] = useState<'resumen' | 'usuarios' | 'actividad' | 'archivados' | 'zoho'>('resumen')

  // Archivados de esta empresa (bajas + cuentas desactivadas).
  const [archived, setArchived] = useState<any[]>([])
  const loadArchived = async () => {
    try { setArchived(await adminService.getTenantArchived(tenantId)) } catch { /* noop */ }
  }
  // También al cambiar de empresa: con el paginador se salta de ficha sin
  // desmontar la página, y quedarse con los archivados de la anterior sería
  // enseñar datos de otra empresa.
  useEffect(() => { if (tab === 'archivados') loadArchived() }, [tab, tenantId]) // eslint-disable-line react-hooks/exhaustive-deps

  // Edición de la empresa
  const [showEdit, setShowEdit] = useState(false)
  const [editForm, setEditForm] = useState({ ...EMPTY_EDIT_FORM })
  const [editSaving, setEditSaving] = useState(false)
  const [editError, setEditError] = useState<string | null>(null)

  // El formulario de edición se precarga con los datos de la empresa: solo
  // cuenta como trabajo lo que se haya cambiado respecto a eso.
  const editBaseline = useRef<string | null>(null)
  if (showEdit && editBaseline.current === null) editBaseline.current = JSON.stringify(editForm)
  if (!showEdit && editBaseline.current !== null) editBaseline.current = null
  const editDirty = showEdit && JSON.stringify(editForm) !== editBaseline.current

  // Reset de contraseña de empleado
  const [resetTarget, setResetTarget] = useState<EmployeeSummary | null>(null)
  const [resetPassword, setResetPassword] = useState('')
  const [resetSaving, setResetSaving] = useState(false)
  const [resetError, setResetError] = useState<string | null>(null)

  // Búsqueda, filtros y paginación de profesionales
  const [empSearch, setEmpSearch] = useState('')
  const [empRole, setEmpRole] = useState('')
  const [empStatus, setEmpStatus] = useState('')
  const [empPage, setEmpPage] = useState(1)

  // Modales de comunicación
  const [commModal, setCommModal] = useState<{
    isOpen: boolean
    type: 'email' | 'whatsapp'
    subject: string
    message: string
  }>({
    isOpen: false,
    type: 'email',
    subject: 'Contacto de soporte Oberstaff',
    message: ''
  })

  // Simulación Zoho Billing
  const [zohoStatus, setZohoStatus] = useState<'unlinked' | 'connecting' | 'linked'>('unlinked')
  const [zohoCredentials, setZohoCredentials] = useState({
    clientId: '',
    clientSecret: '',
    orgId: ''
  })
  const [zohoBills, setZohoBills] = useState<any[]>([])

  useEffect(() => {
    setEmpPage(1)
  }, [empSearch, empRole, empStatus])

  // Al saltar a otra empresa, los filtros de la plantilla anterior no aplican:
  // arrastrarlos haría parecer que la nueva no tiene profesionales.
  useEffect(() => {
    setEmpSearch('')
    setEmpRole('')
    setEmpStatus('')
    setEmpPage(1)
    setActCategory('')
    setActPerson(0)
    setActPage(1)
  }, [tenantId])

  // Cambiar de filtro reordena el expediente entero: seguir en la página 7 del
  // anterior no significa nada.
  useEffect(() => { setActPage(1) }, [actCategory, actPerson])

  // Si al borrar la última nota de la página esta se queda vacía, se retrocede
  // en vez de dejar al usuario mirando un hueco.
  useEffect(() => {
    if (actPage > actTotalPages) setActPage(actTotalPages)
  }, [actPage, actTotalPages])

  const openNewNote = () => {
    setNoteEditingId(null)
    setNoteText('')
    setNoteError(null)
    setNoteOpen(true)
  }

  const openEditNote = (note: { event_id: number; details: string }) => {
    setNoteEditingId(note.event_id)
    setNoteText(note.details)
    setNoteError(null)
    setNoteOpen(true)
  }

  const handleSaveNote = async () => {
    const text = noteText.trim()
    if (!text) {
      setNoteError('Escribe la nota antes de guardarla')
      return
    }
    setNoteSaving(true)
    setNoteError(null)
    try {
      if (noteEditingId !== null) {
        // Corregir conserva la fecha original: la nota no se mueve de sitio en
        // la cronología por arreglar una errata.
        await updateNote(noteEditingId, text)
      } else {
        await addNote(text)
        // La nota nueva es lo más reciente: se ve en la primera página.
        setActPage(1)
      }
      setNoteOpen(false)
      setNoteText('')
      setNoteEditingId(null)
    } catch (err: any) {
      setNoteError(err?.response?.data?.error || 'No se pudo guardar la nota')
    } finally {
      setNoteSaving(false)
    }
  }

  const handleTogglePin = async (note: { event_id: number; pinned?: boolean }) => {
    await setNotePinned(note.event_id, !note.pinned)
  }

  const handleDeleteNote = async (eventId: number) => {
    const ok = await confirm({
      title: '¿Eliminar esta nota?',
      message: 'Desaparecerá del expediente. El resto de movimientos no se puede borrar.',
      confirmLabel: 'Eliminar nota',
      variant: 'danger',
    })
    if (ok) await deleteNote(eventId)
  }

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

  // Igual que en el listado de empresas: la ficha del profesional hereda el
  // orden filtrado de esta tabla para poder recorrer la plantilla de una.
  const openEmployee = (employeeId: number) => {
    setRecordNav(`tenant-employees:${tenantId}`, empFiltered.map(e => e.id))
    navigate(`/admin/tenants/${tenantId}/employees/${employeeId}`)
  }

  // Suspender tumba el acceso de toda la empresa y expulsa a quien esté dentro:
  // no es un botón que se pulse sin querer.
  const handleSuspend = async () => {
    const ok = await confirm({
      title: `¿Suspender el acceso de ${tenant?.company_name}?`,
      message: `La empresa y sus ${employees.length} profesional(es) quedarán fuera de la plataforma de inmediato: se cierran las sesiones abiertas y no podrán volver a entrar hasta que se reactive.`,
      confirmLabel: 'Suspender acceso',
      variant: 'danger',
    })
    if (ok) await suspendTenant()
  }

  const handleActivate = async () => {
    const ok = await confirm({
      title: `¿Reactivar el acceso de ${tenant?.company_name}?`,
      message: 'La empresa y sus profesionales podrán volver a entrar a la plataforma.',
      confirmLabel: 'Reactivar acceso',
      variant: 'primary',
    })
    if (ok) await activateTenant()
  }

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

  const openComm = (type: 'email' | 'whatsapp') => {
    setCommModal({
      isOpen: true,
      type,
      subject: type === 'email' ? 'Contacto oficial Oberstaff' : '',
      message: ''
    })
  }

  const sendCommMessage = async () => {
    if (commModal.type === 'email') {
      if (!tenant?.owner_email) {
        alert('Esta empresa no tiene un correo de contacto asociado.')
        return
      }
      try {
        await emailService.sendQuickEmail({
          to_email: tenant.owner_email,
          to_name: tenant.owner_name || tenant.company_name,
          subject: commModal.subject || 'Contacto oficial Oberstaff',
          html_content: `<p>${commModal.message.replace(/\n/g, '<br>')}</p>`
        })
        alert(`¡Correo enviado con éxito a ${tenant.owner_email}!`)
      } catch (err) {
        console.error(err)
        alert('Error al enviar el correo. Por favor, intente de nuevo.')
      }
    } else {
      console.log(`[SIMULACIÓN] Enviando ${commModal.type.toUpperCase()} a ${tenant?.company_name}:`, {
        subject: commModal.subject,
        message: commModal.message
      })
      alert(`¡Mensaje (${commModal.type.toUpperCase()}) enviado con éxito a ${tenant?.company_name}! (Simulado)`)
    }
    setCommModal(prev => ({ ...prev, isOpen: false }))
  }

  const handleZohoConnect = (e: React.FormEvent) => {
    e.preventDefault()
    if (!zohoCredentials.clientId || !zohoCredentials.clientSecret || !zohoCredentials.orgId) {
      alert('Por favor, rellene todos los campos obligatorios.')
      return
    }
    setZohoStatus('connecting')
    setTimeout(() => {
      setZohoStatus('linked')
      setZohoBills([
        { id: 'ZB-2026-001', plan: 'Plan Premium', amount: '€149.00', status: 'Pagado', date: '2026-06-01' },
        { id: 'ZB-2026-002', plan: 'Plan Premium', amount: '€149.00', status: 'Pagado', date: '2026-05-01' },
        { id: 'ZB-2026-003', plan: 'Plan Premium', amount: '€149.00', status: 'Pendiente', date: '2026-07-01' },
      ])
    }, 1500)
  }

  const handleZohoDisconnect = () => {
    setZohoStatus('unlinked')
    setZohoBills([])
    setZohoCredentials({ clientId: '', clientSecret: '', orgId: '' })
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
    { value: tenant.user_count, label: 'Profesionales', icon: <Users size={24} />, bg: '#faf5ff', color: 'var(--primary)' },
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
      {/* El aire lo pone la fila, no el botón: dentro del flex su margen queda
          centrado y el paginador acabaría rozando la cabecera de abajo. */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', marginBottom: '20px' }}>
        <button className={styles.backBtn} style={{ marginBottom: 0 }} onClick={() => navigate('/admin/tenants')}>
          <ArrowLeft size={18} /> Empresas
        </button>
        <RecordPager
          scope="tenants"
          currentId={tenantId}
          toPath={id => `/admin/tenants/${id}`}
          noun="empresa"
        />
      </div>

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
        <div style={{ display: 'flex', gap: '10px', flexWrap: 'wrap', alignItems: 'center' }}>
          <Button variant="secondary" onClick={() => openComm('email')} leftIcon={<Mail size={16} />}>
            Enviar Correo
          </Button>
          <Button variant="secondary" onClick={() => openComm('whatsapp')} leftIcon={<MessageSquare size={16} />}>
            Enviar WhatsApp
          </Button>
          {canManage && (
            <>
              <Button variant="secondary" onClick={openEdit} leftIcon={<Pencil size={16} />}>
                Editar
              </Button>
              {tenant.is_active ? (
                <button className={`${styles.dangerBtn}`} onClick={handleSuspend}>
                  <Ban size={18} /> Suspender acceso
                </button>
              ) : (
                <button className={`${styles.successBtn}`} onClick={handleActivate}>
                  <CheckCircle2 size={18} /> Reactivar acceso
                </button>
              )}
            </>
          )}
        </div>
      </div>

      <div className={styles.subTabs}>
        <button className={tab === 'resumen' ? styles.subTabActive : styles.subTab} onClick={() => setTab('resumen')}>Resumen</button>
        <button className={tab === 'usuarios' ? styles.subTabActive : styles.subTab} onClick={() => setTab('usuarios')}>Profesionales ({employees.length})</button>
        <button className={tab === 'actividad' ? styles.subTabActive : styles.subTab} onClick={() => setTab('actividad')}>Expediente</button>
        <button className={tab === 'archivados' ? styles.subTabActive : styles.subTab} onClick={() => setTab('archivados')}>Archivados</button>
        <button className={tab === 'zoho' ? styles.subTabActive : styles.subTab} onClick={() => setTab('zoho')}>Zoho</button>
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

          {/* Últimas notas del equipo: lo que hay que saber de esta empresa al
              entrar, sin tener que ir al Expediente a buscarlo. */}
          {(latestNotes.length > 0 || canManage) && (
            <div className={styles.infoCard}>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap' }}>
                <h2 style={{ display: 'flex', alignItems: 'center', gap: 8, margin: 0 }}>
                  <StickyNote size={18} color="#0891b2" /> Notas del equipo
                </h2>
                {canManage && (
                  <Button size="sm" variant="secondary" leftIcon={<Plus size={14} />} onClick={openNewNote}>
                    Añadir nota
                  </Button>
                )}
              </div>
              {latestNotes.length === 0 ? (
                <p style={{ margin: '12px 0 0', fontSize: 13.5, color: '#94a3b8' }}>
                  Todavía no hay notas sobre esta empresa.
                </p>
              ) : (
                <>
                  <div className={styles.notesSummary} style={{ marginTop: 14 }}>
                    {latestNotes.map(note => (
                      <div key={`summary-note-${note.event_id}`} className={styles.notesSummaryItem}>
                        <p>{note.details}</p>
                        <span className={styles.timelineMeta}>
                          {note.pinned && <span className={styles.noteTitle} style={{ fontSize: 10.5 }}><Pin size={11} /> Fijada</span>}
                          <strong style={{ color: '#64748b', fontWeight: 600 }}>{note.user || 'Sistema'}</strong>
                          <span>{new Date(note.timestamp).toLocaleDateString('es-ES')}</span>
                          {note.edited_at && <span className={styles.noteEdited}>editada</span>}
                        </span>
                      </div>
                    ))}
                  </div>
                  <button
                    type="button"
                    onClick={() => { setActCategory('note'); setTab('actividad') }}
                    style={{ marginTop: 12, padding: 0, border: 'none', background: 'none', color: 'var(--primary)', fontSize: 13, fontWeight: 600, cursor: 'pointer' }}
                  >
                    Ver todas en el expediente →
                  </button>
                </>
              )}
            </div>
          )}

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
          <div className={styles.empty}><Users size={40} /><p>Esta empresa no tiene profesionales</p></div>
        ) : (
          <>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'wrap', marginBottom: 20 }}>
              <div className={styles.searchBox} style={{ margin: 0 }}>
                <Search size={18} />
                <input
                  type="text"
                  placeholder="Buscar profesional o correo..."
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
              <div className={styles.empty}><Users size={40} /><p>Sin profesionales que coincidan</p></div>
            ) : (
              <div className={styles.tableWrap}>
                <table className={styles.table}>
                  <thead>
                    <tr>
                      <th>Profesional</th>
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
                        <tr key={emp.id} className={styles.row} onClick={() => openEmployee(emp.id)}>
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
                                    title={emp.is_active ? 'Desactivar profesional' : 'Activar profesional'}
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
                    Mostrando {(empCurrentPage - 1) * EMP_PER_PAGE + 1}–{Math.min(empCurrentPage * EMP_PER_PAGE, empFiltered.length)} de {empFiltered.length} profesionales
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

      {/* El expediente es una vista de auditoría: si no hay movimientos se dice,
          no se rellena con eventos de ejemplo. */}
      {tab === 'actividad' && (
        <>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap', marginBottom: 20 }}>
            {ACTIVITY_CATEGORIES.map(cat => {
              const active = actCategory === cat.value
              return (
                <button
                  key={cat.value || 'all'}
                  type="button"
                  onClick={() => setActCategory(cat.value)}
                  style={{
                    padding: '7px 14px',
                    borderRadius: '999px',
                    border: active ? '1px solid var(--primary)' : '1px solid var(--glass-border, #e2e8f0)',
                    background: active ? 'rgba(204, 51, 204, 0.1)' : 'transparent',
                    color: active ? 'var(--primary)' : '#64748b',
                    fontSize: '13px',
                    fontWeight: 600,
                    cursor: 'pointer',
                    whiteSpace: 'nowrap',
                  }}
                  aria-pressed={active}
                >
                  {cat.label}
                  {/* El contador se calcula con el MISMO filtro de persona que
                      la lista, así que nunca promete más de lo que enseña. */}
                  {actCounts[cat.value] !== undefined && (
                    <span className={styles.chipCount}>{actCounts[cat.value]}</span>
                  )}
                </button>
              )
            })}
            {/* Filtro por persona. Las opciones salen del propio expediente, así
                que incluyen a quien ya causó baja; se busca por nombre porque
                una empresa grande trae decenas. */}
            <div style={{ minWidth: 230, marginLeft: 'auto' }}>
              <Select
                fullWidth
                clearable
                searchable
                placeholder="Todas las personas"
                value={actPerson || ''}
                onChange={v => setActPerson(v ? Number(v) : 0)}
                ariaLabel="Filtrar el expediente por persona"
                options={actPeople.map(p => ({ value: p.user_id, label: p.name }))}
              />
            </div>
            {canManage && (
              <Button
                size="sm"
                variant="secondary"
                leftIcon={<Plus size={14} />}
                onClick={openNewNote}
              >
                Añadir nota
              </Button>
            )}
          </div>

          {actError && <p className={styles.errorMsg}>{actError}</p>}

          {/* Notas fijadas: avisos vigentes, no historia. Van arriba y fuera de
              la cronología, así que no dependen del filtro ni de la página. */}
          {pinnedNotes.length > 0 && (
            <div className={styles.pinnedBox}>
              <span className={styles.pinnedTitle}>
                <Pin size={13} /> Fijado en este expediente
              </span>
              {pinnedNotes.map(note => (
                <div key={`pinned-${note.event_id}`} className={styles.pinnedItem}>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <p className={styles.noteBody}>{note.details}</p>
                    <span className={styles.timelineMeta} style={{ marginTop: 4 }}>
                      <strong style={{ color: '#64748b', fontWeight: 600 }}>{note.user || 'Sistema'}</strong>
                      <span>{new Date(note.timestamp).toLocaleDateString('es-ES')}</span>
                      {note.edited_at && <span className={styles.noteEdited}>editada</span>}
                    </span>
                  </div>
                  {canManage && (
                    <button
                      type="button"
                      className={styles.iconBtn}
                      onClick={() => handleTogglePin(note)}
                      title="Dejar de fijar"
                      aria-label="Dejar de fijar esta nota"
                    >
                      <PinOff size={15} />
                    </button>
                  )}
                </div>
              ))}
            </div>
          )}

          {actLoading ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.6rem' }}>
              {Array.from({ length: 5 }).map((_, i) => <Skeleton key={i} height={56} radius={12} />)}
            </div>
          ) : activity.length === 0 ? (
            <div className={styles.empty}>
              <Activity size={40} />
              <p>
                {actCategory || actPerson
                  ? 'Sin movimientos con estos filtros'
                  : 'Sin movimientos en el expediente'}
              </p>
              {(actCategory || actPerson) && (
                <button
                  type="button"
                  onClick={() => { setActCategory(''); setActPerson(0) }}
                  style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', marginTop: 10, padding: '8px 14px', border: '1px solid var(--glass-border)', borderRadius: '10px', background: 'transparent', color: '#64748b', fontSize: '13px', fontWeight: 600, cursor: 'pointer' }}
                >
                  <X size={14} /> Limpiar filtros
                </button>
              )}
            </div>
          ) : (
            <>
              {/* Mientras llega la página nueva se atenúa la anterior: se ve que
                  algo está pasando sin vaciar la lista. */}
              <div className={styles.timeline} style={{ opacity: actFetching ? 0.6 : 1, transition: 'opacity 0.15s ease' }}>
                {groupByDay(activity).map(group => (
                  <div key={group.key} className={styles.timelineDay}>
                    <div className={styles.timelineDayLabel}>
                      <span className={styles.timelineDayChip}>
                        {group.label}
                        <span className={styles.timelineDayCount}>
                          {group.items.length} {group.items.length === 1 ? 'movimiento' : 'movimientos'}
                        </span>
                      </span>
                    </div>
                    {group.items.map((a, i) => {
                      const date = new Date(a.timestamp)
                      const valid = !isNaN(date.getTime())
                      const st = ACTIVITY_STYLE[a.type] || { icon: Activity, color: '#64748b' }
                      const Icon = st.icon
                      const isNote = a.type === 'company_note'
                      return (
                        <div key={a.event_id ? `note-${a.event_id}` : `act-${actPage}-${group.key}-${i}`} className={styles.timelineItem}>
                          <div className={styles.timelineNode} style={{ color: st.color }}>
                            <Icon size={15} />
                          </div>
                          <div className={`${styles.timelineCard} ${isNote ? styles.isNote : ''}`}>
                            {isNote ? (
                              <>
                                {/* Cabecera explícita: quién la escribió, que es
                                    interna, y si se corrigió después. */}
                                <div className={styles.noteHeader}>
                                  <span className={styles.noteTitle}>
                                    <StickyNote size={13} /> Nota interna del equipo
                                  </span>
                                  <span className={styles.noteScope}>· no la ve la empresa</span>
                                  {a.pinned && (
                                    <span className={styles.noteScope} style={{ color: '#0e7490' }}>
                                      · fijada
                                    </span>
                                  )}
                                  {a.edited_at && <span className={styles.noteEdited}>· editada</span>}
                                  {canManage && (
                                    <div className={styles.timelineActions} style={{ marginLeft: 'auto', display: 'flex', gap: 2 }}>
                                      <button
                                        type="button"
                                        className={styles.iconBtn}
                                        onClick={() => handleTogglePin(a)}
                                        title={a.pinned ? 'Dejar de fijar' : 'Fijar arriba del expediente'}
                                        aria-label={a.pinned ? 'Dejar de fijar esta nota' : 'Fijar esta nota arriba del expediente'}
                                      >
                                        {a.pinned ? <PinOff size={15} /> : <Pin size={15} />}
                                      </button>
                                      <button
                                        type="button"
                                        className={styles.iconBtn}
                                        onClick={() => openEditNote(a)}
                                        title="Editar nota"
                                        aria-label="Editar esta nota"
                                      >
                                        <Pencil size={14} />
                                      </button>
                                      <button
                                        type="button"
                                        className={`${styles.iconBtn} ${styles.danger}`}
                                        onClick={() => handleDeleteNote(a.event_id)}
                                        title="Eliminar nota"
                                        aria-label={`Eliminar la nota de ${a.user || 'Sistema'}`}
                                      >
                                        <Trash2 size={15} />
                                      </button>
                                    </div>
                                  )}
                                </div>
                                <p className={styles.noteBody}>{a.details}</p>
                                <div className={styles.timelineMeta}>
                                  <strong style={{ color: '#64748b', fontWeight: 600 }}>{a.user || 'Sistema'}</strong>
                                  <span className={styles.timelineTime}>
                                    {valid ? date.toLocaleTimeString('es-ES', { hour: '2-digit', minute: '2-digit' }) : '—'}
                                  </span>
                                </div>
                              </>
                            ) : (
                              <div style={{ flex: 1, minWidth: 0 }}>
                                <p className={styles.timelineText}>{a.details}</p>
                                <div className={styles.timelineMeta}>
                                  <span className={styles.timelineTag} style={{ color: st.color }}>
                                    {ACTIVITY_LABEL[a.type] || a.type}
                                  </span>
                                  <strong style={{ color: '#64748b', fontWeight: 600 }}>{a.user || 'Sistema'}</strong>
                                  {/* La fecha vive en la cabecera del día: aquí
                                      basta la hora. */}
                                  <span className={styles.timelineTime}>
                                    {valid ? date.toLocaleTimeString('es-ES', { hour: '2-digit', minute: '2-digit' }) : '—'}
                                  </span>
                                </div>
                              </div>
                            )}
                          </div>
                        </div>
                      )
                    })}
                  </div>
                ))}
              </div>

              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', padding: '14px 4px' }}>
                <span style={{ fontSize: '13px', color: '#64748b' }}>
                  Mostrando {(actPage - 1) * ACTIVITY_PER_PAGE + 1}–{Math.min(actPage * ACTIVITY_PER_PAGE, actTotal)} de {actTotal} movimientos
                </span>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <button
                    type="button"
                    className={styles.iconBtn}
                    onClick={() => setActPage(p => Math.max(1, p - 1))}
                    disabled={actPage <= 1}
                    style={{ opacity: actPage <= 1 ? 0.4 : 1, cursor: actPage <= 1 ? 'not-allowed' : 'pointer' }}
                    title="Página anterior"
                  >
                    <ChevronLeft size={16} />
                  </button>
                  <span style={{ fontSize: '13px', fontWeight: 600, color: '#334155', whiteSpace: 'nowrap' }}>
                    Página {actPage} de {actTotalPages}
                  </span>
                  <button
                    type="button"
                    className={styles.iconBtn}
                    onClick={() => setActPage(p => Math.min(actTotalPages, p + 1))}
                    disabled={actPage >= actTotalPages}
                    style={{ opacity: actPage >= actTotalPages ? 0.4 : 1, cursor: actPage >= actTotalPages ? 'not-allowed' : 'pointer' }}
                    title="Página siguiente"
                  >
                    <ChevronRight size={16} />
                  </button>
                </div>
              </div>
            </>
          )}
        </>
      )}

      {tab === 'archivados' && (
        <ArchivedList entries={archived} showCompany={false} />
      )}

      {tab === 'zoho' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
          <div className={styles.infoCard} style={{ margin: 0 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px', flexWrap: 'wrap', gap: '12px' }}>
              <div>
                <h2 style={{ display: 'flex', alignItems: 'center', gap: '8px', margin: 0 }}>
                  <CreditCard size={20} color="var(--primary)" /> Integración Zoho Billing
                </h2>
                <p style={{ color: '#64748b', fontSize: '13px', margin: '4px 0 0' }}>Conecta este cliente con Zoho para sincronizar suscripciones y gestionar cobros.</p>
              </div>
              <div>
                {zohoStatus === 'unlinked' && (
                  <span className={styles.badge} style={{ background: '#f1f5f9', color: '#64748b' }}>No vinculado</span>
                )}
                {zohoStatus === 'connecting' && (
                  <span className={styles.badge} style={{ background: '#fef3c7', color: '#d97706', display: 'inline-flex', alignItems: 'center', gap: '6px' }}>
                    <RefreshCw size={12} className={styles.spinner} style={{ border: 'none', width: 'auto', height: 'auto', animation: 'spin 1s linear infinite' }} /> Conectando...
                  </span>
                )}
                {zohoStatus === 'linked' && (
                  <span className={styles.badge} style={{ background: '#dcfce7', color: '#15803d', display: 'inline-flex', alignItems: 'center', gap: '4px' }}>
                    <ShieldCheck size={14} /> Vinculado con éxito
                  </span>
                )}
              </div>
            </div>

            {zohoStatus === 'unlinked' && (
              <form onSubmit={handleZohoConnect} style={{ display: 'flex', flexDirection: 'column', gap: '14px', maxWidth: '500px' }}>
                <div className={styles.field}>
                  <label>Client ID *</label>
                  <input
                    type="text"
                    required
                    value={zohoCredentials.clientId}
                    onChange={(e) => setZohoCredentials(prev => ({ ...prev, clientId: e.target.value }))}
                    placeholder="Ej: 1000.XXXXXX..."
                  />
                </div>
                <div className={styles.field}>
                  <label>Client Secret *</label>
                  <input
                    type="password"
                    required
                    value={zohoCredentials.clientSecret}
                    onChange={(e) => setZohoCredentials(prev => ({ ...prev, clientSecret: e.target.value }))}
                    placeholder="Clave privada del cliente API"
                  />
                </div>
                <div className={styles.field}>
                  <label>Organization ID *</label>
                  <input
                    type="text"
                    required
                    value={zohoCredentials.orgId}
                    onChange={(e) => setZohoCredentials(prev => ({ ...prev, orgId: e.target.value }))}
                    placeholder="Ej: 712003..."
                  />
                </div>
                <div style={{ display: 'flex', gap: '10px', marginTop: '10px' }}>
                  <Button type="submit" leftIcon={<Link size={16} />}>Vincular cuenta Zoho</Button>
                </div>
              </form>
            )}

            {zohoStatus === 'connecting' && (
              <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', padding: '40px 0', gap: '12px' }}>
                <RefreshCw size={36} className={styles.spinner} style={{ border: 'none', color: 'var(--primary)', width: 'auto', height: 'auto' }} />
                <p style={{ fontSize: '14px', fontWeight: 600, color: '#334155' }}>Estableciendo conexión segura con la API de Zoho Billing...</p>
              </div>
            )}

            {zohoStatus === 'linked' && (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '12px 16px', background: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: '10px' }}>
                  <div style={{ fontSize: '13px', color: '#475569' }}>
                    <strong>ID Organización:</strong> {zohoCredentials.orgId} · <strong>Cliente:</strong> {tenant.company_name}
                  </div>
                  <Button variant="secondary" size="sm" onClick={handleZohoDisconnect}>Desconectar Zoho</Button>
                </div>

                <div>
                  <h3 style={{ fontSize: '15px', fontWeight: 700, margin: '0 0 12px' }}>Facturas Recientes Sincronizadas</h3>
                  <div className={styles.tableWrap}>
                    <table className={styles.table}>
                      <thead>
                        <tr>
                          <th>ID Factura</th>
                          <th>Plan / Servicio</th>
                          <th>Monto</th>
                          <th>Fecha</th>
                          <th>Estado</th>
                        </tr>
                      </thead>
                      <tbody>
                        {zohoBills.map((bill) => (
                          <tr key={bill.id}>
                            <td style={{ fontWeight: 600, color: 'var(--primary)' }}>{bill.id}</td>
                            <td>{bill.plan}</td>
                            <td>{bill.amount}</td>
                            <td>{bill.date}</td>
                            <td>
                              <span className={bill.status === 'Pagado' ? styles.badgeActive : styles.badgePending}>
                                {bill.status}
                              </span>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Modales de Comunicación en detalle */}
      <Modal
        isOpen={commModal.isOpen}
        isDirty={!!commModal.message?.trim()}
        onClose={() => setCommModal(prev => ({ ...prev, isOpen: false }))}
        title={commModal.type === 'email' ? 'Enviar Correo (Simulado)' : 'Enviar WhatsApp (Simulado)'}
        size="md"
        footer={
          <>
            <Button variant="secondary" onClick={() => setCommModal(prev => ({ ...prev, isOpen: false }))}>Cancelar</Button>
            <Button onClick={sendCommMessage} disabled={!commModal.message}>Enviar</Button>
          </>
        }
      >
        <p className={styles.modalHint}>Escribe el mensaje directo para {tenant.company_name}.</p>
        {commModal.type === 'email' && (
          <div className={styles.field}>
            <label>Asunto</label>
            <input
              type="text"
              value={commModal.subject}
              onChange={(e) => setCommModal(prev => ({ ...prev, subject: e.target.value }))}
              placeholder="Asunto del correo"
            />
          </div>
        )}
        <div className={styles.field}>
          <label>Mensaje</label>
          <textarea
            value={commModal.message}
            onChange={(e) => setCommModal(prev => ({ ...prev, message: e.target.value }))}
            placeholder={commModal.type === 'email' ? 'Escribe el correo aquí...' : 'Escribe tu mensaje...'}
            rows={5}
            style={{ width: '100%', padding: '10px 14px', border: '1px solid #e2e8f0', borderRadius: 'var(--radius)', fontSize: '14px', outline: 'none' }}
          />
        </div>
      </Modal>

      <Modal
        isOpen={noteOpen}
        isDirty={noteText.trim() !== ''}
        onClose={() => setNoteOpen(false)}
        title={noteEditingId !== null ? 'Editar nota del expediente' : 'Añadir nota al expediente'}
        size="md"
        footer={
          <>
            <Button variant="secondary" onClick={() => setNoteOpen(false)} disabled={noteSaving}>Cancelar</Button>
            <Button onClick={handleSaveNote} loading={noteSaving} disabled={!noteText.trim()}>
              {noteEditingId !== null ? 'Guardar cambios' : 'Guardar nota'}
            </Button>
          </>
        }
      >
        <p className={styles.modalHint}>
          {noteEditingId !== null
            ? 'Se conserva la fecha original y quedará marcada como editada, para que el expediente siga siendo fiel.'
            : `Lo que el sistema no puede deducir solo: una llamada, un acuerdo, un aviso. Queda fechada y firmada con tu nombre en el expediente de ${tenant.company_name}, y solo la ve el equipo de la plataforma.`}
        </p>
        <div className={styles.field}>
          <label>Nota</label>
          <textarea
            value={noteText}
            onChange={(e) => setNoteText(e.target.value.slice(0, NOTE_MAX_LENGTH))}
            placeholder="Ej: Llamada con el responsable — renuevan el plan en septiembre."
            rows={5}
            autoFocus
            style={{ width: '100%', padding: '10px 14px', border: '1px solid #e2e8f0', borderRadius: 'var(--radius)', fontSize: '14px', outline: 'none', resize: 'vertical' }}
          />
          <span style={{ display: 'block', marginTop: 4, textAlign: 'right', fontSize: '12px', color: noteText.length >= NOTE_MAX_LENGTH ? '#dc2626' : '#94a3b8' }}>
            {noteText.length}/{NOTE_MAX_LENGTH}
          </span>
        </div>
        {noteError && <p className={styles.errorMsg}>{noteError}</p>}
      </Modal>

      <Modal
        isOpen={showEdit}
        isDirty={editDirty}
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
        isDirty={resetPassword.trim() !== ''}
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
          Define una contraseña nueva o genera una aleatoria. Compártela con el profesional por un canal seguro: no volverá a mostrarse.
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
