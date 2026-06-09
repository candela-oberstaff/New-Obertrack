import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { ArrowLeft, Download, FileText } from 'lucide-react'
import { Ticket, ticketService } from '../../services/ticket.service'
import styles from './Tickets.module.css'

const MONTHS = [
  'Enero', 'Febrero', 'Marzo', 'Abril', 'Mayo', 'Junio',
  'Julio', 'Agosto', 'Septiembre', 'Octubre', 'Noviembre', 'Diciembre',
]

const STAGE_LABEL: Record<string, string> = {
  new: 'Nuevo',
  in_progress: 'En seguimiento',
  waiting: 'En seguimiento',
  closed: 'Resuelto',
}

function professionalName(t: Ticket): string {
  return t.title?.replace(/^Rechazo de horas:\s*/i, '') || '-'
}

export default function RejectionReport() {
  const navigate = useNavigate()
  const now = new Date()
  const [month, setMonth] = useState(now.getMonth() + 1)
  const [year, setYear] = useState(now.getFullYear())

  const { data: items = [], isLoading: loading, error: queryError } = useQuery({
    queryKey: ['rejection-report', month, year],
    queryFn: async (): Promise<Ticket[]> => {
      const data = await ticketService.getRejectionReport(month, year)
      return data.items ?? []
    },
  })
  const error = queryError ? ((queryError as any)?.response?.data?.error ?? 'No se pudo cargar el informe.') : null

  const handleExport = () => {
    const headers = ['Profesional', 'Email', 'Empresa', 'Fechas', 'Motivo', 'Rechazado por', 'Estado', 'Creado']
    const rows = items.map(t => [
      professionalName(t),
      t.professional_email || '',
      t.company_name || '',
      t.work_dates || '',
      t.reason || '',
      t.rejected_by_name || '',
      STAGE_LABEL[t.stage] || t.stage,
      new Date(t.created_at).toLocaleDateString('es-ES'),
    ])
    const csv = '﻿' + [headers.join('\t'), ...rows.map(r => r.map(v => `"${String(v).replace(/"/g, '""')}"`).join('\t'))].join('\n')
    const blob = new Blob([csv], { type: 'application/vnd.ms-excel;charset=utf-8;' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.setAttribute('href', url)
    link.setAttribute('download', `Informe_Rechazos_${year}_${String(month).padStart(2, '0')}.xls`)
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
  }

  const years = [now.getFullYear(), now.getFullYear() - 1, now.getFullYear() - 2]

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
          <button
            onClick={() => navigate('/tickets')}
            className={styles.channelBtn}
            style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', padding: '0.5rem 0.85rem' }}
          >
            <ArrowLeft size={15} /> Volver
          </button>
          <FileText size={22} style={{ color: 'var(--primary)' }} />
          <h1>Informe de rechazos</h1>
          <span style={{
            fontSize: '0.8rem', fontWeight: 600, padding: '0.25rem 0.65rem', borderRadius: '99px',
            background: 'rgba(204,51,204,0.12)', color: 'var(--primary)',
          }}>Total {items.length}</span>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
          <select value={month} onChange={(e) => setMonth(Number(e.target.value))}
            style={{ padding: '0.5rem', borderRadius: '8px', border: '1px solid var(--border, #cbd5e1)' }}>
            {MONTHS.map((m, i) => <option key={i} value={i + 1}>{m}</option>)}
          </select>
          <select value={year} onChange={(e) => setYear(Number(e.target.value))}
            style={{ padding: '0.5rem', borderRadius: '8px', border: '1px solid var(--border, #cbd5e1)' }}>
            {years.map(y => <option key={y} value={y}>{y}</option>)}
          </select>
          <button
            onClick={handleExport}
            disabled={items.length === 0}
            className={styles.channelBtn}
            style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', padding: '0.55rem 1rem', opacity: items.length === 0 ? 0.6 : 1 }}
          >
            <Download size={14} /> Exportar Excel
          </button>
        </div>
      </div>

      {error && (
        <div style={{
          padding: '0.85rem 1.25rem', borderRadius: 'var(--radius-sm)', background: 'rgba(239,68,68,0.08)',
          border: '1px solid rgba(239,68,68,0.2)', color: '#dc2626', fontSize: '0.9rem',
        }}>{error}</div>
      )}

      {loading ? (
        <div style={{ display: 'flex', justifyContent: 'center', padding: '4rem', color: 'var(--gray-400)' }}>
          Cargando informe...
        </div>
      ) : items.length === 0 ? (
        <div style={{ display: 'flex', justifyContent: 'center', padding: '4rem', color: 'var(--gray-400)' }}>
          No hay rechazos registrados en este período.
        </div>
      ) : (
        <div style={{ overflowX: 'auto', border: '1px solid var(--border, #e2e8f0)', borderRadius: '12px' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem' }}>
            <thead>
              <tr style={{ background: 'var(--bg-secondary, #f8fafc)', textAlign: 'left' }}>
                {['Profesional', 'Email', 'Empresa', 'Fechas', 'Motivo', 'Rechazado por', 'Estado', 'Creado'].map(h => (
                  <th key={h} style={{ padding: '0.75rem', fontWeight: 700, color: 'var(--text-secondary)', whiteSpace: 'nowrap' }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {items.map(t => (
                <tr key={t.id} style={{ borderTop: '1px solid var(--border, #f1f5f9)' }}>
                  <td style={{ padding: '0.75rem', fontWeight: 600 }}>{professionalName(t)}</td>
                  <td style={{ padding: '0.75rem' }}>{t.professional_email || '-'}</td>
                  <td style={{ padding: '0.75rem' }}>{t.company_name || '-'}</td>
                  <td style={{ padding: '0.75rem', whiteSpace: 'nowrap' }}>{t.work_dates || '-'}</td>
                  <td style={{ padding: '0.75rem' }}>{t.reason || '-'}</td>
                  <td style={{ padding: '0.75rem' }}>{t.rejected_by_name || '-'}</td>
                  <td style={{ padding: '0.75rem' }}>
                    <span className={`${styles.badge} ${styles[`stage-${t.stage}`]}`}>{STAGE_LABEL[t.stage] || t.stage}</span>
                  </td>
                  <td style={{ padding: '0.75rem', whiteSpace: 'nowrap' }}>{new Date(t.created_at).toLocaleDateString('es-ES')}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
