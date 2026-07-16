import { useState, useMemo, useRef, useEffect } from 'react'
import { useAuth } from '../context/AuthContext'
import { useNotification } from '../context/NotificationContext'
import type { WorkHour } from '../types'
import { useWorkHours } from '../hooks'
import {
  Clock,
  ChevronLeft,
  ChevronRight,
  FileText,
  Download,
  Mail,
  Loader2,
  AlertCircle,
  CheckCircle2
} from 'lucide-react'
import Tooltip from '../components/Common/Tooltip'

import { Select } from '../components/ui/Select'
import { Skeleton } from '../components/ui'
import { adminService, userService } from '../services/api'
import { MONTHS_ES, parseLocalDate } from '../components/WorkHours/utils'
import { WorkHourCalendar } from '../components/WorkHours/WorkHourCalendar'
import { WorkHourStats } from '../components/WorkHours/WorkHourStats'
import { WorkHourList } from '../components/WorkHours/WorkHourList'
import { RegisterDayModal } from '../components/WorkHours/Modals/RegisterDayModal'
import { RecoverHoursModal } from '../components/WorkHours/Modals/RecoverHoursModal'
import { WorkHourDetailModal } from '../components/WorkHours/Modals/WorkHourDetailModal'
import { MissingHoursModal } from '../components/WorkHours/Modals/MissingHoursModal'
import api from '../services/client'
import { htmlToText } from '../utils/sanitize'

import styles from './WorkHours.module.css'

interface CompanyOption { id: number; company_name: string }
interface EmployeeOption { id: number; name: string }

