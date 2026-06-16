import { useState, useEffect } from 'react'
import { Building2, FileText } from 'lucide-react'
import { authService } from '../../services/api'
import { ExpedienteModal } from '../Admin/ExpedienteModal'
import styles from '../../pages/Profile.module.css'

// Mi Expediente: el profesional ve su historial laboral (empresas donde trabaja
// o trabajó) y, por cada empleo, el expediente que la empresa compartió con él
// (resumen de horas/tareas/antigüedad, evaluaciones y documentos compartidos).
export function MiExpediente() {
  const [employments, setEmployments] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [openEmp, setOpenEmp] = useState<any | null>(null)

  useEffect(() => {
    authService.myEmployments()
      .then(setEmployments)
      .catch(() => setEmployments([]))
      .finally(() => setLoading(false))
  }, [])

  if (loading) return null
  if (employments.length === 0) return null

  return (
    <div className={styles['sidebar-card']} style={{ marginTop: '16px' }}>
      <h3 style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '6px' }}>
        <Building2 size={18} /> Mi Expediente
      </h3>
      <p style={{ margin: '0 0 14px', fontSize: '0.82rem', color: '#94a3b8' }}>
        Tu historial laboral. Abre cada empleo para ver el resumen y lo que la empresa compartió contigo.
      </p>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
        {employments.map(emp => {
          const ended = emp.status === 'ended'
          return (
            <div key={emp.id} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '10px', padding: '10px 12px', border: '1px solid #e2e8f0', borderRadius: '10px', opacity: ended ? 0.7 : 1 }}>
              <div>
                <span style={{ fontWeight: 700, color: '#0f172a' }}>{emp.company_name}</span>
                <div style={{ fontSize: '0.78rem', color: '#94a3b8' }}>
                  {emp.job_title || 'Sin cargo'} · desde {new Date(emp.started_at).toLocaleDateString('es-ES')}
                  {ended && emp.ended_at && ` · hasta ${new Date(emp.ended_at).toLocaleDateString('es-ES')}`}
                </div>
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <span style={{ padding: '3px 10px', borderRadius: 999, fontSize: '0.7rem', fontWeight: 700, background: ended ? 'rgba(100,116,139,0.12)' : 'rgba(16,185,129,0.12)', color: ended ? '#64748b' : '#047857' }}>
                  {ended ? 'Finalizado' : 'Activo'}
                </span>
                <button onClick={() => setOpenEmp(emp)} title="Ver expediente"
                  style={{ display: 'inline-flex', alignItems: 'center', gap: '4px', padding: '5px 10px', borderRadius: '8px', border: '1px solid #cbd5e1', background: '#fff', color: '#334155', fontWeight: 600, cursor: 'pointer', fontSize: '0.78rem' }}>
                  <FileText size={14} /> Ver
                </button>
              </div>
            </div>
          )
        })}
      </div>

      {openEmp && (
        <ExpedienteModal
          userId={0}
          employment={openEmp}
          canManage={false}
          selfMode
          onClose={() => setOpenEmp(null)}
        />
      )}
    </div>
  )
}
