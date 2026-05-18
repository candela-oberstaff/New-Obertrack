import { useState, useMemo } from 'react'
import { useAuth } from '../context/AuthContext'
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

import { MONTHS_ES } from '../components/WorkHours/utils'
import { WorkHourCalendar } from '../components/WorkHours/WorkHourCalendar'
import { WorkHourStats } from '../components/WorkHours/WorkHourStats'
import { WorkHourList } from '../components/WorkHours/WorkHourList'
import { RegisterDayModal } from '../components/WorkHours/Modals/RegisterDayModal'
import { WorkHourDetailModal } from '../components/WorkHours/Modals/WorkHourDetailModal'
import { MissingHoursModal } from '../components/WorkHours/Modals/MissingHoursModal'
import api from '../services/client'

import styles from './WorkHours.module.css'

export default function WorkHours() {
  const { user } = useAuth()
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
    filteredHours,
    pendingForSelectedDate,
    weekHours,
    todayWork,
    canApprove,
  } = useWorkHours(user)

  const [showModal, setShowModal] = useState(false)
  const [selectedWorkHour, setSelectedWorkHour] = useState<WorkHour | null>(null)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [showMissingModal, setShowMissingModal] = useState(false)
  const [isMailing, setIsMailing] = useState(false)
  const [mailSuccess, setMailSuccess] = useState<string | null>(null)
  const [mailError, setMailError] = useState<string | null>(null)

  const isEmployer = user?.user_type === 'empleador' || user?.is_superadmin

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
    return workHours.filter(wh => wh.work_type === 'absence').length
  }, [workHours])

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
              ${workHours.map(wh => `
                <tr>
                  <td style="white-space: nowrap;">${new Date(wh.work_date).toLocaleDateString('es-ES')}</td>
                  <td><strong>${wh.user?.name || ''}</strong></td>
                  <td><span class="badge ${wh.work_type === 'complete' ? 'complete' : 'absence'}">${wh.work_type === 'complete' ? 'Completo' : 'Ausencia'}</span></td>
                  <td><strong>${wh.hours_worked}h</strong></td>
                  <td style="color: #64748b;">${wh.work_type === 'complete' ? wh.activities || '-' : `Ausencia: ${wh.absence_hours}h (${wh.absence_reason || ''})`}</td>
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
    const rows = workHours.map(wh => [
      new Date(wh.work_date).toLocaleDateString('es-ES'),
      wh.user?.name || '',
      wh.user?.email || '',
      wh.work_type === 'complete' ? 'Completo' : 'Ausencia',
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
        })
      }
      setShowModal(false)
      setEditingId(null)
      resetForm()
    } catch (error) {
      console.error('Error saving work hour:', error)
    }
  }

  const handleEdit = (wh: WorkHour) => {
    setFormData({
      work_date: wh.work_date.split('T')[0],
      work_type: wh.work_type,
      activities: wh.activities || '',
      absence_reason: wh.absence_reason || '',
      absence_hours: wh.absence_hours || 0,
    })
    setEditingId(wh.id)
    setShowModal(true)
    setSelectedWorkHour(null)
  }

  const handleBulkApprove = async () => {
    if (pendingForSelectedDate.length === 0) return
    try {
      await approveWorkHours(pendingForSelectedDate.map(wh => wh.id))
    } catch (error) {
      console.error('Error approving work hours:', error)
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

  if (isLoading) {
    return (
      <div className={styles['work-hours-loading']}>
        <div className={styles['spinner']} />
        <p>Cargando...</p>
      </div>
    )
  }

  return (
    <div className={styles['work-hours-page']}>
      <div className={styles['wh-page-header']}>
        <div className={styles['header-left']}>
          <h1>
            <Clock size={28} style={{ verticalAlign: 'middle', marginRight: '8px' }} />{' '}
            {isEmployer ? 'Registro de Actividades de mi Equipo' : 'Mi Jornada'}
          </h1>
          <p className={styles['header-subtitle']}>
            {isEmployer ? 'Visualiza y gestiona la jornada laboral de tu equipo' : 'Registra tu día laboral'}
          </p>
        </div>
        {isEmployer ? (
          <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
            <button 
              className={styles['btn-secondary'] || 'btn-secondary'} 
              onClick={handleDownloadPDF}
              style={{ display: 'inline-flex', alignItems: 'center', gap: '8px', backgroundColor: '#f1f5f9', color: '#1e293b', border: '1px solid #cbd5e1', padding: '10px 16px', borderRadius: '10px', fontWeight: '600', cursor: 'pointer', transition: 'all 0.2s' }}
            >
              <FileText size={16} /> Descargar PDF
            </button>
            <button 
              className={styles['btn-secondary'] || 'btn-secondary'} 
              onClick={handleDownloadExcel}
              style={{ display: 'inline-flex', alignItems: 'center', gap: '8px', backgroundColor: '#f1f5f9', color: '#1e293b', border: '1px solid #cbd5e1', padding: '10px 16px', borderRadius: '10px', fontWeight: '600', cursor: 'pointer', transition: 'all 0.2s' }}
            >
              <Download size={16} /> Descargar Excel
            </button>
            <button 
              className={styles['btn-primary']} 
              onClick={handleSendEmail} 
              disabled={isMailing}
              style={{ display: 'inline-flex', alignItems: 'center', gap: '8px', padding: '10px 16px', borderRadius: '10px', fontWeight: '600', cursor: 'pointer', transition: 'all 0.2s' }}
            >
              {isMailing ? <Loader2 size={16} className="animate-spin" /> : <Mail size={16} />}
              {isMailing ? 'Enviando...' : 'Enviar por Correo'}
            </button>
          </div>
        ) : (
          <button className={styles['btn-primary']} onClick={() => {
            setEditingId(null)
            resetForm()
            setShowModal(true)
          }}>
            + Registrar Día
          </button>
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

      <WorkHourStats 
        todayWork={todayWork}
        weekHours={weekHours}
        summary={summary}
        isEmployer={isEmployer}
        employerTodayActiveCount={employerTodayActiveCount}
      />

      <div className={styles['work-hours-content']}>
        <div className={styles['calendar-section']}>
          <div className={styles['calendar-header']}>
            <button className={styles['nav-btn']} onClick={prevMonth}><ChevronLeft size={20} /></button>
            <h3>{MONTHS_ES[currentMonth]} {currentYear}</h3>
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
      />

      <WorkHourDetailModal 
        workHour={selectedWorkHour}
        onClose={() => setSelectedWorkHour(null)}
        canApprove={canApprove}
        onApprove={approveSingle}
        onEdit={handleEdit}
      />

      <MissingHoursModal
        isOpen={showMissingModal}
        onClose={() => setShowMissingModal(false)}
        workHours={workHours}
        currentMonthName={MONTHS_ES[currentMonth]}
      />
    </div>
  )
}
