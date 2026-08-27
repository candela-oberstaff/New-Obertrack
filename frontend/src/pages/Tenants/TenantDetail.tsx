import { useState, useEffect, useRef, cloneElement, useMemo, Fragment } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Building2, Users, User, LayoutGrid, CheckSquare, Activity, Ban, CheckCircle2, Mail, Calendar, RefreshCw, ChevronLeft, ChevronRight, Pencil, Search, Clock, Hourglass, Inbox, Wand2, X, MessageSquare, Plus, Trash2, Pin, PinOff, Send, Phone, Paperclip, MoreVertical, Eye, Briefcase, CalendarDays, Settings } from 'lucide-react'
import { useImagePaste } from '../../hooks/useImagePaste'
import type { TenantContactChannel } from '../../services/admin.service'
import { ACTIVITY_STYLE, ACTIVITY_LABEL, ACTIVITY_FALLBACK, CONTACT_STYLE } from './activityStyle'

// Canales que se anotan a mano porque ocurren fuera de la plataforma. El correo
// y el WhatsApp no están aquí: los registra el sistema al enviarlos, y ofrecer
// anotarlos a mano invitaría a duplicarlos.
const MANUAL_CONTACT_CHANNELS: { value: TenantContactChannel; label: string }[] = [
  { value: 'call', label: 'Llamada telefónica' },
  { value: 'meeting', label: 'Reunión' },
]

const NOTE_MAX_LENGTH = 2000

import { ticketOrigin, TICKET_STAGE, ticketPath } from './ticketStyle'
import { EmployeePeekModal } from './EmployeePeekModal'
import { useQuery } from '@tanstack/react-query'
import { useTenantDetail, useTenantActivity, useFollowUps, ACTIVITY_CATEGORIES } from '../../hooks'
import { TeamActivityPanel } from '../../components/Admin/TeamActivityPanel'
import { AbsenceReportPanel } from '../../components/Admin/AbsenceReportPanel'
import { EmailComposerModal, type ComposerRecipient } from '../../components/Admin/EmailComposerModal'
import { ticketService } from '../../services/ticket.service'
import { openWaConversation } from '../../lib/whatsappInbox'
import { useNotification } from '../../context/NotificationContext'
import { groupByDay } from './activityGrouping'
import { healthSignal, HEALTH_COLOR } from './accountHealth'
import { EventThread } from './EventThread'
import { adminService } from '../../services/api'
import { useAuth } from '../../context/AuthContext'
import type { EmployeeSummary } from '../../types'
import Avatar from '../../components/Common/Avatar'
import { Modal, Button, RecordPager, Skeleton, DatePicker, toISODate } from '../../components/ui'
import { setRecordNav } from '../../lib/recordNav'
import { Select } from '../../components/ui/Select'
import { COUNTRY_OPTIONS, getStatesForCountry } from '../../components/Auth/countries'
import { INDUSTRY_OPTIONS } from '../../components/Auth/industries'
import { ArchivedList } from '../../components/Admin/ArchivedList'
import { useConfirm } from '../../components/ui/ConfirmProvider'
import { OrgChartPanel } from '../../components/OrgChart/OrgChartPanel'
import EmployeeScheduleModal from './EmployeeScheduleModal'
import styles from './Tenants.module.css'

const EMP_PER_PAGE = 5

const EMPTY_EDIT_FORM = {
  company_name: '',
  industry: '',
  phone_number: '',
  country: '',
  state: '',
  city: '',
  location: '',
  address: '',
  client_since: '',
}

function generatePassword(): string {
  const chars = 'ABCDEFGHJKMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789'
  const bytes = crypto.getRandomValues(new Uint8Array(12))
  let pw = Array.from(bytes).map(b => chars[b % chars.length]).join('')
  if (!/\d/.test(pw)) pw = pw.slice(0, -1) + '7'
  if (!/[a-zA-Z]/.test(pw)) pw = pw.slice(0, -1) + 'k'
  return pw
}

const formatTimeToAMPM = (timeStr: string | undefined): string => {
  if (!timeStr) return '—'
  const parts = timeStr.split(':')
  if (parts.length < 2) return timeStr
  let hour = parseInt(parts[0], 10)
  const minute = parts[1]
  const ampm = hour >= 12 ? 'pm' : 'am'
  hour = hour % 12
  hour = hour ? hour : 12
  return `${hour}:${minute} ${ampm}`
}