export default function WorkHours() {
  const { user } = useAuth()
  const notify = useNotification()
  const isSuperadmin = !!user?.is_superadmin
  const isEmployerAccount = user?.user_type === 'empleador'

  // Superadmin scope: company (tenant) + optional employee. Persisted across reloads.
  const [companies, setCompanies] = useState<CompanyOption[]>([])
  const [employees, setEmployees] = useState<EmployeeOption[]>([])
  const [selectedCompanyId, setSelectedCompanyIdState] = useState<number | null>(() => {
    const stored = localStorage.getItem('preferred_company_id')
    return stored ? Number(stored) : null
  })
  const [selectedEmployeeId, setSelectedEmployeeId] = useState<number | null>(null)

  const setSelectedCompanyId = (id: number | null) => {
    setSelectedCompanyIdState(id)
    setSelectedEmployeeId(null)
    if (id) {
      localStorage.setItem('preferred_company_id', String(id))
    } else {
      localStorage.removeItem('preferred_company_id')
    }
  }

  // Load company list (superadmin only)
  useEffect(() => {
    if (!isSuperadmin) return
    let active = true
    adminService.getTenants()
      .then((res: any) => {
        if (!active) return
        setCompanies((res || []).map((t: any) => ({
          id: t.id,
          company_name: t.company_name || t.owner_name || `Empresa ${t.id}`,
        })))
      })
      .catch((err) => console.error('Error fetching companies:', err))
    return () => { active = false }
  }, [isSuperadmin])

  // Load employees of the selected company (superadmin only)
  useEffect(() => {
    if (!isSuperadmin || !selectedCompanyId) {
      setEmployees([])
      return
    }
    let active = true
    adminService.getTenantEmployees(selectedCompanyId)
      .then((res: any) => {
        if (!active) return
        setEmployees((res || []).map((e: any) => ({ id: e.id, name: e.name || e.email })))
      })
      .catch((err) => console.error('Error fetching employees:', err))
    return () => { active = false }
  }, [isSuperadmin, selectedCompanyId])

  // Load the employer's own professionals (para filtrar sus horas por persona).
  useEffect(() => {
    if (!isEmployerAccount) return
    let active = true
    userService.getEmployees()
      .then((res: any) => {
        if (!active) return
        setEmployees((res || []).map((e: any) => ({ id: e.id, name: e.name || e.email })))
      })
      .catch((err) => console.error('Error fetching employees:', err))
    return () => { active = false }
  }, [isEmployerAccount])

  // Opciones del filtro por profesional, ordenadas alfabéticamente (es-419).
  const employeeOptions = useMemo(
    () =>
      [...employees]
        .sort((a, b) => a.name.localeCompare(b.name, 'es', { sensitivity: 'base' }))
        .map((e) => ({ value: e.id, label: e.name })),
    [employees],
  )

  const {
    workHours,
    summary,
    isLoading,
    selectedDate,
    setSelectedDate,
    currentMonth,
    currentYear,
    setCurrentMonth,
    setCurrentYear,
    formData,
    setFormData,
    resetForm,
    createWorkHour,
    approveWorkHours,
    approveSingle,
    rejectSingle,
    filteredHours,
    pendingForSelectedDate,
    weekHours,
    todayWork,
    canApprove,
    canEditHours,
  } = useWorkHours(user, {
    companyId: isSuperadmin ? selectedCompanyId : null,
    employeeId: isSuperadmin || isEmployerAccount ? selectedEmployeeId : null,
  })

  const [showModal, setShowModal] = useState(false)
  const [showRecoverModal, setShowRecoverModal] = useState(false)
  const [selectedWorkHour, setSelectedWorkHour] = useState<WorkHour | null>(null)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [showMissingModal, setShowMissingModal] = useState(false)
  const [isMailing, setIsMailing] = useState(false)
  const [mailSuccess, setMailSuccess] = useState<string | null>(null)
  const [mailError, setMailError] = useState<string | null>(null)
  const [isSavingWorkHour, setIsSavingWorkHour] = useState(false)

  const isEmployer = user?.user_type === 'empleador' || user?.is_superadmin

  const actionBtnsRef = useRef<HTMLDivElement>(null)
  const [showBtnLeft, setShowBtnLeft] = useState(false)
  const [showBtnRight, setShowBtnRight] = useState(false)

  const checkBtnScroll = () => {
    if (actionBtnsRef.current) {
      const { scrollLeft, scrollWidth, clientWidth } = actionBtnsRef.current
      setShowBtnLeft(scrollLeft > 5)
      setShowBtnRight(scrollLeft < scrollWidth - clientWidth - 5)
    }
  }

  useEffect(() => {
    const el = actionBtnsRef.current
    if (el) {
      checkBtnScroll()
      el.addEventListener('scroll', checkBtnScroll)
      window.addEventListener('resize', checkBtnScroll)
      const timer = setTimeout(checkBtnScroll, 100)
      return () => {
        el.removeEventListener('scroll', checkBtnScroll)
        window.removeEventListener('resize', checkBtnScroll)
        clearTimeout(timer)
      }
    }
  }, [isEmployer])

  const scrollBtnsLeft = () => actionBtnsRef.current?.scrollBy({ left: -160, behavior: 'smooth' })
  const scrollBtnsRight = () => actionBtnsRef.current?.scrollBy({ left: 160, behavior: 'smooth' })


  const employerTodayActiveCount = useMemo(() => {
    const todayStr = new Date().toISOString().split('T')[0]
    const uniqueUsers = new Set(
      workHours
        .filter(wh => wh.work_date.startsWith(todayStr))
        .map(wh => wh.user_id)
    )
    return uniqueUsers.size
  }, [workHours])

  const absencesCount = useMemo(() => {
    return workHours.filter(wh => {
      const d = parseLocalDate(wh.work_date)
      return wh.work_type === 'absence' && d.getMonth() === currentMonth && d.getFullYear() === currentYear
    }).length
  }, [workHours, currentMonth, currentYear])

  const totalAbsenceHoursToRecover = useMemo(() => {
    // Filter only current month entries
    const monthHours = workHours.filter(wh => {
      const d = parseLocalDate(wh.work_date)
      return d.getMonth() === currentMonth && d.getFullYear() === currentYear
    })

    const absences = monthHours
      .filter(wh => wh.work_type === 'absence')
      .reduce((sum, wh) => sum + (wh.absence_hours || (8 - wh.hours_worked) || 0), 0)
    
    const recovered = monthHours
      .filter(wh => wh.work_type === 'recover')
      .reduce((sum, wh) => sum + (wh.hours_worked || 0), 0)
      
    return Math.max(0, absences - recovered)
  }, [workHours, currentMonth, currentYear])

  // Filas para los reportes: solo el mes/año visible (la ventana traída puede
  // incluir la semana actual de otro mes). Así el documento coincide con su
  // título "Período: <mes> <año>".
  const exportRows = useMemo(() => {
    return workHours.filter(wh => {
      const d = parseLocalDate(wh.work_date)
      return d.getMonth() === currentMonth && d.getFullYear() === currentYear
    })
  }, [workHours, currentMonth, currentYear])

  const handleDownloadPDF = () => {
    const printWindow = window.open('', '_blank')
    if (!printWindow) return
    const monthName = MONTHS_ES[currentMonth]
    const html = `
      <!DOCTYPE html>
      <html>
        <head>
          <title>Reporte de Jornadas - Obertrack</title>
          <style>
            @import url('https://fonts.googleapis.com/css2?family=Plus+Jakarta+Sans:wght@400;500;600;700;800&display=swap');
            body {
              font-family: 'Plus Jakarta Sans', sans-serif;
              padding: 40px;
              color: #334155;
              background-color: #ffffff;
            }
            .header {
              display: flex;
              justify-content: space-between;
              align-items: center;
              border-bottom: 2px solid #f1f5f9;
              padding-bottom: 24px;
              margin-bottom: 32px;
            }
            .logo {
              font-size: 22px;
              font-weight: 800;
              color: #1e1b4b;
              letter-spacing: -0.025em;
            }
            .title {
              font-size: 20px;
              font-weight: 700;
              color: #4f46e5;
              margin-top: 4px;
            }
            .meta-info {
              font-size: 13px;
              text-align: right;
              line-height: 1.6;
              color: #64748b;
            }
            .stats-grid {
              display: flex;
              gap: 16px;
              margin-bottom: 32px;
            }
            .stat-card {
              flex: 1;
              background: #f8fafc;
              border: 1px solid #e2e8f0;
              border-radius: 12px;
              padding: 18px;
              text-align: center;
            }
            .stat-label {
              font-size: 11px;
              font-weight: 700;
              color: #64748b;
              text-transform: uppercase;
              letter-spacing: 0.05em;
              margin-bottom: 6px;
              display: block;
            }
            .stat-val {
              font-size: 22px;
              font-weight: 800;
              color: #0f172a;
            }
            table {
              width: 100%;
              border-collapse: collapse;
              margin-top: 24px;
            }
            th {
              background-color: #f8fafc;
              border-bottom: 2px solid #e2e8f0;
              padding: 12px 10px;
              text-align: left;
              font-size: 11px;
              font-weight: 700;
              color: #64748b;
              text-transform: uppercase;
            }
            td {
              border-bottom: 1px solid #f1f5f9;
              padding: 12px 10px;
              font-size: 13px;
              color: #334155;
            }
            .badge {
              display: inline-block;
              padding: 2px 8px;
              border-radius: 9999px;
              font-size: 10px;
              font-weight: 600;
              text-transform: uppercase;
            }
            .badge.complete {
              background-color: #dcfce7;
              color: #15803d;
            }
            .badge.absence {
              background-color: #fee2e2;
              color: #b91c1c;
            }
            @media print {
              body { padding: 20px; }
              @page { size: auto; margin: 20mm; }
            }
          </style>
        </head>
        <body>
          <div class="header">
            <div>
              <div class="logo">OBERTRACK</div>
              <div class="title">Reporte Mensual de Jornadas</div>
              <div style="font-size: 13px; color: #64748b; margin-top: 4px;">Empresa: ${user?.company_name || user?.name || ''}</div>
            </div>
            <div class="meta-info">
              <div>Generado el: ${new Date().toLocaleDateString('es-ES')}</div>
              <div>Período: ${monthName} ${currentYear}</div>
            </div>
          </div>
          
          <div class="stats-grid">
            <div class="stat-card">
              <span class="stat-label">Horas Totales</span>
              <div class="stat-val" style="color: #4f46e5;">${summary.total_hours.toFixed(1)}h</div>
            </div>
            <div class="stat-card">
              <span class="stat-label">Horas Aprobadas</span>
              <div class="stat-val" style="color: #10b981;">${summary.approved_hours.toFixed(1)}h</div>
            </div>
            <div class="stat-card">
              <span class="stat-label">Horas Pendientes</span>
              <div class="stat-val" style="color: #64748b;">${summary.pending_hours.toFixed(1)}h</div>
            </div>
          </div>

          <table>
            <thead>
              <tr>
                <th>Fecha</th>
                <th>Profesional</th>
                <th>Tipo</th>
                <th>Horas Trab.</th>
                <th>Detalles / Actividades</th>
              </tr>
            </thead>
            <tbody>
              ${exportRows.map(wh => `
                <tr>
                  <td style="white-space: nowrap;">${parseLocalDate(wh.work_date).toLocaleDateString('es-ES')}</td>
                  <td><strong>${wh.user?.name || ''}</strong></td>
                  <td><span class="badge ${wh.work_type}">${wh.work_type === 'complete' ? 'Completo' : wh.work_type === 'absence' ? 'Ausencia' : 'Recuperación'}</span></td>
                  <td><strong>${wh.hours_worked}h</strong></td>
                  <td style="color: #64748b;">${wh.work_type === 'complete' ? wh.activities || '-' : wh.work_type === 'absence' ? `Ausencia: ${wh.absence_hours}h (${wh.absence_reason || ''})` : 'Recuperación de horas'}</td>
                </tr>
              `).join('')}
            </tbody>
          </table>
          <script>
            window.onload = () => {
              setTimeout(() => {
                window.print();
                window.close();
              }, 500);
            }
          </script>
        </body>
      </html>
    `
    printWindow.document.write(html)
    printWindow.document.close()
  }

  const handleDownloadExcel = () => {
    const headers = ['Fecha', 'Profesional', 'Email', 'Tipo de Registro', 'Horas Trabajadas', 'Horas Faltantes/Ausencia', 'Motivo de Ausencia', 'Actividades', 'Aprobado']
    const rows = exportRows.map(wh => [
      parseLocalDate(wh.work_date).toLocaleDateString('es-ES'),
      wh.user?.name || '',
      wh.user?.email || '',
      wh.work_type === 'complete' ? 'Completo' : wh.work_type === 'recover' ? 'Recuperación' : 'Ausencia',
      wh.hours_worked,
      wh.work_type === 'absence' ? wh.absence_hours : 0,
      wh.absence_reason || '',
      wh.activities || '',
      wh.approved ? 'Sí' : 'No'
    ])
    const csvContent = "\uFEFF" + [headers.join('\t'), ...rows.map(r => r.map(val => `"${String(val).replace(/"/g, '""')}"`).join('\t'))].join('\n')
    const blob = new Blob([csvContent], { type: 'application/vnd.ms-excel;charset=utf-8;' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.setAttribute('href', url)
    link.setAttribute('download', `Reporte_Jornadas_${currentYear}_${currentMonth + 1}.xls`)
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
  }

  const handleSendEmail = async () => {
    setIsMailing(true)
    setMailSuccess(null)
    setMailError(null)
    try {
      await api.post('/work-hours/send-report', {
        month: currentMonth + 1,
        year: currentYear
      }, {
        params: isSuperadmin && selectedCompanyId ? { company_id: selectedCompanyId } : undefined
      })
      setMailSuccess('¡Reporte enviado exitosamente a tu correo!')
      setTimeout(() => setMailSuccess(null), 5000)
    } catch (error: any) {
      console.error('Error sending report email:', error)
      setMailError(error.response?.data?.error || 'Error al enviar el reporte por correo')
      setTimeout(() => setMailError(null), 5000)
    } finally {
      setIsMailing(false)
    }
  }

  const today = new Date().toISOString().split('T')[0]

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!htmlToText(formData.activities)) {
      notify.error('Describe al menos una actividad antes de registrar tu jornada.')
      return
    }

    setIsSavingWorkHour(true)
    try {
      if (editingId) {
        await createWorkHour({
          ...formData,
          id: editingId
        } as any)
      } else {
        await createWorkHour({
          work_date: formData.work_date,
          work_type: formData.work_type,
          activities: formData.activities,
          absence_reason: formData.absence_reason,
          absence_hours: formData.absence_hours,
          comments: formData.comments,
        })
      }
      setShowModal(false)
      setEditingId(null)
      resetForm()
      notify.success(editingId ? 'Jornada actualizada.' : 'Jornada registrada.')
    } catch (error: any) {
      console.error('Error saving work hour:', error)
      const message =
        error?.response?.data?.error ||
        error?.response?.data?.message ||
        error?.message ||
        'No se pudo guardar el registro. Intenta nuevamente.'
      notify.error(message)
    } finally {
      setIsSavingWorkHour(false)
    }
  }



  const handleEdit = (wh: WorkHour) => {
    setFormData({
      work_date: wh.work_date.split('T')[0],
      work_type: wh.work_type,
      activities: wh.activities || '',
      absence_reason: wh.absence_reason || '',
      absence_hours: wh.absence_hours || 0,
      comments: wh.comments || '',
    })
    setEditingId(wh.id)
    setShowModal(true)
    setSelectedWorkHour(null)
  }

  const errorMessage = (error: any, fallback: string) =>
    error?.response?.data?.error || error?.response?.data?.message || error?.message || fallback

  // Aprobar/rechazar individuales: el toast da el feedback (incl. el 403 cuando
  // un manager intenta aprobar sus propias horas). Se re-lanza para que el modal
  // permanezca abierto en caso de error.
  const handleApproveSingle = async (id: number) => {
    try {
      await approveSingle(id)
      notify.success('Jornada aprobada.')
    } catch (error: any) {
      notify.error(errorMessage(error, 'No se pudo aprobar la jornada.'))
      throw error
    }
  }

  const handleRejectSingle = async (id: number, reason: string) => {
    try {
      await rejectSingle(id, reason)
      notify.success('Jornada rechazada.')
    } catch (error: any) {
      notify.error(errorMessage(error, 'No se pudo rechazar la jornada.'))
      throw error
    }
  }

  const handleBulkApprove = async () => {
    if (pendingForSelectedDate.length === 0) return
    try {
      await approveWorkHours(pendingForSelectedDate.map(wh => wh.id))
      notify.success('Jornadas aprobadas.')
    } catch (error: any) {
      notify.error(errorMessage(error, 'No se pudieron aprobar las jornadas.'))
    }
  }

  const prevMonth = () => {
    if (currentMonth === 0) {
      setCurrentMonth(11)
      setCurrentYear(currentYear - 1)
    } else {
      setCurrentMonth(currentMonth - 1)
    }
  }

  const nextMonth = () => {
    if (currentMonth === 11) {
      setCurrentMonth(0)
      setCurrentYear(currentYear + 1)
    } else {
      setCurrentMonth(currentMonth + 1)
    }
  }

  // Selectores de alcance del header. Superadmin: empresa + profesional (ambos
  // con búsqueda y orden alfabético). El filtro por profesional del EMPLEADOR se
  // renderiza aparte, en la cabecera de "Registros" (ver employeeFilter).
  const scopeSelectors = isSuperadmin ? (
    <div style={{ display: 'flex', gap: '8px', marginTop: '10px', flexWrap: 'wrap' }}>
      <Select
        value={selectedCompanyId ?? ''}
        onChange={(v) => setSelectedCompanyId(v ? Number(v) : null)}
        clearable
        searchable
        placeholder="Seleccione una empresa..."
        options={companies.map(c => ({ value: c.id, label: c.company_name }))}
      />
      {selectedCompanyId && (
        <Select
          value={selectedEmployeeId ?? ''}
          onChange={(v) => setSelectedEmployeeId(v ? Number(v) : null)}
          clearable
          searchable
          placeholder="Todos los Profesionales"
          options={employeeOptions}
        />
      )}
    </div>
  ) : null

  // Filtro por profesional para la EMPRESA, montado en la cabecera de "Registros".
  const employeeFilter = isEmployerAccount ? (
    <div style={{ minWidth: 220 }}>
      <Select
        value={selectedEmployeeId ?? ''}
        onChange={(v) => setSelectedEmployeeId(v ? Number(v) : null)}
        clearable
        searchable
        placeholder="Todos los Profesionales"
        options={employeeOptions}
      />
    </div>
  ) : null

  if (isLoading) {
    return (
      <div className={styles['work-hours-page']}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem', padding: '1.5rem' }}>
          <Skeleton height={56} width={320} radius={12} />
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(150px, 1fr))', gap: '1rem' }}>
            {Array.from({ length: 6 }).map((_, i) => <Skeleton key={i} height={110} radius={16} />)}
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1.5fr', gap: '1rem' }}>
            <Skeleton height={340} radius={16} />
            <Skeleton height={340} radius={16} />
          </div>
        </div>
      </div>
    )
  }

  // Superadmin must pick a company before any work hours are shown, so tenants
  // never get mixed in the same view.
  if (isSuperadmin && !selectedCompanyId) {
    return (
      <div className={styles['work-hours-page']}>
        <div className={styles['wh-page-header']} data-tour="work-hours-header">
          <div className={styles['header-left']}>
            <h1>
              <Clock size={28} style={{ verticalAlign: 'middle', marginRight: '8px' }} /> Registro de Actividades
            </h1>
            <p className={styles['header-subtitle']}>
              Selecciona una empresa para ver y gestionar la jornada de su equipo.
            </p>
            {scopeSelectors}
          </div>
        </div>
        <div className={styles['empty-state']} style={{ textAlign: 'center', padding: '48px 24px', color: '#64748b' }}>
          <Clock size={56} style={{ color: 'var(--primary)', opacity: 0.5, marginBottom: '16px' }} />
          <h2 style={{ fontSize: '22px', fontWeight: 700, color: 'var(--black)', marginBottom: '8px' }}>Selecciona una empresa</h2>
          <p style={{ maxWidth: '420px', margin: '0 auto' }}>
            Elige una empresa para ver sus horas. Luego podés filtrar por un empleado en particular. La información de cada empresa se mantiene aislada.
          </p>
        </div>
      </div>
    )
  }

  return (
    <div className={styles['work-hours-page']}>
      <div className={styles['wh-page-header']} data-tour="work-hours-header">
        <div className={styles['header-left']}>
          <h1>
            <Clock size={28} style={{ verticalAlign: 'middle', marginRight: '8px' }} />{' '}
            {isEmployer ? 'Registro de Actividades de mi Equipo' : 'Mi Jornada'}{' '}
            {isEmployer && (
              <Tooltip content="Jornadas registradas por los profesionales de tu empresa" size={18} />
            )}
          </h1>
          <p className={styles['header-subtitle']}>
            {isEmployer ? 'Visualiza y gestiona la jornada laboral de tu equipo' : 'Registra tu día laboral'}
          </p>
          {scopeSelectors}
        </div>
        {isEmployer ? (
          <div className={styles['action-buttons-container']} data-tour="work-hours-actions">
            {showBtnLeft && (
              <button className={styles['scroll-btn-left']} onClick={scrollBtnsLeft} aria-label="Ver botones anteriores">
                <ChevronLeft size={20} />
              </button>
            )}
            <div className={styles['action-buttons-scroll']} ref={actionBtnsRef}>
              <button
                onClick={handleDownloadPDF}
                style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', backgroundColor: '#f1f5f9', color: '#1e293b', border: '1px solid #cbd5e1', padding: '9px 14px', borderRadius: '10px', fontWeight: '600', cursor: 'pointer', transition: 'all 0.2s', fontSize: '13px', whiteSpace: 'nowrap' }}
              >
                <FileText size={15} /> Descargar PDF
              </button>
              <button
                onClick={handleDownloadExcel}
                style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', backgroundColor: '#f1f5f9', color: '#1e293b', border: '1px solid #cbd5e1', padding: '9px 14px', borderRadius: '10px', fontWeight: '600', cursor: 'pointer', transition: 'all 0.2s', fontSize: '13px', whiteSpace: 'nowrap' }}
              >
                <Download size={15} /> Descargar Excel
              </button>
              <button
                className={styles['btn-primary']}
                onClick={handleSendEmail}
                disabled={isMailing}
                style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '9px 14px', borderRadius: '10px', fontWeight: '600', cursor: 'pointer', transition: 'all 0.2s', fontSize: '13px', whiteSpace: 'nowrap' }}
              >
                {isMailing ? <Loader2 size={15} className="animate-spin" /> : <Mail size={15} />}
                {isMailing ? 'Enviando...' : 'Enviar por Correo'}
              </button>
            </div>
            {showBtnRight && (
              <button className={styles['scroll-btn-right']} onClick={scrollBtnsRight} aria-label="Ver más botones">
                <ChevronRight size={20} />
              </button>
            )}
          </div>
        ) : canEditHours ? (
          <div style={{ display: 'flex', gap: '8px' }} data-tour="work-hours-actions">
            {totalAbsenceHoursToRecover > 0 && (
              <button className={styles['btn-secondary']} onClick={() => setShowRecoverModal(true)}>
                Recuperar horas
              </button>
            )}
            <button className={styles['btn-primary']} onClick={() => {
              setEditingId(null)
              resetForm()
              setShowModal(true)
            }}>
              + Registrar Día
            </button>
          </div>
        ) : (
          <span style={{ fontSize: '13px', color: '#94a3b8', fontWeight: 600, alignSelf: 'center' }}>
            Tu rol tiene acceso de solo lectura en Horas
          </span>
        )}
      </div>

      {mailSuccess && (
        <div style={{ backgroundColor: '#dcfce7', border: '1px solid #10b981', color: '#15803d', padding: '12px 16px', borderRadius: '8px', display: 'flex', alignItems: 'center', gap: '8px', margin: '16px 0', fontSize: '14px', fontWeight: '500' }}>
          <CheckCircle2 size={18} /> {mailSuccess}
        </div>
      )}
      {mailError && (
        <div style={{ backgroundColor: '#fee2e2', border: '1px solid #ef4444', color: '#b91c1c', padding: '12px 16px', borderRadius: '8px', display: 'flex', alignItems: 'center', gap: '8px', margin: '16px 0', fontSize: '14px', fontWeight: '500' }}>
          <AlertCircle size={18} /> {mailError}
        </div>
      )}

      {!isEmployer && totalAbsenceHoursToRecover > 0 && (
        <div data-tour="work-hours-alert" style={{ backgroundColor: '#fffbeb', border: '1px solid #f59e0b', color: '#b45309', padding: '16px 20px', borderRadius: '12px', display: 'flex', alignItems: 'center', gap: '12px', margin: '16px 0', fontSize: '14px', fontWeight: '500', boxShadow: '0 4px 6px -1px rgba(0, 0, 0, 0.05)' }}>
          <AlertCircle size={20} style={{ color: '#d97706', flexShrink: 0 }} />
          <div>
            <strong>Recordatorio de ausencias:</strong> Tienes un total de <strong>{totalAbsenceHoursToRecover.toFixed(1)}h</strong> pendientes de recuperar debido a tus ausencias registradas este mes. Puedes coordinar con tu empresa para recuperarlas en otro momento.
          </div>
        </div>
      )}

      <WorkHourStats 
        todayWork={todayWork}
        weekHours={weekHours}
        summary={summary}
        isEmployer={isEmployer}
        employerTodayActiveCount={employerTodayActiveCount}
        absenceHoursToRecover={totalAbsenceHoursToRecover}
      />

      <div className={styles['work-hours-content']}>
        <div className={styles['calendar-section']} data-tour="work-hours-calendar">
          <div className={styles['calendar-header']}>
            <button className={styles['nav-btn']} onClick={prevMonth}><ChevronLeft size={20} /></button>
            <h3>
              {MONTHS_ES[currentMonth]} {currentYear}{' '}
              {!isEmployer && (
                <Tooltip content="Resumen de jornadas completas y ausencias que haz registrado" size={14} />
              )}
            </h3>
            <button className={styles['nav-btn']} onClick={nextMonth}><ChevronRight size={20} /></button>
          </div>
          <WorkHourCalendar
            workHours={workHours}
            onDayClick={setSelectedDate}
            selectedDate={selectedDate}
            currentMonth={currentMonth}
            currentYear={currentYear}
          />
          <div className={styles['calendar-legend']}>
            <span className={styles['legend-item']}><span className={`${styles['legend-dot']} ${styles['complete']}`}></span>Completa</span>
            <span className={styles['legend-item']}><span className={`${styles['legend-dot']} ${styles['absence']}`}></span>Ausencia</span>
          </div>

          {isEmployer && (
            <div style={{ borderTop: '1px solid #e2e8f0', marginTop: '16px', paddingTop: '16px' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', fontSize: '13px', color: '#475569' }}>
                <span style={{ display: 'flex', alignItems: 'center', gap: '6px', fontWeight: '600' }}>
                  <AlertCircle size={16} style={{ color: '#ef4444' }} /> Total de ausencias este mes: {absencesCount}
                </span>
                <button 
                  className={styles['clear-filter']} 
                  onClick={() => setShowMissingModal(true)}
                  style={{ padding: '4px 8px', margin: 0, textDecoration: 'underline', fontSize: '13px', fontWeight: '600', background: 'none', border: 'none', cursor: 'pointer', color: '#5a52e6' }}
                >
                  Ver horas faltantes
                </button>
              </div>
            </div>
          )}

          {selectedDate && (
            <button className={styles['clear-filter']} onClick={() => setSelectedDate(null)}>
              Ver todos los días
            </button>
          )}
        </div>

        <WorkHourList
          filteredHours={filteredHours}
          selectedDate={selectedDate}
          canApprove={canApprove}
          pendingForSelectedDate={pendingForSelectedDate}
          onBulkApprove={handleBulkApprove}
          onItemClick={setSelectedWorkHour}
          isEmployer={isEmployer}
          filterSlot={employeeFilter}
        />
      </div>

      <RegisterDayModal 
        isOpen={showModal}
        onClose={() => {
          setShowModal(false)
          setEditingId(null)
        }}
        formData={formData}
        setFormData={setFormData}
        onSubmit={handleSubmit}
        today={today}
        isSubmitting={isSavingWorkHour}
      />

      <WorkHourDetailModal
        workHour={selectedWorkHour}
        onClose={() => setSelectedWorkHour(null)}
        canApprove={canApprove}
        canEdit={canEditHours}
        onApprove={handleApproveSingle}
        onReject={handleRejectSingle}
        onEdit={handleEdit}
        isEmployer={isEmployer}
        isOwnRecord={!isSuperadmin && selectedWorkHour?.user_id === user?.id}
      />

      <RecoverHoursModal
        isOpen={showRecoverModal}
        onClose={() => setShowRecoverModal(false)}
        onSubmit={async (date, hours, comments) => {
          await createWorkHour({
            work_date: date,
            work_type: 'recover',
            activities: comments,
            hours_worked: hours,
          } as any)
        }}
        today={today}
        absenceHoursToRecover={totalAbsenceHoursToRecover}
      />

      <MissingHoursModal
        isOpen={showMissingModal}
        onClose={() => setShowMissingModal(false)}
        workHours={workHours}
        currentMonthName={MONTHS_ES[currentMonth]}
        currentMonth={currentMonth}
        currentYear={currentYear}
      />
    </div>
  )
}
