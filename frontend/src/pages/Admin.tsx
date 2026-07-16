import { useState, useEffect, useMemo, Fragment } from 'react'
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
  UserCog,
  X,
  ChevronLeft,
  ChevronRight,
  Mail,
  MessageCircle,
  MessageSquare,
  Archive,
  UploadCloud,
  Download,
  ChevronDown,
  FileSpreadsheet,
} from 'lucide-react'
import Avatar from '../components/Common/Avatar'
import { UserModal } from '../components/Admin/Modals/UserModal'
import { CreateUserModal } from '../components/Admin/Modals/CreateUserModal'
import { ImportUsersModal } from '../components/Admin/Modals/ImportUsersModal'
import { ExportUsersModal } from '../components/Admin/Modals/ExportUsersModal'
import { EmailComposerModal, type ComposerRecipient } from '../components/Admin/EmailComposerModal'
import { Select } from '../components/ui/Select'
import { Skeleton } from '../components/ui'
import { ActivityFeed } from '../components/Admin/ActivityFeed'
import { ArchivedList } from '../components/Admin/ArchivedList'
import { authService, adminService } from '../services/api'
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
  managerId: '' as number | '',
  phoneNumber: '',
  country: '',
  province: '',
  city: '',
  location: '',
  address: '',
  jobTitle: '',
}