export default function TenantDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { user: viewer } = useAuth()
  // CS entra en modo consulta: sin editar, suspender ni tocar profesionales.
  const canManage = !!viewer?.is_superadmin
  // Anotar el expediente (notas y contactos) sí lo hace también Customer
  // Success: es el área que atiende a estas empresas, y un expediente que solo
  // puede alimentar el superadmin no sirve de material de seguimiento.
  const canAnnotate = canManage || viewer?.user_type === 'customer_success'
  const tenantId = Number(id)
  const { tenant, employees, isLoading, error, refresh, suspendTenant, activateTenant, toggleEmployeeStatus, resetEmployeePassword } = useTenantDetail(tenantId)
  const confirm = useConfirm()
  const notify = useNotification()

  // Expediente: filtros (categoría y persona) + paginación, todo de servidor.
  const [actCategory, setActCategory] = useState('lifecycle')
  const [actPerson, setActPerson] = useState(0)
  const [actPage, setActPage] = useState(1)
  const actPageSize = actCategory === 'note' ? 1000 : 5
  const {
    activity,
    total: actTotal,
    counts: actCounts,
    people: actPeople,
    // pinnedNotes, -- eliminado: las notas fijadas se renderizan desde 'activity' directamente
    threads,
    isLoading: actLoading,
    isFetching: actFetching,
    error: actError,
    addNote,
    logContact,
    updateNote,
    setNotePinned,
    deleteNote,
    addComment,
    updateComment,
    deleteComment,
    addAttachment,
    deleteAttachment,
  } = useTenantActivity(tenantId, actCategory, actPerson, actPage, actPageSize)

  // Ordenar las notas fijadas arriba del expediente cuando estamos en la pestaña de notas
  const processedActivity = useMemo(() => {
    if (actCategory !== 'note') return activity

    const pinned = activity.filter(a => a.pinned)
    const unpinned = activity.filter(a => !a.pinned)

    const sortFn = (x: any, y: any) => {
      const timeX = new Date(x.timestamp).getTime()
      const timeY = new Date(y.timestamp).getTime()
      return timeY - timeX // newest first
    }

    pinned.sort(sortFn)
    unpinned.sort(sortFn)

    return [...pinned, ...unpinned]
  }, [activity, actCategory])

  const lastPinnedId = useMemo(() => {
    if (actCategory !== 'note') return null
    const pinned = activity.filter(a => a.pinned)
    if (pinned.length === 0) return null

    const sortFn = (x: any, y: any) => {
      const timeX = new Date(x.timestamp).getTime()
      const timeY = new Date(y.timestamp).getTime()
      return timeY - timeX // newest first
    }
    pinned.sort(sortFn)
    return pinned[pinned.length - 1]?.event_id || null
  }, [activity, actCategory])

  const [actSubTab, setActSubTab] = useState<'inactividad' | 'ausencias'>('inactividad')
  const actTotalPages = Math.max(1, Math.ceil(actTotal / actPageSize))
  const [noteOpen, setNoteOpen] = useState(false)
  const [noteText, setNoteText] = useState('')
  const [noteSaving, setNoteSaving] = useState(false)
  const [noteError, setNoteError] = useState<string | null>(null)
  // Id de la nota que se está corrigiendo; null = se está escribiendo una nueva.
  const [noteEditingId, setNoteEditingId] = useState<number | null>(null)
  // Archivos en cola: se suben DESPUÉS de guardar la nota, porque necesitan la
  // entrada creada para colgarse de ella.
  const [noteFiles, setNoteFiles] = useState<File[]>([])
  const noteFileRef = useRef<HTMLInputElement>(null)
  // Pegar una captura o arrastrarla la encola igual que el botón de adjuntar.
  const { onPaste: onNotePaste, onDrop: onNoteDrop } = useImagePaste(files => {
    setNoteFiles(prev => [...prev, ...files])
  })

  // Registro manual de un contacto que pasó fuera de la plataforma.
  const [contactOpen, setContactOpen] = useState(false)
  const [contactChannel, setContactChannel] = useState<TenantContactChannel>('call')
  const [contactDetail, setContactDetail] = useState('')
  const [contactSaving, setContactSaving] = useState(false)
  const [contactError, setContactError] = useState<string | null>(null)
  const [scheduleModalEmployee, setScheduleModalEmployee] = useState<EmployeeSummary | null>(null)
  const [scheduleViewEmployee, setScheduleViewEmployee] = useState<EmployeeSummary | null>(null)

  const [tab, setTab] = useState<'resumen' | 'usuarios' | 'organigrama' | 'expediente' | 'actividad' | 'tickets' | 'archivados' | 'horarios'>('resumen')

  const [menuOpen, setMenuOpen] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  // Archivados de esta empresa (bajas + cuentas desactivadas).
  //
  // Por React Query como el resto: la carga a mano se tragaba el error en
  // silencio, así que un fallo del servidor se veía igual que "no hay
  // archivados". La clave incluye tenantId, que además resuelve lo de saltar de
  // ficha con el paginador sin desmontar la página.
  const { data: archived = [], isLoading: archivedLoading, error: archivedError } = useQuery({
    queryKey: ['tenant-archived', tenantId],
    queryFn: () => adminService.getTenantArchived(tenantId),
    enabled: !!tenantId && tab === 'archivados',
  })

  // Pestaña Actividad: el mismo semáforo de inactividad y reporte de ausencias
  // del panel de administración, pero solo de esta empresa. El backend ya los
  // acota; aquí no se filtra en cliente para no traerse el resto de empresas.
  const { data: teamInactive = [], isLoading: teamInactiveLoading } = useQuery({
    queryKey: ['tenant-inactive-users', tenantId],
    queryFn: () => adminService.getTenantInactiveUsers(tenantId, 1),
    enabled: !!tenantId && tab === 'actividad',
  })
  const { data: tenantAbsence } = useQuery({
    queryKey: ['tenant-absence-report', tenantId],
    queryFn: () => adminService.getTenantAbsenceReport(tenantId),
    enabled: !!tenantId && tab === 'actividad',
  })
  // La gestión de CS es la misma bitácora del panel: comparte clave de consulta
  // para que anotar aquí se vea allí sin recargar.
  const inactivityFollowUps = useFollowUps('inactivity', tab === 'actividad')
  const absenceFollowUps = useFollowUps('absence', tab === 'actividad')
  // Redacción de correo a un profesional (la del contacto con la empresa es
  // otra: esta va dirigida a la persona y queda en su expediente).
  const [proComposer, setProComposer] = useState<{ recipient: ComposerRecipient; body: string } | null>(null)

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
  const [schedPage, setSchedPage] = useState(1)
  const [schedSearch, setSchedSearch] = useState('')
  // Profesional del vistazo rápido (null = ninguno abierto).
  const [peekEmployee, setPeekEmployee] = useState<number | null>(null)

  // Redacción de correo a la empresa. El WhatsApp no pasa por aquí: o lleva a
  // la conversación real de la bandeja, o abre wa.me.
  const [commOpen, setCommOpen] = useState(false)

  // ¿Hay conversación de WhatsApp con el teléfono de esta empresa? De la
  // respuesta depende si el botón lleva a la bandeja o abre wa.me. Se consulta
  // aparte de la empresa para no retrasar la carga de la ficha por ello.
  const { data: waLookup } = useQuery({
    queryKey: ['tenant-wa-lookup', tenantId, tenant?.phone_number],
    queryFn: () => ticketService.lookupWaChat(tenant!.phone_number!),
    enabled: !!tenant?.phone_number,
    // Si la bandeja no está disponible se cae al enlace wa.me, que siempre
    // funciona: no tiene sentido reintentar y dejar el botón en el limbo.
    retry: false,
  })

  // Tickets de la empresa. Mismo criterio que el KPI de la cabecera, así que el
  // número y la lista no se contradicen nunca.
  const { data: tickets = [], isLoading: ticketsLoading, error: ticketsError } = useQuery({
    queryKey: ['tenant-tickets', tenantId],
    queryFn: () => adminService.getTenantTickets(tenantId),
    enabled: !!tenantId && tab === 'tickets',
  })
  const TICKETS_PER_PAGE = 5
  const [ticketPage, setTicketPage] = useState(1)
  const ticketTotalPages = Math.max(1, Math.ceil(tickets.length / TICKETS_PER_PAGE))
  const ticketsSlice = tickets.slice((ticketPage - 1) * TICKETS_PER_PAGE, ticketPage * TICKETS_PER_PAGE)

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
    setSchedPage(1)
    setSchedSearch('')
    setActCategory('lifecycle')
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
        // Los archivos que se hayan añadido al corregir se cuelgan de la misma
        // entrada, que ya existe.
        await uploadNoteFiles(noteEditingId)
      } else {
        // La nota primero: los archivos necesitan una entrada a la que colgarse,
        // igual que en los comentarios del expediente.
        const eventId = await addNote(text)
        await uploadNoteFiles(eventId)
        // La nota nueva es lo más reciente: se ve en la primera página.
        setActPage(1)
      }
      setNoteOpen(false)
      setNoteText('')
      setNoteFiles([])
      setNoteEditingId(null)
    } catch (err: any) {
      setNoteError(err?.response?.data?.error || 'No se pudo guardar la nota')
    } finally {
      setNoteSaving(false)
    }
  }

  // Los adjuntos van de uno en uno y en serie: si uno falla, se dice cuál y la
  // nota ya está guardada (no se pierde lo escrito por un archivo que pesaba de
  // más o tenía un tipo no admitido).
  const uploadNoteFiles = async (eventId: number) => {
    if (!eventId || noteFiles.length === 0) return
    for (const file of noteFiles) {
      try {
        await addAttachment(eventId, file)
      } catch (err: any) {
        throw new Error(err?.response?.data?.error || `No se pudo adjuntar "${file.name}"`)
      }
    }
  }

  const openContact = () => {
    setContactChannel('call')
    setContactDetail('')
    setContactError(null)
    setContactOpen(true)
  }

  const handleSaveContact = async () => {
    setContactSaving(true)
    setContactError(null)
    try {
      await logContact(contactChannel, contactDetail.trim())
      // El contacto recién registrado es lo más reciente: se ve en la primera
      // página del expediente.
      setActPage(1)
      setContactOpen(false)
      setContactDetail('')
    } catch (err: any) {
      setContactError(err?.response?.data?.error || 'No se pudo registrar el contacto')
    } finally {
      setContactSaving(false)
    }
  }

  const handleTogglePin = async (note: { event_id: number; pinned?: boolean }) => {
    try {
      await setNotePinned(note.event_id, !note.pinned)
    } catch (err: any) {
      notify.error(err?.response?.data?.error || 'No se pudo cambiar el estado de la nota')
    }
  }

  const handleDeleteNote = async (eventId: number) => {
    // Se dice qué más se lleva por delante ANTES de borrar. Una nota con la
    // conversación y las capturas de una incidencia no es "una nota", y quien
    // pulsa el botón tiene que saber qué está tirando.
    const thread = threads[eventId]
    const nComments = thread?.comments?.length ?? 0
    const nFiles = (thread?.attachments?.length ?? 0) +
      (thread?.comments ?? []).reduce((sum, c) => sum + (c.attachments?.length ?? 0), 0)
    const extras = [
      nComments > 0 ? `${nComments} ${nComments === 1 ? 'comentario' : 'comentarios'}` : '',
      nFiles > 0 ? `${nFiles} ${nFiles === 1 ? 'archivo' : 'archivos'}` : '',
    ].filter(Boolean).join(' y ')

    const ok = await confirm({
      title: '¿Eliminar esta nota?',
      message: extras
        ? `Se eliminarán también ${extras}. El resto de movimientos no se puede borrar.`
        : 'Desaparecerá del expediente. El resto de movimientos no se puede borrar.',
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

  const schedFiltered = employees
    .filter(emp => {
      const q = schedSearch.trim().toLowerCase()
      if (q && !(emp.name?.toLowerCase().includes(q) || emp.email?.toLowerCase().includes(q))) return false
      return true
    })
    .sort((a, b) => (a.name || '').localeCompare(b.name || '', 'es', { sensitivity: 'base' }))

  const schedTotalPages = Math.max(1, Math.ceil(schedFiltered.length / 5))
  const schedCurrentPage = Math.min(schedPage, schedTotalPages)
  const schedPaginated = schedFiltered.slice((schedCurrentPage - 1) * 5, schedCurrentPage * 5)

  // Igual que en el listado de empresas: la ficha del profesional hereda el
  // orden filtrado de esta tabla para poder recorrer la plantilla de una.
  const openEmployee = (employeeId: number) => {
    setRecordNav(`tenant-employees:${tenantId}`, empFiltered.map(e => e.id))
    navigate(`/admin/tenants/${tenantId}/employees/${employeeId}`)
  }

  // El pager de la ficha completa se siembra ANTES de navegar, también cuando
  // se llega desde el vistazo rápido: si no, la ficha abierta desde el modal
  // no sabría por qué plantilla se está recorriendo.
  const seedEmployeeNav = () => setRecordNav(`tenant-employees:${tenantId}`, empFiltered.map(e => e.id))

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
      client_since: (tenant.client_since || '').slice(0, 10),
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

  // El correo a la empresa pasa por el mismo redactor que el de profesionales:
  // deja elegir una plantilla de Tools o escribir de cero. Antes era un modal
  // con asunto y cuerpo en blanco, así que las plantillas guardadas no servían
  // para la comunicación con clientes, que es justo donde se repiten los textos.
  const openComm = () => {
    if (!tenant?.owner_email) {
      notify.warning('Esta empresa no tiene un correo de contacto asociado.')
      return
    }
    setCommOpen(true)
  }

  // WhatsApp: SIEMPRE a nuestra bandeja, aunque todavía no haya conversación
  // (se crea el hilo vacío). Antes, sin conversación previa, saltaba a wa.me y
  // el mensaje se iba por el WhatsApp personal de quien atendía: fuera del
  // historial, sin estado de entrega y sin que nadie más lo viera.
  //
  // Abrirla NO habilita escribir en frío: el envío sigue rechazándose si nadie
  // ha escrito desde el otro lado (así no se expone la línea oficial a un
  // bloqueo de Meta). En ese caso la propia vista lo dice y ofrece wa.me.
  const handleWhatsApp = async () => {
    if (!tenant?.phone_number?.trim()) {
      notify.warning('Esta empresa no tiene un teléfono registrado.')
      return
    }
    const ok = await openWaConversation(tenant.phone_number, tenant.owner_name || tenant.company_name, navigate)
    if (ok) {
      // El clic ES el contacto: refleja el intento, no la entrega.
      logContact('whatsapp', 'Abierto desde la ficha (bandeja de WhatsApp)').catch(() => { })
      return
    }
    notify.error('No se pudo abrir la conversación de WhatsApp. Revisa la bandeja e inténtalo de nuevo.')
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

  // El alta que se muestra es la corregida a mano cuando existe. created_at es
  // solo cuándo se creó la cuenta aquí, que en las empresas cargadas después de
  // empezar a trabajar con nosotros no es el alta que reconoce nadie.
  const clientSince = tenant.client_since || tenant.created_at
  const createdLabel = clientSince ? new Date(clientSince).toLocaleDateString('es-ES') : '-'
  const editStates = getStatesForCountry(editForm.country)
  const contactHealth = healthSignal(tenant.last_contact_at)
  const activityHealth = healthSignal(tenant.last_activity_at)

  // Iconos en estilo "suave": fondo pastel + icono del mismo tono (igual que las
  // tarjetas del panel admin), en vez de gradientes saturados.
  const kpis = [
    { value: createdLabel, label: 'Fecha de alta', icon: <Calendar size={24} />, bg: '#f1f5f9', color: '#64748b' },
    { value: tenant.user_count, label: 'Profesionales', icon: <Users size={24} />, bg: '#f1f5f9', color: '#64748b' },
    { value: tenant.board_count, label: 'Tableros', icon: <LayoutGrid size={24} />, bg: '#f1f5f9', color: '#64748b' },
    { value: tenant.task_count, label: 'Tareas', icon: <CheckSquare size={24} />, bg: '#f1f5f9', color: '#64748b' },
    { value: `${(tenant.hours_this_month ?? 0).toFixed(1)} h`, label: 'Horas este mes', icon: <Clock size={24} />, bg: '#f1f5f9', color: '#64748b' },
    { value: `${(tenant.pending_hours ?? 0).toFixed(1)} h`, label: 'Horas por aprobar', icon: <Hourglass size={24} />, bg: '#f1f5f9', color: '#64748b' },
    { value: tenant.open_tickets ?? 0, label: 'Tickets abiertos', icon: <Inbox size={24} />, bg: '#f1f5f9', color: '#64748b' },
    {
      value: <span style={{ color: HEALTH_COLOR[contactHealth.level], fontWeight: 800 }}>{contactHealth.label}</span>,
      label: 'Último contacto',
      icon: <Send size={24} />,
      bg: '#f1f5f9',
      color: '#64748b'
    },
    {
      value: <span style={{ color: HEALTH_COLOR[activityHealth.level], fontWeight: 800 }}>{activityHealth.label}</span>,
      label: 'Última actividad',
      icon: <Activity size={24} />,
      bg: '#f1f5f9',
      color: '#64748b'
    },
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
      {/* Cabecera unificada en una sola fila */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', marginBottom: '20px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '12px', minWidth: 0 }}>
          <button className={styles.backBtn} style={{ marginBottom: 0, paddingRight: '8px' }} onClick={() => navigate('/admin/tenants')} title="Volver a Empresas" aria-label="Volver a Empresas">
            <ArrowLeft size={18} />
          </button>
          <div className={styles.companyLogoSm}>{tenant.company_name?.charAt(0).toUpperCase() || '?'}</div>
          <h1 className={styles.headerTitle}>{tenant.company_name}</h1>
          <span className={`${styles.badge} ${tenant.is_active ? styles.badgeActive : styles.badgeSuspended}`} style={{ marginLeft: '4px' }}>
            {tenant.is_active ? 'Activa' : 'Suspendida'}
          </span>
        </div>
        <div className={styles.pagerActionsContainer}>
          {canManage && (
            <button
              className={styles.circleActionBtn}
              onClick={openEdit}
              title="Editar empresa"
              aria-label="Editar empresa"
            >
              <Pencil size={18} />
            </button>
          )}
          <div className={styles.dropdownWrapper} ref={menuRef}>
            <button
              className={styles.circleActionBtn}
              onClick={() => setMenuOpen(!menuOpen)}
              title="Más opciones"
              aria-label="Más opciones"
            >
              <MoreVertical size={18} />
            </button>
            {menuOpen && (
              <div className={styles.dropdownMenu}>
                <button
                  className={styles.dropdownItem}
                  onClick={() => {
                    setMenuOpen(false)
                    openComm()
                  }}
                >
                  <Mail size={16} /> Enviar Correo
                </button>
                <button
                  className={styles.dropdownItem}
                  onClick={() => {
                    if (tenant.phone_number?.trim()) {
                      setMenuOpen(false)
                      handleWhatsApp()
                    }
                  }}
                  title={
                    !tenant.phone_number?.trim()
                      ? 'Esta empresa no tiene teléfono registrado'
                      : 'Abre la conversación en la bandeja de WhatsApp'
                  }
                  disabled={!tenant.phone_number?.trim()}
                >
                  <MessageSquare size={16} /> {waLookup?.ticket_id ? 'Ver conversación' : 'Abrir WhatsApp'}
                </button>
                {canManage && (
                  <>
                    {tenant.is_active ? (
                      <button
                        className={`${styles.dropdownItem} ${styles.dropdownItemDanger}`}
                        onClick={() => {
                          setMenuOpen(false)
                          handleSuspend()
                        }}
                      >
                        <Ban size={16} /> Suspender acceso
                      </button>
                    ) : (
                      <button
                        className={`${styles.dropdownItem} ${styles.dropdownItemSuccess}`}
                        onClick={() => {
                          setMenuOpen(false)
                          handleActivate()
                        }}
                      >
                        <CheckCircle2 size={16} /> Reactivar acceso
                      </button>
                    )}
                  </>
                )}
              </div>
            )}
          </div>
          <RecordPager
            scope="tenants"
            currentId={tenantId}
            toPath={id => `/admin/tenants/${id}`}
            noun="empresa"
          />
        </div>
      </div>

      <div className={styles.twoColumnGrid}>
        {/* Columna Izquierda: Información Básica y Otra Información */}
        <div className={styles.leftSidebar}>
          {/* Card 1: Información Básica */}
          <div className={styles.sidebarCard}>
            <h2 className={styles.sidebarCardTitle}>Información Básica</h2>
            <div className={styles.sidebarFieldsList}>
              <div className={styles.sidebarField}>
                <span className={styles.sidebarLabel}>Propietario</span>
                <span className={styles.sidebarFieldIconRow}>
                  <User size={14} style={{ color: '#94a3b8', flexShrink: 0 }} /> {tenant.owner_name || '—'}
                </span>
              </div>
              <div className={styles.sidebarField}>
                <span className={styles.sidebarLabel}>Email</span>
                <span className={styles.sidebarFieldIconRow}>
                  <Mail size={14} style={{ color: '#94a3b8', flexShrink: 0 }} /> {tenant.owner_email || '—'}
                </span>
              </div>


              {infoFields.map(f => (
                <div className={styles.sidebarField} key={f.label}>
                  <span className={styles.sidebarLabel}>{f.label}</span>
                  <span className={styles.sidebarValue}>{f.value?.trim() || '—'}</span>
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* Columna Derecha: Tabs y Contenido de las pestañas */}
        <div className={styles.rightContentArea}>
          {/* Navegación principal: desplegable de sección + desplegable de categoría (expediente) */}
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'wrap', marginBottom: 24 }}>
            <div style={{ minWidth: 200 }}>
              <Select
                fullWidth
                placeholder="Sección"
                value={tab}
                onChange={v => setTab(String(v) as 'resumen' | 'usuarios' | 'organigrama' | 'expediente' | 'actividad' | 'tickets' | 'archivados' | 'horarios')}
                options={[
                  { value: 'resumen', label: 'Resumen' },
                  { value: 'usuarios', label: `Profesionales (${employees.length})` },
                  { value: 'organigrama', label: 'Organigrama' },
                  { value: 'expediente', label: 'Expediente' },
                  { value: 'actividad', label: 'Actividad' },
                  { value: 'tickets', label: `Tickets${(tenant.open_tickets ?? 0) > 0 ? ` (${tenant.open_tickets})` : ''}` },
                  { value: 'archivados', label: 'Archivados' },
                  ...(canAnnotate ? [{ value: 'horarios', label: 'Horarios' }] : []),
                ]}
              />
            </div>
            {/* Cuando la sección es "Expediente", aparece el desplegable de categoría */}
            {tab === 'expediente' && (
              <div style={{ minWidth: 200 }}>
                <Select
                  fullWidth
                  placeholder="Categoría"
                  value={actCategory ?? ''}
                  onChange={v => setActCategory(String(v))}
                  options={ACTIVITY_CATEGORIES.map(cat => ({
                    value: cat.value,
                    label: actCounts[cat.value] !== undefined
                      ? `${cat.label} (${actCounts[cat.value]})`
                      : cat.label,
                  }))}
                />
              </div>
            )}
            {/* Cuando la sección es "Actividad", aparece el desplegable Inactividad / Ausencias */}
            {tab === 'actividad' && (
              <div style={{ minWidth: 180 }}>
                <Select
                  fullWidth
                  placeholder="Vista"
                  value={actSubTab}
                  onChange={v => setActSubTab(String(v) as 'inactividad' | 'ausencias')}
                  options={[
                    { value: 'inactividad', label: `Inactividad (${teamInactive.length})` },
                    { value: 'ausencias', label: `Ausencias (${tenantAbsence?.items?.length || 0})` },
                  ]}
                />
              </div>
            )}
          </div>

          {tab === 'resumen' && (
            <div className={styles.sidebarCard} style={{ margin: 0 }}>
              <h2 className={styles.sidebarCardTitle}>Otra Información</h2>
              <div className={styles.kpiGrid3x2}>
                {kpis.map((kpi: any) => {
                  const Tag = kpi.onClick ? 'button' : 'div'
                  return (
                    <Tag
                      key={kpi.label}
                      className={styles.kpiGridItem}
                      {...(kpi.onClick
                        ? { type: 'button' as const, onClick: kpi.onClick }
                        : {})}
                    >
                      <div className={styles.kpiGridIcon} style={{ background: kpi.bg, color: kpi.color }}>
                        {kpi.icon && typeof kpi.icon === 'object'
                          ? cloneElement(kpi.icon as any, { size: 24 })
                          : kpi.icon}
                      </div>
                      <div className={styles.kpiGridContent}>
                        <span className={styles.kpiGridLabel}>{kpi.label}</span>
                        <span className={styles.kpiGridValue}>{kpi.value}</span>
                      </div>
                    </Tag>
                  )
                })}
              </div>
            </div>
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
                            <tr key={emp.id} className={styles.row} onClick={() => setPeekEmployee(emp.id)} title="Ver un vistazo rápido">
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
                                  {/* La fila abre el vistazo; el chevron se salta el
                                  atajo y va directo a la ficha completa. */}
                                  <button
                                    className={styles.iconBtn}
                                    onClick={(e) => { e.stopPropagation(); openEmployee(emp.id) }}
                                    title="Abrir la ficha completa"
                                    aria-label={`Abrir la ficha completa de ${emp.name}`}
                                  >
                                    <ChevronRight size={18} className={styles.chevron} />
                                  </button>
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

          {/* El organigrama del cliente. El superadmin puede reordenarlo: es la
          misma acción que ya hacía desde la ficha de cada profesional, solo que
          viendo la estructura entera. */}
          {tab === 'organigrama' && (
            <OrgChartPanel
              companyId={tenant.id}
              editable
              hint="Arrastra a una persona sobre otra para cambiar su manager. Se lleva a su equipo con ella. Haz clic en alguien para abrir su ficha."
              profileHref={p => `/admin/users/${p.user_id}`}
            />
          )}

          {/* El expediente es una vista de auditoría: si no hay movimientos se dice,
          no se rellena con eventos de ejemplo. */}
          {tab === 'expediente' && (
            <>

              {/* Filtro por persona + botones de acción: fila separada, alineada a la izquierda */}
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap', marginBottom: 20 }}>
                <div style={{ minWidth: 230 }}>
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
                {canAnnotate && (
                  <>
                    {actCategory === 'contact' && (
                      <Button
                        size="sm"
                        variant="secondary"
                        leftIcon={<Phone size={14} />}
                        onClick={openContact}
                      >
                        Registrar
                      </Button>
                    )}
                    {actCategory === 'note' && (
                      <Button
                        size="sm"
                        variant="secondary"
                        leftIcon={<Plus size={14} />}
                        onClick={openNewNote}
                      >
                        Añadir
                      </Button>
                    )}
                  </>
                )}
              </div>

              {actError && <p className={styles.errorMsg}>{actError}</p>}


              {actLoading ? (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '0.6rem' }}>
                  {Array.from({ length: 5 }).map((_, i) => <Skeleton key={i} height={56} radius={12} />)}
                </div>
              ) : activity.length === 0 ? (
                <div className={styles.empty}>
                  <Activity size={40} />
                  <p>
                    {actCategory !== 'lifecycle' || actPerson !== 0
                      ? 'Sin movimientos con estos filtros'
                      : 'Sin movimientos en el expediente'}
                  </p>
                  {(actCategory !== 'lifecycle' || actPerson !== 0) && (
                    <button
                      type="button"
                      onClick={() => { setActCategory('lifecycle'); setActPerson(0) }}
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
                  <div className={styles.timelineScrollContainer}>
                    {/* Pinned section eliminada: las notas fijadas se muestran ya en la lista
                    principal con el estilo isPinnedCard y ordenadas al inicio. */}
                    <div className={`${styles.timeline} ${actCategory === 'note' ? styles.isNotesOnly : ''}`} style={{ opacity: actFetching ? 0.6 : 1, transition: 'opacity 0.15s ease' }}>
                      {groupByDay(processedActivity).map((group, gi) => (
                        <div key={`${group.key}-${gi}`} className={styles.timelineDay}>
                          {/* Comentado a petición del usuario:
                          <div className={styles.timelineDayLabel}>
                            <span className={styles.timelineDayChip}>
                              {group.label}
                              <span className={styles.timelineDayCount}>
                                {group.items.length} {group.items.length === 1 ? 'movimiento' : 'movimientos'}
                              </span>
                            </span>
                          </div>
                          */}
                          {group.items.map((a, i) => {
                            const date = new Date(a.timestamp)
                            const valid = !isNaN(date.getTime())
                            const st = ACTIVITY_STYLE[a.type] || ACTIVITY_FALLBACK
                            const Icon = st.icon
                            const isNote = a.type === 'company_note'
                            return (
                              <Fragment key={a.event_id ? `note-${a.event_id}` : `act-${actPage}-${group.key}-${i}`}>
                                <div className={`${styles.timelineItem} ${isNote ? styles.noteTimelineItem : ''} ${isNote && a.pinned ? styles.isPinnedRow : ''}`}>
                                  {isNote ? (
                                    <div className={styles.noteAvatarNode}>
                                      <Avatar name={a.user || 'Sistema'} size="sm" />
                                    </div>
                                  ) : (
                                    <div className={styles.timelineNode} style={{ color: st.color }}>
                                      <Icon size={15} />
                                    </div>
                                  )}
                                  <div className={`${styles.timelineCard} ${isNote ? styles.isNote : ''} ${isNote && a.pinned ? styles.isPinnedCard : ''}`}>
                                    {isNote ? (
                                      <>
                                        <div className={styles.noteHeader}>
                                          <strong className={styles.noteUser}>{a.user || 'Sistema'}</strong>
                                          <span className={styles.noteDateDivider}>•</span>
                                          <span className={styles.noteDateTime}>
                                            {valid
                                              ? `${date.toLocaleDateString('es-ES', { day: '2-digit', month: '2-digit', year: 'numeric' })} ${date.toLocaleTimeString('es-ES', { hour: '2-digit', minute: '2-digit' })}`
                                              : '—'}
                                          </span>
                                          {a.pinned && (
                                            <span className={styles.notePinText}>
                                              <Pin size={11} /> fijada
                                            </span>
                                          )}
                                          {a.edited_at && <span className={styles.noteEdited}>· editada</span>}
                                          {canAnnotate && (
                                            <div className={styles.timelineActions} style={{ marginLeft: 'auto', display: 'flex', gap: 6 }}>
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
                                      </>
                                    ) : (
                                      <div style={{ flex: 1, minWidth: 0 }}>
                                        <p className={styles.timelineText}>{a.details}</p>
                                        <div className={styles.timelineMeta}>
                                          <span className={styles.timelineTag} style={{ color: st.color }}>
                                            {ACTIVITY_LABEL[a.type] || a.type}
                                          </span>
                                          {/* En un contacto, el canal es la mitad de la
                                          información: no es lo mismo haberles
                                          escrito que haberles llamado. */}
                                          {a.channel && CONTACT_STYLE[a.channel] && (
                                            <span className={styles.timelineTag} style={{ color: st.color }}>
                                              {(() => { const CI = CONTACT_STYLE[a.channel!].icon; return <CI size={11} /> })()}
                                              {CONTACT_STYLE[a.channel].label}
                                            </span>
                                          )}
                                          <strong style={{ color: '#64748b', fontWeight: 600 }}>{a.user || 'Sistema'}</strong>
                                          {/* La fecha vive en la cabecera del día: aquí
                                          basta la hora. */}
                                          <span className={styles.timelineTime}>
                                            {valid ? date.toLocaleTimeString('es-ES', { hour: '2-digit', minute: '2-digit' }) : '—'}
                                          </span>
                                        </div>
                                      </div>
                                    )}

                                    {/* Comentarios y archivos. Solo cuelgan de las
                                    entradas que existen como registro: las
                                    derivadas (jornadas, altas, gestiones de CS)
                                    llegan con event_id 0 y no hay a qué atarlas. */}
                                    {a.event_id > 0 && (
                                      <EventThread
                                        key={`thread-${a.event_id}`}
                                        tenantId={tenantId}
                                        eventId={a.event_id}
                                        thread={threads[a.event_id]}
                                        canEdit={canAnnotate}
                                        addComment={addComment}
                                        updateComment={updateComment}
                                        deleteComment={deleteComment}
                                        addAttachment={addAttachment}
                                        deleteAttachment={deleteAttachment}
                                      />
                                    )}
                                  </div>
                                </div>
                                {isNote && a.event_id === lastPinnedId && (
                                  <div style={{ borderBottom: '1px dashed var(--glass-border, #e2e8f0)', margin: '12px 12px 12px 58px' }} />
                                )}
                              </Fragment>
                            )
                          })}
                        </div>
                      ))}
                    </div>
                  </div>

                  {actCategory !== 'note' && (
                    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', padding: '14px 4px' }}>
                      <span style={{ fontSize: '13px', color: '#64748b' }}>
                        Mostrando {(actPage - 1) * actPageSize + 1}–{Math.min(actPage * actPageSize, actTotal)} de {actTotal} movimientos
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
                  )}
                </>
              )}
            </>
          )}

          {/* Actividad de la empresa: quién no está registrando horas y qué
          ausencias hay este mes, con la misma gestión y las mismas vías de
          contacto que en el panel. Son los dos paneles compartidos, sin la
          columna ni el filtro de empresa, que aquí sobran. */}
          {tab === 'actividad' && (
            <>

              {actSubTab === 'inactividad' ? (
                <TeamActivityPanel
                  items={teamInactive}
                  loading={teamInactiveLoading}
                  followUps={inactivityFollowUps.followUps}
                  onSetFollowUp={inactivityFollowUps.setFollowUp}
                  onCompose={(recipient, body) => setProComposer({ recipient, body })}
                  onOpenUser={(userId, sequence) => {
                    setRecordNav(`tenant-employees:${tenantId}`, sequence)
                    navigate(`/admin/tenants/${tenantId}/employees/${userId}`)
                  }}
                  showCompany={false}
                  description="Profesionales de esta empresa sin registrar horas. Los de 2+ días disparan alerta automática al equipo de customer success."
                />
              ) : (
                <AbsenceReportPanel
                  items={tenantAbsence?.items || []}
                  followUps={absenceFollowUps.followUps}
                  onSetFollowUp={absenceFollowUps.setFollowUp}
                  onOpenUser={(userId, sequence) => {
                    setRecordNav(`tenant-employees:${tenantId}`, sequence)
                    navigate(`/admin/tenants/${tenantId}/employees/${userId}`)
                  }}
                  showCompany={false}
                  description="Ausencias de esta empresa este mes, agrupadas por profesional. Haz clic en una fila para ver el detalle."
                />
              )}
            </>
          )}

          {tab === 'archivados' && (
            archivedLoading ? (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '0.6rem' }}>
                {Array.from({ length: 4 }).map((_, i) => <Skeleton key={i} height={56} radius={12} />)}
              </div>
            ) : archivedError ? (
              <p className={styles.errorMsg}>No se pudieron cargar los archivados de esta empresa.</p>
            ) : (
              <ArchivedList entries={archived} showCompany={false} />
            )
          )}

          {tab === 'horarios' && canAnnotate && (
            <>
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'wrap', marginBottom: 20 }}>
                <div className={styles.searchBox} style={{ margin: 0 }}>
                  <Search size={18} />
                  <input
                    type="text"
                    placeholder="Buscar profesional"
                    value={schedSearch}
                    onChange={(e) => {
                      setSchedSearch(e.target.value)
                      setSchedPage(1)
                    }}
                  />
                </div>
              </div>

              <div className={styles.tableWrap}>
                <table className={styles.table}>
                  <thead>
                    <tr>
                      <th>Profesional</th>
                      <th>Jornada</th>
                      <th>Días</th>
                      <th>Horario</th>
                      <th>Acción</th>
                    </tr>
                  </thead>
                  <tbody>
                    {schedPaginated.map(emp => (
                      <tr key={emp.id} className={styles.row}>
                        <td>
                          <div className={styles.companyCell}>
                            <Avatar src={emp.avatar} name={emp.name} size="sm" />
                            <div className={styles.ownerCell}>
                              <span>{emp.name}</span>
                              <small>{emp.email}</small>
                            </div>
                          </div>
                        </td>
                        <td>{emp.schedule_type || <span style={{ color: '#94a3b8' }}>—</span>}</td>
                        <td>{emp.schedule_days || <span style={{ color: '#94a3b8' }}>—</span>}</td>
                        <td>
                          {emp.schedule_start_time ? (
                            `${formatTimeToAMPM(emp.schedule_start_time)} - ${formatTimeToAMPM(emp.schedule_end_time)}`
                          ) : (
                            <span style={{ color: '#94a3b8' }}>—</span>
                          )}
                        </td>
                        <td>
                          {emp.schedule_type ? (
                            <div style={{ display: 'flex', gap: '8px' }}>
                              <button
                                type="button"
                                onClick={() => setScheduleViewEmployee(emp)}
                                style={{
                                  display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
                                  width: '36px', height: '36px',
                                  border: '1px solid #e2e8f0', borderRadius: '10px',
                                  background: '#f8fafc', color: '#10b981',
                                  cursor: 'pointer', transition: 'all 0.15s ease',
                                }}
                                onMouseEnter={e => (e.currentTarget.style.background = '#e2e8f0')}
                                onMouseLeave={e => (e.currentTarget.style.background = '#f8fafc')}
                                title="Ver Horario"
                              >
                                <Eye size={18} />
                              </button>
                              <button
                                type="button"
                                onClick={() => setScheduleModalEmployee(emp)}
                                style={{
                                  display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
                                  width: '36px', height: '36px',
                                  border: '1px solid #e2e8f0', borderRadius: '10px',
                                  background: '#f8fafc', color: '#f59e0b',
                                  cursor: 'pointer', transition: 'all 0.15s ease',
                                }}
                                onMouseEnter={e => (e.currentTarget.style.background = '#e2e8f0')}
                                onMouseLeave={e => (e.currentTarget.style.background = '#f8fafc')}
                                title="Editar Horario"
                              >
                                <Pencil size={18} />
                              </button>
                            </div>
                          ) : (
                            <button
                              type="button"
                              onClick={() => setScheduleModalEmployee(emp)}
                              style={{
                                display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
                                width: '36px', height: '36px',
                                border: '1px solid #e2e8f0', borderRadius: '10px',
                                background: '#f8fafc', color: '#3b82f6',
                                cursor: 'pointer', transition: 'all 0.15s ease',
                              }}
                              onMouseEnter={e => (e.currentTarget.style.background = '#e2e8f0')}
                              onMouseLeave={e => (e.currentTarget.style.background = '#f8fafc')}
                              title="Configurar Horario"
                            >
                              <Settings size={18} />
                            </button>
                          )}
                        </td>
                      </tr>
                    ))}
                    {schedFiltered.length === 0 && (
                      <tr>
                        <td colSpan={5} style={{ textAlign: 'center', padding: '32px 0', color: '#64748b' }}>
                          No se encontraron profesionales que coincidan con la búsqueda
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
                {schedTotalPages > 1 && (
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', padding: '14px 16px' }}>
                    <span style={{ fontSize: '13px', color: '#64748b' }}>
                      Mostrando {(schedCurrentPage - 1) * 5 + 1}–{Math.min(schedCurrentPage * 5, schedFiltered.length)} de {schedFiltered.length} profesionales
                    </span>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <button
                        type="button"
                        className={styles.iconBtn}
                        onClick={() => setSchedPage(p => Math.max(1, p - 1))}
                        disabled={schedCurrentPage <= 1}
                        style={{ opacity: schedCurrentPage <= 1 ? 0.4 : 1, cursor: schedCurrentPage <= 1 ? 'not-allowed' : 'pointer' }}
                        title="Página anterior"
                      >
                        <ChevronLeft size={16} />
                      </button>
                      <span style={{ fontSize: '13px', fontWeight: 600, color: '#334155', whiteSpace: 'nowrap' }}>
                        Página {schedCurrentPage} de {schedTotalPages}
                      </span>
                      <button
                        type="button"
                        className={styles.iconBtn}
                        onClick={() => setSchedPage(p => Math.min(schedTotalPages, p + 1))}
                        disabled={schedCurrentPage >= schedTotalPages}
                        style={{ opacity: schedCurrentPage >= schedTotalPages ? 0.4 : 1, cursor: schedCurrentPage >= schedTotalPages ? 'not-allowed' : 'pointer' }}
                        title="Página siguiente"
                      >
                        <ChevronRight size={16} />
                      </button>
                    </div>
                  </div>
                )}
              </div>
            </>
          )}

          {/* Tickets de la empresa: las alertas que genera la plataforma sobre su
          gente y las conversaciones de WhatsApp de su número, juntas. Cada fila
          lleva a su detalle real, que es donde se trabaja el ticket. */}
          {tab === 'tickets' && (
            ticketsLoading ? (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '0.6rem' }}>
                {Array.from({ length: 4 }).map((_, i) => <Skeleton key={i} height={56} radius={12} />)}
              </div>
            ) : ticketsError ? (
              <p className={styles.errorMsg}>No se pudieron cargar los tickets de esta empresa.</p>
            ) : tickets.length === 0 ? (
              <div className={styles.empty}>
                <Inbox size={40} />
                <p>Esta empresa no tiene tickets</p>
                <span style={{ fontSize: 13, color: '#94a3b8', maxWidth: 460, textAlign: 'center', marginTop: 6 }}>
                  Aparecerán aquí las alertas de la plataforma sobre sus profesionales
                  y las conversaciones de WhatsApp con {tenant.phone_number?.trim() || 'su teléfono'}.
                </span>
              </div>
            ) : (
              <div className={styles.tableWrap}>
                <table className={styles.table}>
                  <thead>
                    <tr>
                      <th>Ticket</th>
                      <th>Origen</th>
                      <th>Sobre</th>
                      <th>Responsable</th>
                      <th>Últ. movimiento</th>
                      <th>Estado</th>
                      <th></th>
                    </tr>
                  </thead>
                  <tbody>
                    {ticketsSlice.map(tk => {
                      const st = ticketOrigin(tk.origin)
                      const OriginIcon = st.icon
                      const updated = new Date(tk.updated_at)
                      const updatedValid = !isNaN(updated.getTime())
                      const isOpen = tk.status === 'open'
                      return (
                        <tr key={tk.id} className={styles.row} onClick={() => navigate(ticketPath(tk))}>
                          <td>
                            <div className={styles.ownerCell}>
                              <span>{tk.title?.trim() || `Ticket #${tk.id}`}</span>
                              <small>#{tk.id}{tk.stage ? ` · ${TICKET_STAGE[tk.stage] || tk.stage}` : ''}</small>
                            </div>
                          </td>
                          <td>
                            <span className={styles.timelineTag} style={{ color: st.color }}>
                              <OriginIcon size={13} /> {st.label}
                            </span>
                          </td>
                          <td>{tk.about?.trim() || '—'}</td>
                          <td>{tk.assignee?.trim() || <span style={{ color: '#94a3b8' }}>Sin asignar</span>}</td>
                          <td>{updatedValid ? updated.toLocaleDateString('es-ES') : '—'}</td>
                          <td>
                            <span className={`${styles.badge} ${isOpen ? styles.badgeActive : styles.badgeSuspended}`}>
                              {isOpen ? 'Abierto' : 'Cerrado'}
                            </span>
                          </td>
                          <td>
                            <div className={styles.rowActions}>
                              <ChevronRight size={18} className={styles.chevron} />
                            </div>
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
                {ticketTotalPages > 1 && (
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', padding: '14px 4px' }}>
                    <span style={{ fontSize: '13px', color: '#64748b' }}>
                      Mostrando {(ticketPage - 1) * TICKETS_PER_PAGE + 1}–{Math.min(ticketPage * TICKETS_PER_PAGE, tickets.length)} de {tickets.length} tickets
                    </span>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <button
                        type="button"
                        className={styles.iconBtn}
                        onClick={() => setTicketPage(p => Math.max(1, p - 1))}
                        disabled={ticketPage <= 1}
                        style={{ opacity: ticketPage <= 1 ? 0.4 : 1, cursor: ticketPage <= 1 ? 'not-allowed' : 'pointer' }}
                        title="Página anterior"
                      >
                        <ChevronLeft size={16} />
                      </button>
                      <span style={{ fontSize: '13px', color: '#334155', fontWeight: 600 }}>
                        {ticketPage} / {ticketTotalPages}
                      </span>
                      <button
                        type="button"
                        className={styles.iconBtn}
                        onClick={() => setTicketPage(p => Math.min(ticketTotalPages, p + 1))}
                        disabled={ticketPage >= ticketTotalPages}
                        style={{ opacity: ticketPage >= ticketTotalPages ? 0.4 : 1, cursor: ticketPage >= ticketTotalPages ? 'not-allowed' : 'pointer' }}
                        title="Página siguiente"
                      >
                        <ChevronRight size={16} />
                      </button>
                    </div>
                  </div>
                )}
              </div>
            )
          )}

        </div> {/* .rightContentArea */}
      </div> {/* .twoColumnGrid */}

      {/* Los dos correos de esta ficha pasan por el mismo redactor (plantilla de
          Tools o texto nuevo). Se diferencian en a quién van y dónde se
          registra: el del profesional en su ficha, el de la empresa en el
          expediente de la empresa. */}
      <EmailComposerModal
        isOpen={proComposer !== null}
        onClose={() => setProComposer(null)}
        recipient={proComposer?.recipient ?? null}
        defaultBody={proComposer?.body ?? ''}
      />

      <EmailComposerModal
        isOpen={commOpen}
        onClose={() => setCommOpen(false)}
        recipient={tenant.owner_email
          ? { id: tenant.id, name: tenant.owner_name || tenant.company_name, email: tenant.owner_email }
          : null}
        defaultSubject="Contacto oficial Oberstaff"
        // El responsable de una empresa no es un profesional: su contacto va al
        // expediente de la empresa, no a la ficha de una persona. Se registra
        // desde aquí (y no con logTenantId) para que el expediente que está
        // abierto en pantalla se refresque solo.
        logContact={false}
        onSent={(subject) => { logContact('email', subject).catch(() => { }) }}
      />

      <Modal
        isOpen={noteOpen}
        isDirty={noteText.trim() !== '' || noteFiles.length > 0}
        onClose={() => { setNoteOpen(false); setNoteFiles([]) }}
        title={noteEditingId !== null ? 'Editar nota del expediente' : 'Añadir nota al expediente'}
        size="md"
        footer={
          <>
            <Button variant="secondary" onClick={() => { setNoteOpen(false); setNoteFiles([]) }} disabled={noteSaving}>Cancelar</Button>
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
        {/* Arrastrar sobre el modal entero, no solo sobre el textarea: soltar un
            archivo un centímetro al lado y ver cómo el navegador lo abre en otra
            pestaña es la forma más fácil de perder lo escrito. */}
        <div className={styles.field} onDrop={onNoteDrop} onDragOver={e => e.preventDefault()}>
          <label>Nota</label>
          <textarea
            value={noteText}
            onChange={(e) => setNoteText(e.target.value.slice(0, NOTE_MAX_LENGTH))}
            onPaste={onNotePaste}
            placeholder="Ej: Llamada con el responsable — renuevan el plan en septiembre."
            rows={5}
            autoFocus
            style={{ width: '100%', padding: '10px 14px', border: '1px solid #e2e8f0', borderRadius: 'var(--radius)', fontSize: '14px', outline: 'none', resize: 'vertical' }}
          />
          <span style={{ display: 'block', marginTop: 4, textAlign: 'right', fontSize: '12px', color: noteText.length >= NOTE_MAX_LENGTH ? '#dc2626' : '#94a3b8' }}>
            {noteText.length}/{NOTE_MAX_LENGTH}
          </span>

          {/* Los archivos van en cola y se suben al guardar: la entrada tiene
              que existir antes de que nada pueda colgarse de ella. */}
          {noteFiles.length > 0 && (
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginTop: 8 }}>
              {noteFiles.map((f, i) => (
                <span key={`${f.name}-${i}`} style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '4px 8px', borderRadius: 999, background: '#f1f5f9', fontSize: 12, color: '#334155', maxWidth: '100%' }}>
                  <Paperclip size={12} style={{ flexShrink: 0 }} />
                  <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{f.name}</span>
                  <button
                    type="button"
                    onClick={() => setNoteFiles(prev => prev.filter((_, idx) => idx !== i))}
                    disabled={noteSaving}
                    title={`Quitar ${f.name}`}
                    style={{ border: 'none', background: 'none', padding: 0, cursor: 'pointer', color: '#94a3b8', display: 'inline-flex' }}
                  >
                    <X size={12} />
                  </button>
                </span>
              ))}
            </div>
          )}

          <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginTop: 8 }}>
            {/* Los mismos tipos que acepta el servidor (upload_service). Sin
                esto se puede elegir un .txt y el error llega después de haber
                guardado la nota, que es el peor momento para enterarse. */}
            <input
              ref={noteFileRef}
              type="file"
              multiple
              hidden
              accept=".pdf,.doc,.docx,.xls,.xlsx,.jpg,.jpeg,.png,.gif,.webp,.mp3,.wav,.ogg,.webm"
              onChange={(e) => {
                setNoteFiles(prev => [...prev, ...Array.from(e.target.files || [])])
                e.target.value = ''
              }}
            />
            <button
              type="button"
              onClick={() => noteFileRef.current?.click()}
              disabled={noteSaving}
              style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '6px 10px', border: '1px solid #e2e8f0', borderRadius: 8, background: 'transparent', color: '#475569', fontSize: 13, fontWeight: 600, cursor: 'pointer' }}
              title="Adjuntar un archivo a esta nota"
            >
              <Paperclip size={14} /> Adjuntar
            </button>
            <span style={{ fontSize: 12, color: '#94a3b8' }}>pega o arrastra imágenes · PDF, Word, Excel, imagen o audio</span>
          </div>
        </div>
        {noteError && <p className={styles.errorMsg}>{noteError}</p>}
      </Modal>

      <Modal
        isOpen={contactOpen}
        isDirty={contactDetail.trim() !== ''}
        onClose={() => setContactOpen(false)}
        title="Registrar contacto con la empresa"
        size="md"
        footer={
          <>
            <Button variant="secondary" onClick={() => setContactOpen(false)} disabled={contactSaving}>Cancelar</Button>
            <Button onClick={handleSaveContact} loading={contactSaving}>Registrar</Button>
          </>
        }
      >
        <p className={styles.modalHint}>
          Para lo que pasa fuera de la plataforma. Los correos y los WhatsApp que
          salen desde aquí se registran solos: esto es para dejar constancia de
          una llamada o una reunión con {tenant.company_name}.
        </p>
        <div className={styles.field}>
          <label>Vía</label>
          <Select
            fullWidth
            value={contactChannel}
            onChange={v => setContactChannel(String(v) as TenantContactChannel)}
            options={MANUAL_CONTACT_CHANNELS.map(c => ({ value: c.value, label: c.label }))}
          />
        </div>
        <div className={styles.field}>
          <label>Resumen <span style={{ color: '#94a3b8', fontWeight: 400 }}>(opcional)</span></label>
          <textarea
            value={contactDetail}
            onChange={(e) => setContactDetail(e.target.value.slice(0, NOTE_MAX_LENGTH))}
            placeholder="Ej: Repasamos las horas pendientes de aprobar; lo revisan esta semana."
            rows={4}
            autoFocus
            style={{ width: '100%', padding: '10px 14px', border: '1px solid #e2e8f0', borderRadius: 'var(--radius)', fontSize: '14px', outline: 'none', resize: 'vertical' }}
          />
          <span style={{ display: 'block', marginTop: 4, textAlign: 'right', fontSize: '12px', color: contactDetail.length >= NOTE_MAX_LENGTH ? '#dc2626' : '#94a3b8' }}>
            {contactDetail.length}/{NOTE_MAX_LENGTH}
          </span>
        </div>
        {contactError && <p className={styles.errorMsg}>{contactError}</p>}
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
          <label>Alta como cliente</label>
          <DatePicker
            fullWidth
            clearable
            value={editForm.client_since}
            max={toISODate(new Date())}
            onChange={(v) => setEditForm(f => ({ ...f, client_since: v }))}
            ariaLabel="Alta como cliente"
          />
          <small style={{ color: '#94a3b8' }}>
            Desde cuándo es cliente de verdad. Vacío = la fecha en que se creó la cuenta aquí.
          </small>
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

      {/* Vistazo rápido desde la plantilla: se abre al pulsar la fila y deja la
          tabla, sus filtros y su página intactos detrás. */}
      {peekEmployee !== null && (
        <EmployeePeekModal
          employeeId={peekEmployee}
          tenantId={tenantId}
          canManage={canManage}
          onClose={() => setPeekEmployee(null)}
          onOpenFull={seedEmployeeNav}
        />
      )}

      <EmployeeScheduleModal
        tenantId={tenantId}
        employee={scheduleModalEmployee}
        onClose={() => setScheduleModalEmployee(null)}
        onSaved={refresh}
      />

      {/* Modal de solo lectura: ver el horario de un profesional */}
      <Modal
        isOpen={!!scheduleViewEmployee}
        onClose={() => setScheduleViewEmployee(null)}
        title={`Horario — ${scheduleViewEmployee?.name ?? ''}`}
        size="sm"
        footer={
          <>
            <Button variant="secondary" onClick={() => setScheduleViewEmployee(null)}>Cerrar</Button>
            <Button onClick={() => {
              setScheduleModalEmployee(scheduleViewEmployee)
              setScheduleViewEmployee(null)
            }}>Editar Horario</Button>
          </>
        }
      >
        {scheduleViewEmployee && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '14px', padding: '14px 16px', background: 'var(--surface-2, #f8fafc)', borderRadius: '10px' }}>
              <div style={{ width: '36px', height: '36px', borderRadius: '9px', background: 'rgba(139,92,246,0.12)', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
                <Briefcase size={18} color="#7c3aed" />
              </div>
              <div>
                <div style={{ fontSize: '11px', fontWeight: 600, color: '#94a3b8', textTransform: 'uppercase', letterSpacing: '0.06em', marginBottom: '3px' }}>Tipo de Jornada</div>
                <div style={{ fontSize: '15px', fontWeight: 600, color: 'var(--text-primary, #1e293b)' }}>{scheduleViewEmployee.schedule_type || '—'}</div>
              </div>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '14px', padding: '14px 16px', background: 'var(--surface-2, #f8fafc)', borderRadius: '10px' }}>
              <div style={{ width: '36px', height: '36px', borderRadius: '9px', background: 'rgba(59,130,246,0.12)', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
                <CalendarDays size={18} color="#2563eb" />
              </div>
              <div>
                <div style={{ fontSize: '11px', fontWeight: 600, color: '#94a3b8', textTransform: 'uppercase', letterSpacing: '0.06em', marginBottom: '3px' }}>Días Laborales</div>
                <div style={{ fontSize: '15px', fontWeight: 600, color: 'var(--text-primary, #1e293b)' }}>{scheduleViewEmployee.schedule_days || '—'}</div>
              </div>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '14px', padding: '14px 16px', background: 'var(--surface-2, #f8fafc)', borderRadius: '10px' }}>
              <div style={{ width: '36px', height: '36px', borderRadius: '9px', background: 'rgba(16,185,129,0.12)', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
                <Clock size={18} color="#059669" />
              </div>
              <div>
                <div style={{ fontSize: '11px', fontWeight: 600, color: '#94a3b8', textTransform: 'uppercase', letterSpacing: '0.06em', marginBottom: '3px' }}>Horario</div>
                <div style={{ fontSize: '15px', fontWeight: 600, color: 'var(--text-primary, #1e293b)' }}>
                  {scheduleViewEmployee.schedule_start_time
                    ? `${formatTimeToAMPM(scheduleViewEmployee.schedule_start_time)} — ${formatTimeToAMPM(scheduleViewEmployee.schedule_end_time)}`
                    : '—'}
                </div>
              </div>
            </div>
          </div>
        )}
      </Modal>
    </div>
  )
}
