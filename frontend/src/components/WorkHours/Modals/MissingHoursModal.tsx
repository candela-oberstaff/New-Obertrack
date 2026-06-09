import { Clock, AlertCircle } from 'lucide-react'
import { Modal, Button } from '../../ui'

interface WorkHour {
  id: number
  work_date: string
  work_type: 'complete' | 'absence' | 'recover'
  hours_worked: number
  absence_hours?: number
  absence_reason?: string
  activities?: string
  approved: boolean
  user?: {
    id: number
    name: string
    email: string
  }
}

interface MissingHoursModalProps {
  isOpen: boolean
  onClose: () => void
  workHours: WorkHour[]
  currentMonthName: string
}

export function MissingHoursModal({
  isOpen,
  onClose,
  workHours,
  currentMonthName
}: MissingHoursModalProps) {
  // Filter only absence entries in the current selected month
  const absences = workHours.filter(wh => wh.work_type === 'absence')

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      size="lg"
      title={
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: '8px' }}>
          <AlertCircle size={22} style={{ color: '#5a52e6' }} />
          Horas Faltantes y Ausencias
        </span>
      }
      footer={<Button onClick={onClose}>Entendido</Button>}
    >
      <p style={{ margin: '0 0 16px 0', fontSize: '13px', color: '#64748b' }}>
        Historial detallado del equipo para el mes de {currentMonthName}
      </p>

      {absences.length === 0 ? (
        <div style={{ textAlign: 'center', padding: '40px 20px', color: '#64748b' }}>
          <AlertCircle size={48} style={{ color: '#94a3b8', marginBottom: '12px' }} />
          <p style={{ fontWeight: '500', margin: '0 0 4px 0' }}>No hay ausencias registradas</p>
          <p style={{ fontSize: '12px', color: '#94a3b8', margin: 0 }}>
            Todos los profesionales de tu equipo han completado sus jornadas este mes.
          </p>
        </div>
      ) : (
        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left' }}>
            <thead>
              <tr style={{ borderBottom: '2px solid #e2e8f0', color: '#64748b', fontSize: '12px', fontWeight: '700', textTransform: 'uppercase' }}>
                <th style={{ padding: '12px 8px' }}>Profesional</th>
                <th style={{ padding: '12px 8px' }}>Fecha</th>
                <th style={{ padding: '12px 8px' }}>Horas Faltantes</th>
                <th style={{ padding: '12px 8px' }}>Motivo</th>
              </tr>
            </thead>
            <tbody>
              {absences.map((wh) => (
                <tr
                  key={wh.id}
                  style={{
                    borderBottom: '1px solid #f1f5f9',
                    fontSize: '14px',
                    color: '#334155'
                  }}
                >
                  <td style={{ padding: '14px 8px', fontWeight: '600', display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <span
                      style={{
                        width: '28px',
                        height: '28px',
                        borderRadius: '50%',
                        backgroundColor: '#f5f3ff',
                        color: '#5a52e6',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        fontSize: '11px',
                        fontWeight: '700'
                      }}
                    >
                      {wh.user?.name ? wh.user.name.split(' ').map(n => n[0]).join('').substring(0, 2).toUpperCase() : 'P'}
                    </span>
                    <span>{wh.user?.name || 'Profesional'}</span>
                  </td>
                  <td style={{ padding: '14px 8px' }}>
                    {new Date(wh.work_date).toLocaleDateString('es-ES', { day: '2-digit', month: '2-digit', year: 'numeric' })}
                  </td>
                  <td style={{ padding: '14px 8px' }}>
                    <span
                      style={{
                        backgroundColor: '#fee2e2',
                        color: '#ef4444',
                        padding: '4px 10px',
                        borderRadius: '6px',
                        fontWeight: '700',
                        fontSize: '12px',
                        display: 'inline-flex',
                        alignItems: 'center',
                        gap: '4px'
                      }}
                    >
                      <Clock size={13} /> {wh.absence_hours || 8}h
                    </span>
                  </td>
                  <td style={{ padding: '14px 8px', textTransform: 'capitalize' }}>
                    <span
                      style={{
                        backgroundColor: '#f8fafc',
                        border: '1px solid #e2e8f0',
                        padding: '3px 8px',
                        borderRadius: '6px',
                        fontSize: '12px',
                        fontWeight: '500',
                        color: '#475569'
                      }}
                    >
                      {wh.absence_reason?.replace('_', ' ') || 'Otro'}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Modal>
  )
}