// Campo etiqueta/valor para el modal de detalle de ausencia.
function DetailField({ label, value, full }: { label: string; value: string; full?: boolean }) {
  return (
    <div style={full ? { gridColumn: '1 / -1' } : undefined}>
      <div style={{ fontSize: 11, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.04em', color: '#94a3b8', marginBottom: 2 }}>{label}</div>
      <div style={{ color: '#0f172a' }}>{value}</div>
    </div>
  )
}

export default function Admin() {
  const {
    stats,
    users,
    inactiveUsers,
    teamInactivity,
    teamInactivityLoading,
    seniority,
    tenants,
    followUps,
    setFollowUp,
    recentActivity,
    absenceReport,
    isLoading,
    activeTab,
    setActiveTab,
    createUser,
    deleteUser,
    updateUser,
    fetchUsers,
    fetchDashboard,
    fetchCompanies,
  } = useAdmin()

  const navigate = useNavigate()
  const { user: viewer } = useAuth()
  // CS (manager y analista) entran en modo consulta: sin crear/editar/eliminar.
  const canManage = !!viewer?.is_superadmin
  const [searchQuery, setSearchQuery] = useState('')
  const [roleFilter, setRoleFilter] = useState('')
  const [companyFilter, setCompanyFilter] = useState<number | ''>('')
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set())
  const [bulkManagerId, setBulkManagerId] = useState<number | ''>('')
  const [bulkBusy, setBulkBusy] = useState(false)
  const [bulkMsg, setBulkMsg] = useState<string | null>(null)
  const [showBulkDeleteModal, setShowBulkDeleteModal] = useState(false)
  const [bulkDeleteBusy, setBulkDeleteBusy] = useState(false)
  const [bulkDeleteResult, setBulkDeleteResult] = useState<{ deleted: number; skipped: { id: number; name: string; reason: string }[] } | null>(null)
  const [bulkDeleteError, setBulkDeleteError] = useState<string | null>(null)
  const [usersPage, setUsersPage] = useState(1)

  // Actividad de equipo (pestaña Actividad): semáforo de inactividad.
  const [teamSearch, setTeamSearch] = useState('')
  const [teamTier, setTeamTier] = useState<'' | 'yellow' | 'red'>('')
  const [teamPage, setTeamPage] = useState(1)

  // Reporte de ausencias (pestaña Actividad): agrupado por usuario.
  const [absSearch, setAbsSearch] = useState('')
  const [absPage, setAbsPage] = useState(1)
  const [absExpandedUserId, setAbsExpandedUserId] = useState<number | null>(null)
  // Tarjeta "Reporte de ausencias" del dashboard: filtro por empresa, paginación
  // y detalle de cada registro en modal.
  const [rptCompany, setRptCompany] = useState<string>('')
  const [rptReason, setRptReason] = useState<string>('')
  const [rptPage, setRptPage] = useState(1)
  const [rptDetail, setRptDetail] = useState<any | null>(null)
  // Archivados global (todas las empresas): bajas + cuentas desactivadas.
  const [archived, setArchived] = useState<any[]>([])
  const loadArchived = async () => {
    try { setArchived(await adminService.getArchived()) } catch { /* noop */ }
  }
  useEffect(() => { if (activeTab === 'archived') loadArchived() }, [activeTab]) // eslint-disable-line react-hooks/exhaustive-deps

  // Métricas CS (pestaña Dashboard): tabla de antigüedad.
  const [csSearch, setCsSearch] = useState('')
  const [csPage, setCsPage] = useState(1)
  const [showDeleteModal, setShowDeleteModal] = useState(false)
  const [userToDelete, setUserToDelete] = useState<any>(null)
  const [showEditModal, setShowEditModal] = useState(false)
  const [editId, setEditId] = useState<number | null>(null)
  const [editForm, setEditForm] = useState<any>({})

  const [showCreateModal, setShowCreateModal] = useState(false)
  const [showImportModal, setShowImportModal] = useState(false)
  const [showExportModal, setShowExportModal] = useState(false)
  const [showActionsMenu, setShowActionsMenu] = useState(false)
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

  const employers = Array.isArray(users) ? users.filter((u: any) => u.user_type === 'empleador' && ((u.company_name || '').trim() || (u.name || '').trim())) : []
  const managers = Array.isArray(users) ? users.filter((u: any) => u.is_manager) : []

  // Resuelve el nombre de empresa para el export: un empleador es su propia
  // empresa; un profesional/CS toma la del empleador al que está vinculado.
  const companyNameById = useMemo(() => {
    const m = new Map<number, string>()
    ;(Array.isArray(users) ? users : []).forEach((u: any) => {
      if (u.user_type === 'empleador') m.set(u.id, (u.company_name || '').trim() || u.name || '')
    })
    return m
  }, [users])
  const resolveCompanyName = (u: any): string => {
    if (u.user_type === 'empleador') return (u.company_name || '').trim() || u.name || ''
    return (u.company_name || '').trim() || (u.empleador_id ? companyNameById.get(u.empleador_id) || '' : '')
  }

  const openEdit = (u: any) => {
    setEditId(u.id)
    setEditForm({
      name: u.name || '',
      email: u.email || '',
      user_type: u.user_type || '',
      job_title: u.job_title || '',
      phone_number: u.phone_number || '',
      country: u.country || '',
      state: u.state || '',
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
      // Los profesionales/CS desactivados se gestionan desde "Archivados", no en
      // la tabla principal.
      if (u.is_active === false && (u.user_type === 'profesional' || u.user_type === 'customer_success')) return false
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

  const isBulkSelectable = (u: any) => !u.is_superadmin && u.id !== viewer?.id
  const selectableIds: number[] = filteredUsers.filter(isBulkSelectable).map((u: any) => u.id)
  const allSelected = selectableIds.length > 0 && selectableIds.every((id: number) => selectedIds.has(id))
  const bulkManagerOptions = managers.filter((m: any) => companyFilter === '' || m.empleador_id === companyFilter)
  const toggleSelect = (id: number) => setSelectedIds(prev => {
    const next = new Set(prev)
    if (next.has(id)) next.delete(id); else next.add(id)
    return next
  })
  const toggleSelectAll = () => setSelectedIds(prev =>
    selectableIds.every((id: number) => prev.has(id)) ? new Set<number>() : new Set<number>(selectableIds)
  )
  const clearSelection = () => { setSelectedIds(new Set()); setBulkMsg(null) }

  const reportsCountByManager = useMemo(() => {
    const m = new Map<number, number>()
    ;(Array.isArray(users) ? users : []).forEach((u: any) => {
      if (u.manager_id) m.set(u.manager_id, (m.get(u.manager_id) || 0) + 1)
    })
    return m
  }, [users])
  const selectedManagersWithTeam = useMemo(() =>
    (Array.isArray(users) ? users : [])
      .filter((u: any) => selectedIds.has(u.id) && u.is_manager && (reportsCountByManager.get(u.id) || 0) > 0)
      .map((u: any) => ({ id: u.id, name: u.name, count: reportsCountByManager.get(u.id) || 0 }))
      .sort((a: any, b: any) => b.count - a.count)
  , [users, selectedIds, reportsCountByManager])
  const bulkWillDelete = Math.max(0, selectedIds.size - selectedManagersWithTeam.length)

  // Borrar una cuenta empleador se lleva la empresa entera, así que el modal
  // las nombra aparte junto con la gente que quedaría sin empresa.
  const selectedEmployers = useMemo(() => {
    const all = Array.isArray(users) ? users : []
    return all
      .filter((u: any) => selectedIds.has(u.id) && u.user_type === 'empleador')
      .map((u: any) => ({
        id: u.id,
        name: u.company_name?.trim() || u.name,
        linked: all.filter((o: any) => o.empleador_id === u.id).length,
      }))
      .sort((a: any, b: any) => b.linked - a.linked)
  }, [users, selectedIds])

  const handleBulkAssign = async () => {
    if (selectedIds.size === 0) return
    setBulkBusy(true); setBulkMsg(null)
    try {
      const res: any = await adminService.bulkAssignManager(Array.from(selectedIds), bulkManagerId === '' ? null : Number(bulkManagerId))
      const a = res?.assigned ?? 0, s = res?.skipped ?? 0
      setBulkMsg(`Asignados ${a}${s ? ` · Omitidos ${s} (otra empresa o sin empleo activo)` : ''}.`)
      setSelectedIds(new Set())
      await fetchUsers()
    } catch (err: any) {
      setBulkMsg(err?.response?.data?.error ?? 'No se pudo asignar el manager.')
    } finally { setBulkBusy(false) }
  }

  const openBulkDelete = () => {
    if (selectedIds.size === 0) return
    setBulkDeleteResult(null)
    setBulkDeleteError(null)
    setShowBulkDeleteModal(true)
  }
  const handleBulkDelete = async () => {
    if (selectedIds.size === 0) return
    setBulkDeleteBusy(true); setBulkDeleteError(null)
    try {
      const res = await adminService.bulkDeleteUsers(Array.from(selectedIds))
      setBulkDeleteResult({ deleted: res?.deleted ?? 0, skipped: res?.skipped ?? [] })
      setSelectedIds(new Set())
      setBulkMsg(null)
      await fetchUsers()
    } catch (err: any) {
      setBulkDeleteError(err?.response?.data?.error ?? 'No se pudieron eliminar los usuarios.')
    } finally { setBulkDeleteBusy(false) }
  }

  useEffect(() => {
    setUsersPage(1)
  }, [searchQuery, roleFilter, companyFilter])

  // ── Actividad de equipo: filtrado + paginación ──────────────────────────────
  const TEAM_PER_PAGE = 10

  const teamFiltered = teamInactivity
    .filter(u => {
      const q = teamSearch.trim().toLowerCase()
      if (q && !(u.name?.toLowerCase().includes(q) || u.email?.toLowerCase().includes(q) || u.company?.toLowerCase().includes(q))) return false
      if (teamTier === 'yellow' && u.days_inactive !== 1) return false
      if (teamTier === 'red' && u.days_inactive < 2) return false
      return true
    })
    .sort((a, b) => b.days_inactive - a.days_inactive || (a.name || '').localeCompare(b.name || '', 'es', { sensitivity: 'base' }))

  const teamYellowCount = teamInactivity.filter(u => u.days_inactive === 1).length
  const teamRedCount = teamInactivity.filter(u => u.days_inactive >= 2).length
  const teamTotalPages = Math.max(1, Math.ceil(teamFiltered.length / TEAM_PER_PAGE))
  const teamCurrentPage = Math.min(teamPage, teamTotalPages)
  const teamPaginated = teamFiltered.slice((teamCurrentPage - 1) * TEAM_PER_PAGE, teamCurrentPage * TEAM_PER_PAGE)

  useEffect(() => {
    setTeamPage(1)
  }, [teamSearch, teamTier])

  // Mensaje de seguimiento prellenado para las acciones rápidas (email / WhatsApp).
  const followUpMessage = (u: { name: string; days_inactive: number }) =>
    `Hola ${u.name?.split(' ')[0] || ''}, te escribimos del equipo de Obertrack: notamos que no registras horas desde hace ${u.days_inactive} día${u.days_inactive === 1 ? '' : 's'}. ¿Está todo bien? Si tienes algún inconveniente cuéntanos para ayudarte.`

  // Modal de redacción de correo (plantillas de Tools / redacción nueva).
  const [composer, setComposer] = useState<{ recipient: ComposerRecipient; body: string } | null>(null)

  const whatsappHref = (u: { name: string; phone_number?: string; days_inactive: number }) => {
    const digits = (u.phone_number || '').replace(/\D/g, '')
    return digits ? `https://wa.me/${digits}?text=${encodeURIComponent(followUpMessage(u as any))}` : null
  }

  // ── Reporte de ausencias: agrupado por usuario ──────────────────────────────
  const ABS_PER_PAGE = 10

  const absenceGroups = (() => {
    const map = new Map<number, {
      user_id: number; name: string; email: string; phone_number?: string; avatar?: string
      tenant_id: number; company: string; count: number; totalHours: number; pending: number
      lastDate: string; lastReason: string; items: typeof absenceItems
    }>()
    for (const item of absenceItems) {
      const g = map.get(item.user_id) || {
        user_id: item.user_id, name: item.user, email: item.email, phone_number: item.phone_number,
        avatar: item.avatar, tenant_id: item.tenant_id, company: item.company,
        count: 0, totalHours: 0, pending: 0, lastDate: '', lastReason: '', items: [] as typeof absenceItems,
      }
      g.count++
      g.totalHours += item.absence_hours || 0
      if (!item.approved && !item.rejected) g.pending++
      if (!g.lastDate || item.work_date > g.lastDate) {
        g.lastDate = item.work_date
        g.lastReason = item.absence_reason
      }
      g.items.push(item)
      map.set(item.user_id, g)
    }
    return Array.from(map.values()).sort((a, b) => b.count - a.count || (a.name || '').localeCompare(b.name || '', 'es', { sensitivity: 'base' }))
  })()

  const absFiltered = absenceGroups.filter(g => {
    const q = absSearch.trim().toLowerCase()
    return !q || g.name?.toLowerCase().includes(q) || g.email?.toLowerCase().includes(q) || g.company?.toLowerCase().includes(q)
  })
  const absTotalPages = Math.max(1, Math.ceil(absFiltered.length / ABS_PER_PAGE))
  const absCurrentPage = Math.min(absPage, absTotalPages)
  const absPaginated = absFiltered.slice((absCurrentPage - 1) * ABS_PER_PAGE, absCurrentPage * ABS_PER_PAGE)

  useEffect(() => {
    setAbsPage(1)
  }, [absSearch])

  // ── Tarjeta del dashboard: lista plana con filtro por empresa + paginación ──
  const RPT_PER_PAGE = 5
  const rptCompanies = Array.from(new Set(absenceItems.map((i: any) => i.company).filter((c: any) => c && c !== '-' && c !== '—'))).sort() as string[]
  // Conteo de motivos acotado a la empresa seleccionada (los chips reflejan el
  // filtro, no el total global del mes).
  const rptReasonCounts = (() => {
    const map = new Map<string, number>()
    absenceItems
      .filter((i: any) => !rptCompany || i.company === rptCompany)
      .forEach((i: any) => {
        const r = i.absence_reason || 'Sin motivo'
        map.set(r, (map.get(r) || 0) + 1)
      })
    return Array.from(map.entries())
      .map(([reason, count]) => ({ reason, count }))
      .sort((a, b) => b.count - a.count || a.reason.localeCompare(b.reason, 'es'))
  })()
  const rptFiltered = absenceItems
    .filter((i: any) => !rptCompany || i.company === rptCompany)
    .filter((i: any) => !rptReason || (i.absence_reason || 'Sin motivo') === rptReason)
    .sort((a: any, b: any) => (b.work_date || '').localeCompare(a.work_date || ''))
  const rptTotalPages = Math.max(1, Math.ceil(rptFiltered.length / RPT_PER_PAGE))
  const rptCurrentPage = Math.min(rptPage, rptTotalPages)
  const rptPaginated = rptFiltered.slice((rptCurrentPage - 1) * RPT_PER_PAGE, rptCurrentPage * RPT_PER_PAGE)

  useEffect(() => {
    setRptPage(1)
  }, [rptCompany, rptReason])

  const absenceFollowUp = (g: { name: string; count: number }) =>
    `Hola ${g.name?.split(' ')[0] || ''}, te escribimos del equipo de Obertrack por el seguimiento de tus ${g.count} ausencia${g.count === 1 ? '' : 's'} registrada${g.count === 1 ? '' : 's'} este mes. ¿Está todo bien? Si necesitas apoyo, cuéntanos.`

  const absEmailHref = (g: { name: string; email: string; count: number }) =>
    `mailto:${g.email}?subject=${encodeURIComponent('Seguimiento de ausencias en Obertrack')}&body=${encodeURIComponent(absenceFollowUp(g))}`

  const absWhatsappHref = (g: { name: string; phone_number?: string; count: number }) => {
    const digits = (g.phone_number || '').replace(/\D/g, '')
    return digits ? `https://wa.me/${digits}?text=${encodeURIComponent(absenceFollowUp(g))}` : null
  }

  // ── Métricas de Customer Success (dashboard) ───────────────────────────────
  const CS_PER_PAGE = 10

  const absenceReasons = absenceReport?.reasons || []
  const reasonMax = Math.max(1, ...absenceReasons.map(r => r.count))
  const topAbsentees = absenceGroups.slice(0, 10)
  const tenantsPending = [...tenants]
    .filter((t: any) => (t.pending_count || 0) > 0)
    .sort((a: any, b: any) => (b.pending_count || 0) - (a.pending_count || 0))
    .slice(0, 5)
  const tenantsRejecting = [...tenants]
    .filter((t: any) => (t.rejected_count || 0) > 0)
    .sort((a: any, b: any) => (b.rejected_count || 0) - (a.rejected_count || 0))
    .slice(0, 5)

  const seniorityFiltered = seniority.filter(s => {
    const q = csSearch.trim().toLowerCase()
    return !q || s.name?.toLowerCase().includes(q) || s.email?.toLowerCase().includes(q) || s.company?.toLowerCase().includes(q)
  })
  const csTotalPages = Math.max(1, Math.ceil(seniorityFiltered.length / CS_PER_PAGE))
  const csCurrentPage = Math.min(csPage, csTotalPages)
  const seniorityPaginated = seniorityFiltered.slice((csCurrentPage - 1) * CS_PER_PAGE, csCurrentPage * CS_PER_PAGE)

  useEffect(() => {
    setCsPage(1)
  }, [csSearch])

  const seniorityLabel = (days: number) => {
    if (days >= 365) {
      const years = Math.floor(days / 365)
      const months = Math.floor((days % 365) / 30)
      return `${years} año${years === 1 ? '' : 's'}${months > 0 ? ` y ${months} mes${months === 1 ? '' : 'es'}` : ''}`
    }
    if (days >= 30) {
      const months = Math.floor(days / 30)
      return `${months} mes${months === 1 ? '' : 'es'}`
    }
    return `${days} día${days === 1 ? '' : 's'}`
  }

  const csCardStyle: React.CSSProperties = { background: '#fff', border: '1px solid #e2e8f0', borderRadius: '16px', padding: '18px' }
  const csCardTitleStyle: React.CSSProperties = { margin: '0 0 12px', fontSize: '14px', fontWeight: 800, color: '#0f172a' }

  // ── Bitácora de gestión CS: celda compartida (inactividad / ausencias) ─────
  const FOLLOWUP_STYLES: Record<string, { bg: string; color: string }> = {
    contacted: { bg: 'rgba(59,130,246,0.12)', color: '#1d4ed8' },
    justified: { bg: 'rgba(16,185,129,0.12)', color: '#047857' },
    escalated: { bg: 'rgba(168,85,247,0.14)', color: '#7e22ce' },
  }

  const renderFollowUpCell = (userId: number, kind: 'inactivity' | 'absence') => {
    const info = followUps[kind][userId]
    const palette = info ? FOLLOWUP_STYLES[info.status] : undefined
    return (
      <div onClick={(e) => e.stopPropagation()}>
        <select
          value={info?.status || ''}
          onChange={(e) => { if (e.target.value) setFollowUp(userId, kind, e.target.value) }}
          title="Estado de gestión del seguimiento"
          style={{ padding: '5px 8px', borderRadius: '8px', border: '1px solid #e2e8f0', fontSize: '12px', fontWeight: 700, background: palette?.bg || '#f8fafc', color: palette?.color || '#64748b', cursor: 'pointer' }}
        >
          {!info && <option value="">— Gestionar —</option>}
          <option value="contacted">📞 Contactado</option>
          <option value="justified">✅ Justificado</option>
          <option value="escalated">⚠️ Escalado</option>
        </select>
        {info && (
          <small style={{ display: 'block', color: '#94a3b8', marginTop: '3px' }}>
            por {info.by_name} · {new Date(info.created_at).toLocaleDateString('es-ES')}
          </small>
        )}
      </div>
    )
  }

  const tabs = [
    { id: 'dashboard', label: 'Dashboard', icon: BarChart3 },
    { id: 'users', label: 'Usuarios', icon: Users },
    { id: 'activity', label: 'Actividad', icon: Activity },
    { id: 'archived', label: 'Archivados', icon: Archive },
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
                <div className={styles['stat-icon']} style={{ background: '#faf5ff', color: 'var(--primary)' }}>
                  <Users size={26} />
                </div>
                <div className={styles['stat-info']}>
                  <span className={styles['stat-value']}>{stats?.totalUsers || 0}</span>
                  <span className={styles['stat-label']}>Total Usuarios</span>
                </div>
              </div>
              <div className={styles['stat-card']}>
                <div className={styles['stat-icon']} style={{ background: '#ecfdf5', color: '#10b981' }}>
                  <Users size={26} />
                </div>
                <div className={styles['stat-info']}>
                  <span className={styles['stat-value']}>{stats?.activeUsers || 0}</span>
                  <span className={styles['stat-label']}>Usuarios Activos</span>
                </div>
              </div>
              <div className={styles['stat-card']}>
                <div className={styles['stat-icon']} style={{ background: '#fffbeb', color: '#f59e0b' }}>
                  <Building2 size={26} />
                </div>
                <div className={styles['stat-info']}>
                  <span className={styles['stat-value']}>{stats?.totalCompanies || 0}</span>
                  <span className={styles['stat-label']}>Empresas registradas</span>
                </div>
              </div>
              <div className={styles['stat-card']}>
                <div className={styles['stat-icon']} style={{ background: '#f5f3ff', color: '#8b5cf6' }}>
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

                {rptReasonCounts.length ? (
                  <div className={styles['reason-cloud']}>
                    {rptReasonCounts.map((reason: any) => {
                      const active = rptReason === reason.reason
                      return (
                        <span
                          key={reason.reason}
                          onClick={() => setRptReason(active ? '' : reason.reason)}
                          title={active ? 'Quitar filtro' : `Filtrar por ${reason.reason}`}
                          style={{
                            cursor: 'pointer',
                            ...(active ? { background: '#ede9fe', borderColor: '#8b5cf6', color: '#6d28d9' } : {}),
                          }}
                        >
                          {reason.reason} ({reason.count})
                        </span>
                      )
                    })}
                  </div>
                ) : null}

                {rptCompanies.length > 1 && (
                  <div style={{ margin: '4px 0 12px', maxWidth: 240 }}>
                    <Select
                      value={rptCompany}
                      onChange={(v) => setRptCompany(String(v))}
                      options={[{ value: '', label: 'Todas las empresas' }, ...rptCompanies.map(c => ({ value: c, label: c }))]}
                    />
                  </div>
                )}

                <div className={styles['absence-list']}>
                  {rptFiltered.length === 0 ? (
                    <p className={styles['empty-message']}>
                      {(rptCompany || rptReason) ? 'Sin ausencias con los filtros aplicados' : 'No hay ausencias registradas este mes'}
                    </p>
                  ) : (
                    rptPaginated.map((item: any) => {
                      const status = getAbsenceStatus(item)
                      return (
                        <div
                          key={item.id}
                          className={styles['absence-row']}
                          onClick={() => setRptDetail(item)}
                          style={{ cursor: 'pointer' }}
                          title="Ver detalle del registro"
                        >
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

                {rptTotalPages > 1 && (
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginTop: 12 }}>
                    <span style={{ fontSize: 12, color: '#64748b' }}>
                      {rptFiltered.length} registro{rptFiltered.length === 1 ? '' : 's'} · página {rptCurrentPage} de {rptTotalPages}
                    </span>
                    <div style={{ display: 'flex', gap: 6 }}>
                      <button type="button" className={styles['btn-icon']} onClick={() => setRptPage(p => Math.max(1, p - 1))} disabled={rptCurrentPage <= 1} style={{ opacity: rptCurrentPage <= 1 ? 0.4 : 1 }} title="Página anterior">
                        <ChevronLeft size={16} />
                      </button>
                      <button type="button" className={styles['btn-icon']} onClick={() => setRptPage(p => Math.min(rptTotalPages, p + 1))} disabled={rptCurrentPage >= rptTotalPages} style={{ opacity: rptCurrentPage >= rptTotalPages ? 0.4 : 1 }} title="Página siguiente">
                        <ChevronRight size={16} />
                      </button>
                    </div>
                  </div>
                )}
              </section>
            </div>

            {/* ── Métricas de Customer Success ── */}
            <div style={{ margin: '28px 0' }} data-tour="admin-cs-metrics">
              <h3 style={{ margin: '0 0 4px', fontSize: '17px', fontWeight: 800, color: '#0f172a' }}>Métricas de Customer Success</h3>
              <p style={{ margin: '0 0 16px', fontSize: '13px', color: '#64748b' }}>Indicadores de la operación en Obertrack: ausencias, antigüedad y salud de aprobación por cliente.</p>

              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: '16px', marginBottom: '20px' }}>
                {/* Motivos de ausencia más recurrentes */}
                <div style={csCardStyle}>
                  <h4 style={csCardTitleStyle}>📋 Ausencias más recurrentes</h4>
                  {absenceReasons.length === 0 ? (
                    <p style={{ fontSize: '13px', color: '#94a3b8', margin: 0 }}>Sin ausencias este mes</p>
                  ) : (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                      {absenceReasons.map(r => (
                        <div key={r.reason}>
                          <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '13px', marginBottom: '3px' }}>
                            <span style={{ color: '#334155', fontWeight: 600 }}>{r.reason}</span>
                            <span style={{ color: '#64748b' }}>{r.count}</span>
                          </div>
                          <div style={{ height: 6, borderRadius: 999, background: '#f1f5f9' }}>
                            <div style={{ height: 6, borderRadius: 999, width: `${Math.round((r.count / reasonMax) * 100)}%`, background: 'linear-gradient(90deg, #f59e0b, #ef4444)' }} />
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>

                {/* Top ausentistas del mes */}
                <div style={csCardStyle}>
                  <h4 style={csCardTitleStyle}>🏆 Top ausencias del mes</h4>
                  {topAbsentees.length === 0 ? (
                    <p style={{ fontSize: '13px', color: '#94a3b8', margin: 0 }}>Sin ausencias este mes</p>
                  ) : (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                      {topAbsentees.map((g, i) => (
                        <div key={g.user_id} style={{ display: 'flex', alignItems: 'center', gap: '10px', cursor: 'pointer' }} onClick={() => navigate(`/admin/users/${g.user_id}`)} title="Ver detalle">
                          <span style={{ width: 18, fontSize: '12px', fontWeight: 800, color: '#94a3b8' }}>{i + 1}</span>
                          <Avatar src={g.avatar} name={g.name} size="sm" />
                          <div style={{ flex: 1, minWidth: 0 }}>
                            <span style={{ display: 'block', fontSize: '13px', fontWeight: 600, color: '#0f172a', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{g.name}</span>
                            <small style={{ color: '#94a3b8' }}>{g.company}</small>
                          </div>
                          <span style={{ padding: '2px 10px', borderRadius: 999, fontSize: '12px', fontWeight: 700, background: 'rgba(239,68,68,0.1)', color: '#b91c1c' }}>{g.count}</span>
                        </div>
                      ))}
                    </div>
                  )}
                </div>

                {/* Clientes sin aprobar horas */}
                <div style={csCardStyle}>
                  <h4 style={csCardTitleStyle}>⏳ Clientes sin aprobar horas</h4>
                  {tenantsPending.length === 0 ? (
                    <p style={{ fontSize: '13px', color: '#94a3b8', margin: 0 }}>Todos los clientes están al día 🎉</p>
                  ) : (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                      {tenantsPending.map((t: any) => (
                        <div key={t.id} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '10px', cursor: 'pointer' }} onClick={() => navigate(`/admin/tenants/${t.id}`)} title="Ver empresa">
                          <span style={{ fontSize: '13px', fontWeight: 600, color: '#0f172a' }}>{t.company_name}</span>
                          <span style={{ padding: '2px 10px', borderRadius: 999, fontSize: '12px', fontWeight: 700, background: 'rgba(245,158,11,0.14)', color: '#b45309', whiteSpace: 'nowrap' }}>
                            {t.pending_count} reg. · {(t.pending_hours || 0).toFixed(1)}h
                          </span>
                        </div>
                      ))}
                    </div>
                  )}
                </div>

                {/* Clientes que rechazan registros */}
                <div style={csCardStyle}>
                  <h4 style={csCardTitleStyle}>🚫 Clientes que rechazan registros</h4>
                  {tenantsRejecting.length === 0 ? (
                    <p style={{ fontSize: '13px', color: '#94a3b8', margin: 0 }}>Sin rechazos registrados</p>
                  ) : (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                      {tenantsRejecting.map((t: any) => (
                        <div key={t.id} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '10px', cursor: 'pointer' }} onClick={() => navigate(`/admin/tenants/${t.id}`)} title="Ver empresa">
                          <span style={{ fontSize: '13px', fontWeight: 600, color: '#0f172a' }}>{t.company_name}</span>
                          <span style={{ padding: '2px 10px', borderRadius: 999, fontSize: '12px', fontWeight: 700, background: 'rgba(239,68,68,0.12)', color: '#b91c1c', whiteSpace: 'nowrap' }}>
                            {t.rejected_count} rechazado{t.rejected_count === 1 ? '' : 's'}
                          </span>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>

              {/* Top antigüedad */}
              <div style={csCardStyle}>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', marginBottom: '12px' }}>
                  <h4 style={{ ...csCardTitleStyle, margin: 0 }}>🎖️ Antigüedad de profesionales</h4>
                  <div className={styles['search-box']} style={{ margin: 0 }}>
                    <Search size={16} />
                    <input
                      type="text"
                      placeholder="Buscar por nombre, correo o empresa..."
                      value={csSearch}
                      onChange={(e) => setCsSearch(e.target.value)}
                    />
                  </div>
                </div>

                {seniorityFiltered.length === 0 ? (
                  <p style={{ fontSize: '13px', color: '#94a3b8', margin: 0 }}>
                    {seniority.length === 0 ? 'Sin profesionales registrados' : 'Sin resultados con la búsqueda'}
                  </p>
                ) : (
                  <>
                    <div className={styles['users-table']}>
                      <table>
                        <thead>
                          <tr>
                            <th>#</th>
                            <th>Profesional</th>
                            <th>Empresa</th>
                            <th>Desde</th>
                            <th>Antigüedad</th>
                          </tr>
                        </thead>
                        <tbody>
                          {seniorityPaginated.map((s, idx) => (
                            <tr key={s.user_id} style={{ cursor: 'pointer' }} onClick={() => navigate(`/admin/users/${s.user_id}`)} title="Ver detalle del profesional">
                              <td style={{ fontWeight: 800, color: '#94a3b8' }}>{(csCurrentPage - 1) * CS_PER_PAGE + idx + 1}</td>
                              <td>
                                <div className={styles['user-cell']}>
                                  <Avatar src={s.avatar} name={s.name} size="sm" />
                                  <div>
                                    <span style={{ display: 'block' }}>{s.name}</span>
                                    <small style={{ color: '#94a3b8' }}>{s.email}{s.job_title ? ` · ${s.job_title}` : ''}</small>
                                  </div>
                                </div>
                              </td>
                              <td>
                                {s.tenant_id > 0 ? (
                                  <button type="button" onClick={(e) => { e.stopPropagation(); navigate(`/admin/tenants/${s.tenant_id}`) }} style={{ background: 'none', border: 'none', padding: 0, color: '#5a52e6', fontWeight: 600, cursor: 'pointer', textDecoration: 'underline', fontSize: 'inherit' }}>
                                    {s.company}
                                  </button>
                                ) : (s.company || '—')}
                              </td>
                              <td>{new Date(s.started_at).toLocaleDateString('es-ES')}</td>
                              <td>
                                <span style={{ padding: '3px 10px', borderRadius: 999, fontSize: '12px', fontWeight: 700, background: 'rgba(90,82,230,0.1)', color: '#5a52e6' }}>
                                  {seniorityLabel(s.days_employed)}
                                </span>
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>

                    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', paddingTop: '12px' }}>
                      <span style={{ fontSize: '13px', color: '#64748b' }}>
                        Mostrando {(csCurrentPage - 1) * CS_PER_PAGE + 1}–{Math.min(csCurrentPage * CS_PER_PAGE, seniorityFiltered.length)} de {seniorityFiltered.length} profesionales
                      </span>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                        <button type="button" className={styles['btn-icon']} onClick={() => setCsPage(p => Math.max(1, p - 1))} disabled={csCurrentPage <= 1} style={{ opacity: csCurrentPage <= 1 ? 0.4 : 1 }} title="Página anterior">
                          <ChevronLeft size={16} />
                        </button>
                        <span style={{ fontSize: '13px', fontWeight: 600, color: '#334155', whiteSpace: 'nowrap' }}>
                          Página {csCurrentPage} de {csTotalPages}
                        </span>
                        <button type="button" className={styles['btn-icon']} onClick={() => setCsPage(p => Math.min(csTotalPages, p + 1))} disabled={csCurrentPage >= csTotalPages} style={{ opacity: csCurrentPage >= csTotalPages ? 0.4 : 1 }} title="Página siguiente">
                          <ChevronRight size={16} />
                        </button>
                      </div>
                    </div>
                  </>
                )}
              </div>
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
              <div style={{ position: 'relative' }}>
                <button
                  type="button"
                  onClick={() => setShowActionsMenu(o => !o)}
                  style={{
                    display: 'inline-flex',
                    alignItems: 'center',
                    gap: '8px',
                    padding: '10px 16px',
                    background: '#fff',
                    color: '#6d28d9',
                    border: '1px solid #ddd6fe',
                    borderRadius: '12px',
                    fontWeight: 700,
                    fontSize: '14px',
                    cursor: 'pointer',
                    whiteSpace: 'nowrap',
                  }}
                  title="Importar o exportar usuarios"
                >
                  <FileSpreadsheet size={16} /> Acciones
                  <ChevronDown size={15} style={{ transform: showActionsMenu ? 'rotate(180deg)' : 'none', transition: 'transform 0.15s' }} />
                </button>
                {showActionsMenu && (
                  <>
                    <div style={{ position: 'fixed', inset: 0, zIndex: 20 }} onClick={() => setShowActionsMenu(false)} />
                    <div style={{ position: 'absolute', top: 'calc(100% + 6px)', right: 0, zIndex: 21, background: '#fff', border: '1px solid #e2e8f0', borderRadius: '12px', boxShadow: '0 12px 32px rgba(0,0,0,0.12)', padding: '6px', minWidth: '200px' }}>
                      <button type="button" onClick={() => { setShowActionsMenu(false); setShowImportModal(true) }} style={actionMenuItem}>
                        <UploadCloud size={16} /> Importar desde Excel
                      </button>
                      <button type="button" onClick={() => { setShowActionsMenu(false); setShowExportModal(true) }} style={actionMenuItem}>
                        <Download size={16} /> Exportar a Excel
                      </button>
                    </div>
                  </>
                )}
              </div>
              )}
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

            {canManage && (selectedIds.size > 0 || bulkMsg) && (
              <div style={{ display: 'flex', alignItems: 'center', gap: '12px', flexWrap: 'wrap', padding: '10px 14px', marginBottom: '12px', background: 'rgba(124,58,237,0.06)', border: '1px solid #e9d5ff', borderRadius: '12px' }}>
                {selectedIds.size > 0 ? (
                  <>
                    <span style={{ fontWeight: 700, color: '#6d28d9', whiteSpace: 'nowrap' }}>{selectedIds.size} usuario(s) seleccionado(s)</span>
                    <select
                      value={bulkManagerId}
                      onChange={e => setBulkManagerId(e.target.value === '' ? '' : Number(e.target.value))}
                      disabled={bulkBusy || bulkDeleteBusy}
                      style={{ padding: '8px 10px', border: '1px solid #cbd5e1', borderRadius: '10px', fontSize: '14px', minWidth: 200, background: '#fff', color: '#334155' }}
                    >
                      <option value="">Sin manager (desasignar)</option>
                      {bulkManagerOptions.map((m: any) => (
                        <option key={m.id} value={m.id}>{m.name}</option>
                      ))}
                    </select>
                    <button
                      onClick={handleBulkAssign}
                      disabled={bulkBusy || bulkDeleteBusy}
                      style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '9px 16px', borderRadius: '10px', border: 'none', background: 'var(--primary, #7c3aed)', color: '#fff', fontWeight: 700, fontSize: '13px', cursor: bulkBusy ? 'progress' : 'pointer' }}
                    >
                      <UserCog size={15} /> Asignar ({selectedIds.size})
                    </button>
                    <button
                      onClick={openBulkDelete}
                      disabled={bulkBusy || bulkDeleteBusy}
                      style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '9px 16px', borderRadius: '10px', border: 'none', background: '#dc2626', color: '#fff', fontWeight: 700, fontSize: '13px', cursor: (bulkBusy || bulkDeleteBusy) ? 'progress' : 'pointer' }}
                    >
                      <Trash2 size={15} /> Eliminar ({selectedIds.size})
                    </button>
                    <button
                      onClick={clearSelection}
                      disabled={bulkBusy || bulkDeleteBusy}
                      style={{ padding: '9px 14px', border: '1px solid #e2e8f0', borderRadius: '10px', background: 'transparent', color: '#64748b', fontSize: '13px', fontWeight: 600, cursor: 'pointer' }}
                    >
                      Limpiar
                    </button>
                  </>
                ) : null}
                {bulkMsg && <span style={{ fontSize: '13px', color: '#475569', fontWeight: 600 }}>{bulkMsg}</span>}
              </div>
            )}

            <div className={styles['users-table']} data-tour="admin-users-table">
              <table>
                <thead>
                  <tr>
                    {canManage && (
                      <th style={{ width: 36 }}>
                        <input type="checkbox" checked={allSelected} onChange={toggleSelectAll} title="Seleccionar todos los usuarios filtrados" style={{ cursor: 'pointer' }} />
                      </th>
                    )}
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
                      <td colSpan={canManage ? 6 : 5} style={{ textAlign: 'center', padding: '32px 16px', color: '#94a3b8' }}>
                        No se encontraron usuarios con los filtros aplicados.
                      </td>
                    </tr>
                  )}
                  {paginatedUsers.map((u: any, index: number) => (
                    <tr key={u.id || `user-${index}`}>
                      {canManage && (
                        <td style={{ width: 36 }}>
                          {isBulkSelectable(u) && (
                            <input type="checkbox" checked={selectedIds.has(u.id)} onChange={() => toggleSelect(u.id)} style={{ cursor: 'pointer' }} />
                          )}
                        </td>
                      )}
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
            {/* ── Actividad de equipo: semáforo de inactividad ── */}
            <div style={{ marginBottom: '32px' }} data-tour="admin-team-activity">
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', marginBottom: '14px' }}>
                <div>
                  <h3 style={{ margin: 0, fontSize: '17px', fontWeight: 800, color: '#0f172a' }}>Actividad de equipo</h3>
                  <p style={{ margin: '2px 0 0', fontSize: '13px', color: '#64748b' }}>
                    Profesionales sin registrar horas. Los de 2+ días disparan alerta automática al equipo de customer success.
                  </p>
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap' }}>
                  <button
                    type="button"
                    onClick={() => setTeamTier(teamTier === 'yellow' ? '' : 'yellow')}
                    style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '6px 12px', borderRadius: '999px', border: teamTier === 'yellow' ? '1px solid #f59e0b' : '1px solid #e2e8f0', background: 'rgba(245,158,11,0.1)', color: '#b45309', fontSize: '13px', fontWeight: 700, cursor: 'pointer' }}
                  >
                    🟡 1 día: {teamYellowCount}
                  </button>
                  <button
                    type="button"
                    onClick={() => setTeamTier(teamTier === 'red' ? '' : 'red')}
                    style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '6px 12px', borderRadius: '999px', border: teamTier === 'red' ? '1px solid #ef4444' : '1px solid #e2e8f0', background: 'rgba(239,68,68,0.1)', color: '#b91c1c', fontSize: '13px', fontWeight: 700, cursor: 'pointer' }}
                  >
                    🔴 2+ días: {teamRedCount}
                  </button>
                  <div className={styles['search-box']}>
                    <Search size={16} />
                    <input
                      type="text"
                      placeholder="Buscar por nombre, correo o empresa..."
                      value={teamSearch}
                      onChange={(e) => setTeamSearch(e.target.value)}
                    />
                  </div>
                </div>
              </div>

              {teamInactivityLoading ? (
                <Skeleton height={180} radius={12} />
              ) : teamFiltered.length === 0 ? (
                <div className={styles['empty-state']} style={{ padding: '28px' }}>
                  <CheckCircle2 size={34} />
                  <p>{teamInactivity.length === 0 ? 'Todo el equipo está registrando horas 🎉' : 'Sin resultados con los filtros aplicados'}</p>
                </div>
              ) : (
                <div className={styles['users-table']}>
                  <table>
                    <thead>
                      <tr>
                        <th>Profesional</th>
                        <th>Empresa</th>
                        <th>Últ. actividad</th>
                        <th>Inactividad</th>
                        <th>Gestión</th>
                        <th>Acciones</th>
                      </tr>
                    </thead>
                    <tbody>
                      {teamPaginated.map(u => {
                        const isRed = u.days_inactive >= 2
                        const waLink = whatsappHref(u)
                        return (
                          <tr key={u.id} style={{ background: isRed ? 'rgba(239,68,68,0.07)' : 'rgba(245,158,11,0.07)' }}>
                            <td>
                              <div className={styles['user-cell']} style={{ cursor: 'pointer' }} onClick={() => navigate(`/admin/users/${u.id}`)} title="Ver detalle del profesional">
                                <Avatar src={u.avatar} name={u.name} size="sm" />
                                <div>
                                  <span style={{ display: 'block' }}>{u.name}</span>
                                  <small style={{ color: '#94a3b8' }}>{u.email}{u.job_title ? ` · ${u.job_title}` : ''}</small>
                                </div>
                              </div>
                            </td>
                            <td>
                              {u.tenant_id > 0 ? (
                                <button
                                  type="button"
                                  onClick={() => navigate(`/admin/tenants/${u.tenant_id}`)}
                                  style={{ background: 'none', border: 'none', padding: 0, color: '#5a52e6', fontWeight: 600, cursor: 'pointer', textDecoration: 'underline', fontSize: 'inherit' }}
                                  title="Ver detalle de la empresa"
                                >
                                  {u.company}
                                </button>
                              ) : (u.company || '—')}
                            </td>
                            <td>{u.last_active ? new Date(u.last_active).toLocaleDateString('es-ES') : '—'}</td>
                            <td>
                              <span style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '4px 10px', borderRadius: '999px', fontSize: '12px', fontWeight: 700, background: isRed ? 'rgba(239,68,68,0.12)' : 'rgba(245,158,11,0.14)', color: isRed ? '#b91c1c' : '#b45309' }}>
                                {isRed ? '🔴' : '🟡'} {u.days_inactive} día{u.days_inactive === 1 ? '' : 's'} háb.
                              </span>
                            </td>
                            <td>{renderFollowUpCell(u.id, 'inactivity')}</td>
                            <td>
                              <div className={styles['action-buttons']}>
                                <button
                                  type="button"
                                  onClick={() => setComposer({ recipient: { id: u.id, name: u.name, email: u.email }, body: followUpMessage(u) })}
                                  className={styles['btn-icon']}
                                  title={`Enviar email a ${u.email}`}
                                  style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center' }}
                                >
                                  <Mail size={16} />
                                </button>
                                {waLink ? (
                                  <a
                                    href={waLink}
                                    target="_blank"
                                    rel="noopener noreferrer"
                                    onClick={() => adminService.logContact(u.id, 'whatsapp')}
                                    className={styles['btn-icon']}
                                    title={`Escribir por WhatsApp (${u.phone_number})`}
                                    style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', color: '#16a34a' }}
                                  >
                                    <MessageCircle size={16} />
                                  </a>
                                ) : (
                                  <span className={styles['btn-icon']} title="Sin teléfono registrado" style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', opacity: 0.35, cursor: 'not-allowed' }}>
                                    <MessageCircle size={16} />
                                  </span>
                                )}
                                <button
                                  className={styles['btn-icon']}
                                  onClick={() => { adminService.logContact(u.id, 'chat'); navigate(`/chat?userId=${u.id}`) }}
                                  title="Chat interno"
                                  style={{ color: '#7c3aed' }}
                                >
                                  <MessageSquare size={16} />
                                </button>
                                <button
                                  className={styles['btn-icon']}
                                  onClick={() => navigate(`/admin/users/${u.id}`)}
                                  title="Ver detalle"
                                >
                                  <Eye size={16} />
                                </button>
                              </div>
                            </td>
                          </tr>
                        )
                      })}
                    </tbody>
                  </table>

                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', padding: '12px 16px' }}>
                    <span style={{ fontSize: '13px', color: '#64748b' }}>
                      Mostrando {(teamCurrentPage - 1) * TEAM_PER_PAGE + 1}–{Math.min(teamCurrentPage * TEAM_PER_PAGE, teamFiltered.length)} de {teamFiltered.length} profesionales
                    </span>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <button type="button" className={styles['btn-icon']} onClick={() => setTeamPage(p => Math.max(1, p - 1))} disabled={teamCurrentPage <= 1} style={{ opacity: teamCurrentPage <= 1 ? 0.4 : 1 }} title="Página anterior">
                        <ChevronLeft size={16} />
                      </button>
                      <span style={{ fontSize: '13px', fontWeight: 600, color: '#334155', whiteSpace: 'nowrap' }}>
                        Página {teamCurrentPage} de {teamTotalPages}
                      </span>
                      <button type="button" className={styles['btn-icon']} onClick={() => setTeamPage(p => Math.min(teamTotalPages, p + 1))} disabled={teamCurrentPage >= teamTotalPages} style={{ opacity: teamCurrentPage >= teamTotalPages ? 0.4 : 1 }} title="Página siguiente">
                        <ChevronRight size={16} />
                      </button>
                    </div>
                  </div>
                </div>
              )}
            </div>

            {/* ── Reporte de ausencias: agrupado por usuario ── */}
            <div style={{ marginBottom: '32px' }} data-tour="admin-absence-report">
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', marginBottom: '14px' }}>
                <div>
                  <h3 style={{ margin: 0, fontSize: '17px', fontWeight: 800, color: '#0f172a' }}>Reporte de ausencias</h3>
                  <p style={{ margin: '2px 0 0', fontSize: '13px', color: '#64748b' }}>
                    Ausencias del mes agrupadas por profesional. Haz clic en una fila para ver el detalle.
                  </p>
                </div>
                <div className={styles['search-box']}>
                  <Search size={16} />
                  <input
                    type="text"
                    placeholder="Buscar por nombre, correo o empresa..."
                    value={absSearch}
                    onChange={(e) => setAbsSearch(e.target.value)}
                  />
                </div>
              </div>

              {absFiltered.length === 0 ? (
                <div className={styles['empty-state']} style={{ padding: '28px' }}>
                  <CalendarX size={34} />
                  <p>{absenceItems.length === 0 ? 'Sin ausencias registradas este mes' : 'Sin resultados con los filtros aplicados'}</p>
                </div>
              ) : (
                <div className={styles['users-table']}>
                  <table>
                    <thead>
                      <tr>
                        <th>Profesional</th>
                        <th>Empresa</th>
                        <th>Ausencias</th>
                        <th>Horas</th>
                        <th>Última ausencia</th>
                        <th>Gestión</th>
                        <th>Acciones</th>
                      </tr>
                    </thead>
                    <tbody>
                      {absPaginated.map(g => {
                        const expanded = absExpandedUserId === g.user_id
                        const waLink = absWhatsappHref(g)
                        return (
                          <Fragment key={g.user_id}>
                            <tr
                              onClick={() => setAbsExpandedUserId(expanded ? null : g.user_id)}
                              style={{ cursor: 'pointer', background: expanded ? 'rgba(204,51,204,0.05)' : undefined }}
                              title={expanded ? 'Ocultar detalle' : 'Ver detalle de las ausencias'}
                            >
                              <td>
                                <div className={styles['user-cell']}>
                                  <Avatar src={g.avatar} name={g.name} size="sm" />
                                  <div>
                                    <span style={{ display: 'block' }}>{g.name}</span>
                                    <small style={{ color: '#94a3b8' }}>{g.email}</small>
                                  </div>
                                </div>
                              </td>
                              <td>
                                {g.tenant_id > 0 ? (
                                  <button
                                    type="button"
                                    onClick={(e) => { e.stopPropagation(); navigate(`/admin/tenants/${g.tenant_id}`) }}
                                    style={{ background: 'none', border: 'none', padding: 0, color: '#5a52e6', fontWeight: 600, cursor: 'pointer', textDecoration: 'underline', fontSize: 'inherit' }}
                                  >
                                    {g.company}
                                  </button>
                                ) : (g.company || '—')}
                              </td>
                              <td>
                                <span style={{ fontWeight: 700 }}>{g.count}</span>
                                {g.pending > 0 && (
                                  <span style={{ marginLeft: 8, padding: '2px 8px', borderRadius: '999px', fontSize: '11px', fontWeight: 700, background: 'rgba(245,158,11,0.14)', color: '#b45309' }}>
                                    {g.pending} pendiente{g.pending === 1 ? '' : 's'}
                                  </span>
                                )}
                              </td>
                              <td>{g.totalHours.toFixed(1)} h</td>
                              <td>
                                {g.lastDate ? new Date(g.lastDate).toLocaleDateString('es-ES') : '—'}
                                <small style={{ display: 'block', color: '#94a3b8' }}>{g.lastReason}</small>
                              </td>
                              <td>{renderFollowUpCell(g.user_id, 'absence')}</td>
                              <td>
                                <div className={styles['action-buttons']} onClick={(e) => e.stopPropagation()}>
                                  <a href={absEmailHref(g)} onClick={() => adminService.logContact(g.user_id, 'email')} className={styles['btn-icon']} title={`Enviar email a ${g.email}`} style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center' }}>
                                    <Mail size={16} />
                                  </a>
                                  {waLink ? (
                                    <a href={waLink} target="_blank" rel="noopener noreferrer" onClick={() => adminService.logContact(g.user_id, 'whatsapp')} className={styles['btn-icon']} title={`Escribir por WhatsApp (${g.phone_number})`} style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', color: '#16a34a' }}>
                                      <MessageCircle size={16} />
                                    </a>
                                  ) : (
                                    <span className={styles['btn-icon']} title="Sin teléfono registrado" style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', opacity: 0.35, cursor: 'not-allowed' }}>
                                      <MessageCircle size={16} />
                                    </span>
                                  )}
                                  <button className={styles['btn-icon']} onClick={() => { adminService.logContact(g.user_id, 'chat'); navigate(`/chat?userId=${g.user_id}`) }} title="Chat interno" style={{ color: '#7c3aed' }}>
                                    <MessageSquare size={16} />
                                  </button>
                                  <button className={styles['btn-icon']} onClick={() => navigate(`/admin/users/${g.user_id}`)} title="Ver detalle del profesional">
                                    <Eye size={16} />
                                  </button>
                                </div>
                              </td>
                            </tr>
                            {expanded && (
                              <tr>
                                <td colSpan={7} style={{ background: 'rgba(204,51,204,0.03)', padding: '10px 18px' }}>
                                  <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                                    {g.items.map(item => {
                                      const status = getAbsenceStatus(item)
                                      return (
                                        <div key={item.id} style={{ display: 'flex', alignItems: 'center', gap: '14px', fontSize: '13px', color: '#475569' }}>
                                          <span style={{ minWidth: 90, fontWeight: 600 }}>{new Date(item.work_date).toLocaleDateString('es-ES')}</span>
                                          <span style={{ flex: 1 }}>{item.absence_reason}</span>
                                          <span>{(item.absence_hours || 0).toFixed(1)} h</span>
                                          <span style={{ padding: '2px 10px', borderRadius: '999px', fontSize: '11px', fontWeight: 700, background: status.className === 'success' ? 'rgba(16,185,129,0.12)' : status.className === 'danger' ? 'rgba(239,68,68,0.12)' : 'rgba(245,158,11,0.14)', color: status.className === 'success' ? '#047857' : status.className === 'danger' ? '#b91c1c' : '#b45309' }}>
                                            {status.label}
                                          </span>
                                        </div>
                                      )
                                    })}
                                  </div>
                                </td>
                              </tr>
                            )}
                          </Fragment>
                        )
                      })}
                    </tbody>
                  </table>

                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', padding: '12px 16px' }}>
                    <span style={{ fontSize: '13px', color: '#64748b' }}>
                      Mostrando {(absCurrentPage - 1) * ABS_PER_PAGE + 1}–{Math.min(absCurrentPage * ABS_PER_PAGE, absFiltered.length)} de {absFiltered.length} profesionales con ausencias
                    </span>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <button type="button" className={styles['btn-icon']} onClick={() => setAbsPage(p => Math.max(1, p - 1))} disabled={absCurrentPage <= 1} style={{ opacity: absCurrentPage <= 1 ? 0.4 : 1 }} title="Página anterior">
                        <ChevronLeft size={16} />
                      </button>
                      <span style={{ fontSize: '13px', fontWeight: 600, color: '#334155', whiteSpace: 'nowrap' }}>
                        Página {absCurrentPage} de {absTotalPages}
                      </span>
                      <button type="button" className={styles['btn-icon']} onClick={() => setAbsPage(p => Math.min(absTotalPages, p + 1))} disabled={absCurrentPage >= absTotalPages} style={{ opacity: absCurrentPage >= absTotalPages ? 0.4 : 1 }} title="Página siguiente">
                        <ChevronRight size={16} />
                      </button>
                    </div>
                  </div>
                </div>
              )}
            </div>

            {/* ── Actividad reciente (feed existente) ── */}
            <h3 style={{ margin: '0 0 14px', fontSize: '17px', fontWeight: 800, color: '#0f172a' }}>Actividad reciente</h3>
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

        {activeTab === 'archived' && (
          <div style={{ padding: '4px 0' }}>
            <div style={{ marginBottom: '16px' }}>
              <h3 style={{ margin: 0, fontSize: '17px', fontWeight: 800, color: '#0f172a' }}>Archivados</h3>
              <p style={{ margin: '2px 0 0', fontSize: '13px', color: '#64748b' }}>
                Profesionales con empleo finalizado o cuenta desactivada. Puedes reactivarlos.
              </p>
            </div>
            <ArchivedList entries={archived} showCompany />
          </div>
        )}
      </div>

      {rptDetail && (
        <div className={styles['modal-overlay']} onClick={() => setRptDetail(null)} style={{ position: 'fixed', inset: 0, zIndex: 1000, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <div className={styles['modal']} onClick={(e) => e.stopPropagation()} style={{ maxWidth: 460, width: '92%' }}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 4 }}>
              <h2 style={{ margin: 0 }}>Detalle de ausencia</h2>
              <button type="button" className={styles['btn-icon']} onClick={() => setRptDetail(null)} title="Cerrar"><X size={18} /></button>
            </div>

            <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '12px 0', borderBottom: '1px solid #e2e8f0', marginBottom: 14 }}>
              <Avatar src={rptDetail.avatar} name={rptDetail.user} size="md" />
              <div>
                <div style={{ fontWeight: 800, color: '#0f172a' }}>{rptDetail.user}</div>
                <div style={{ fontSize: 13, color: '#64748b' }}>{rptDetail.company}</div>
              </div>
              <span className={`${styles['pill']} ${styles[getAbsenceStatus(rptDetail).className]}`} style={{ marginLeft: 'auto' }}>
                {getAbsenceStatus(rptDetail).label}
              </span>
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px 16px', fontSize: 14 }}>
              <DetailField label="Fecha" value={new Date(rptDetail.work_date).toLocaleDateString('es-ES', { weekday: 'long', day: '2-digit', month: 'long', year: 'numeric' })} full />
              <DetailField label="Horas ausentes" value={`${(rptDetail.absence_hours || 0).toFixed(1)} h`} />
              <DetailField label="Horas trabajadas" value={`${(rptDetail.hours_worked || 0).toFixed(1)} h`} />
              <DetailField label="Motivo" value={rptDetail.absence_reason || 'Sin motivo'} full />
              {rptDetail.email && <DetailField label="Email" value={rptDetail.email} full />}
              {rptDetail.phone_number && <DetailField label="Teléfono" value={rptDetail.phone_number} />}
              {rptDetail.created_at && <DetailField label="Registrado" value={new Date(rptDetail.created_at).toLocaleString('es-ES')} full />}
            </div>

            <div className={styles['modal-actions']} style={{ marginTop: 18 }}>
              {rptDetail.email && (
                <a className={styles['btn-secondary']} href={`mailto:${rptDetail.email}?subject=${encodeURIComponent('Seguimiento de ausencia en Obertrack')}`} onClick={() => adminService.logContact(rptDetail.user_id, 'email')} style={{ textDecoration: 'none', display: 'inline-flex', alignItems: 'center', gap: 6 }}>
                  <Mail size={15} /> Contactar
                </a>
              )}
              <button className={styles['btn-secondary']} onClick={() => setRptDetail(null)}>Cerrar</button>
            </div>
          </div>
        </div>
      )}

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

      {showBulkDeleteModal && (
        <div className={styles['modal-overlay']} onClick={() => !bulkDeleteBusy && setShowBulkDeleteModal(false)}>
          <div className={styles['modal']} onClick={(e) => e.stopPropagation()}>
            {bulkDeleteResult ? (
              <>
                <h2>Eliminación masiva completada</h2>
                <p><strong>{bulkDeleteResult.deleted}</strong> usuario(s) eliminado(s).</p>
                {bulkDeleteResult.skipped.length > 0 && (
                  <>
                    <p className={styles['warning-text']}>
                      {bulkDeleteResult.skipped.length} omitido(s):
                    </p>
                    <ul style={{ maxHeight: 200, overflowY: 'auto', margin: '0 1.5rem 0.5rem', padding: '0 0 0 1.1rem', fontSize: '13px', color: '#475569' }}>
                      {bulkDeleteResult.skipped.map((s) => (
                        <li key={s.id}><strong>{s.name || `#${s.id}`}</strong> — {s.reason}</li>
                      ))}
                    </ul>
                  </>
                )}
                <div className={styles['modal-actions']}>
                  <button className={styles['btn-secondary']} onClick={() => setShowBulkDeleteModal(false)}>Cerrar</button>
                </div>
              </>
            ) : (
              <>
                <h2>Eliminar usuarios seleccionados</h2>
                <p>
                  Seleccionaste <strong>{selectedIds.size}</strong> usuario(s). Se eliminarán{' '}
                  <strong>{bulkWillDelete}</strong>.
                </p>
                {selectedEmployers.length > 0 && (
                  <>
                    <p className={styles['warning-text']} style={{ color: '#b91c1c' }}>
                      Atención: hay {selectedEmployers.length} cuenta(s) de empresa en la selección.
                      Al eliminarlas, su gente queda sin empresa:
                    </p>
                    <ul style={{ maxHeight: 180, overflowY: 'auto', margin: '0 1.5rem 0.5rem', padding: '0 0 0 1.1rem', fontSize: '13px', color: '#475569' }}>
                      {selectedEmployers.map((e) => (
                        <li key={e.id}><strong>{e.name}</strong> — {e.linked} usuario(s) vinculado(s)</li>
                      ))}
                    </ul>
                  </>
                )}
                {selectedManagersWithTeam.length > 0 && (
                  <>
                    <p className={styles['warning-text']}>
                      Se omitirán {selectedManagersWithTeam.length} manager(es) con profesionales a
                      cargo (reasigna su equipo primero para poder eliminarlos):
                    </p>
                    <ul style={{ maxHeight: 180, overflowY: 'auto', margin: '0 1.5rem 0.5rem', padding: '0 0 0 1.1rem', fontSize: '13px', color: '#475569' }}>
                      {selectedManagersWithTeam.map((m) => (
                        <li key={m.id}><strong>{m.name}</strong> — {m.count} profesional(es) a cargo</li>
                      ))}
                    </ul>
                  </>
                )}
                <p className={styles['warning-text']}>
                  Dejarán de aparecer y no podrán iniciar sesión (sus registros se conservan, podés
                  restaurarlos desde la Papelera). Los superadmins y tu propia cuenta nunca se
                  eliminan desde aquí.
                </p>
                {bulkDeleteError && (
                  <p style={{ color: '#dc2626', fontWeight: 600, padding: '0 1.5rem', margin: '0 0 0.5rem' }}>{bulkDeleteError}</p>
                )}
                <div className={styles['modal-actions']}>
                  <button className={styles['btn-secondary']} onClick={() => setShowBulkDeleteModal(false)} disabled={bulkDeleteBusy}>Cancelar</button>
                  <button className={styles['btn-danger']} onClick={handleBulkDelete} disabled={bulkDeleteBusy || bulkWillDelete === 0}>
                    {bulkDeleteBusy ? 'Eliminando…' : `Eliminar ${bulkWillDelete}`}
                  </button>
                </div>
              </>
            )}
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

      <EmailComposerModal
        isOpen={composer !== null}
        onClose={() => setComposer(null)}
        recipient={composer?.recipient ?? null}
        defaultBody={composer?.body ?? ''}
      />

      {/* ===== MODAL CREAR USUARIO ===== */}
      {showCreateModal && (
        <CreateUserModal
          form={createForm}
          setForm={setCreateForm}
          onClose={() => setShowCreateModal(false)}
          onSubmit={handleCreateUser}
          loading={createLoading}
          error={createError}
          publicCompanies={publicCompanies}
        />
      )}

      {showImportModal && (
        <ImportUsersModal
          onClose={() => setShowImportModal(false)}
          onDone={() => { fetchUsers(); fetchDashboard(); fetchCompanies() }}
        />
      )}

      {showExportModal && (
        <ExportUsersModal
          users={filteredUsers}
          companyName={resolveCompanyName}
          filtered={!!(searchQuery.trim() || roleFilter || companyFilter !== '')}
          onClose={() => setShowExportModal(false)}
        />
      )}    </div>
  )
}

const actionMenuItem: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 10,
  width: '100%',
  padding: '10px 12px',
  border: 'none',
  borderRadius: 8,
  background: 'transparent',
  color: '#334155',
  fontSize: 14,
  fontWeight: 600,
  cursor: 'pointer',
  textAlign: 'left',
}
