import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Clock, CheckSquare, CalendarClock, Ban, CheckCircle2, RefreshCw, User as UserIcon } from 'lucide-react'
import { useEmployeeTracking } from '../../hooks'
import Avatar from '../../components/Common/Avatar'
import { useConfirm } from '../../components/ui/ConfirmProvider'
import { formatDateOnly } from '../../utils/date'
import styles from './Tenants.module.css'

export default function EmployeeDetail() {
  const { id, eid } = useParams<{ id: string; eid: string }>()
  const navigate = useNavigate()
  const employeeId = Number(eid)
  const { tracking, isLoading, error, toggleStatus, resetPassword } = useEmployeeTracking(employeeId)

  const [tab, setTab] = useState<'jornadas' | 'tareas'>('jornadas')
  const confirm = useConfirm()

  const handleReset = async () => {
    const ok = await confirm({
      title: 'Resetear contraseña',
      message: '¿Resetear la contraseña a "temporary123"?',
      confirmLabel: 'Resetear',
      variant: 'primary',
    })
    if (ok) await resetPassword('temporary123')
  }

  const getWorkHourStatus = (wh: any) => {
    if (wh.approved) return { className: styles.badgeActive, label: 'Aprobada' }
    if (wh.rejected) return { className: styles.badgeRejected, label: 'Rechazada' }
    return { className: styles.badgePending, label: 'Pendiente' }
  }

  if (isLoading) {
    return (
      <div className={styles.page}>
        <div className={styles.loading}>
          <div className={styles.spinner} />
          <p>Cargando profesional...</p>
        </div>
      </div>
    )
  }

  if (error || !tracking) {
    return (
      <div className={styles.page}>
        <button className={styles.backBtn} onClick={() => navigate(`/admin/tenants/${id}`)}>
          <ArrowLeft size={18} /> Volver
        </button>
        <div className={styles.empty}>
          <UserIcon size={40} />
          <p>{error || 'Profesional no encontrado'}</p>
        </div>
      </div>
    )
  }

  const { user, summary, work_hours, tasks } = tracking

  return (
    <div className={styles.page}>
      <button className={styles.backBtn} onClick={() => navigate(`/admin/tenants/${id}`)}>
        <ArrowLeft size={18} /> Profesionales
      </button>

      <div className={styles.detailHeader}>
        <div className={styles.detailIdentity}>
          <Avatar src={user.avatar} name={user.name} size="lg" />
          <div>
            <div className={styles.detailTitleRow}>
              <h1>{user.name}</h1>
              <span className={`${styles.badge} ${user.is_active ? styles.badgeActive : styles.badgeSuspended}`}>
                {user.is_active ? 'Activo' : 'Inactivo'}
              </span>
            </div>
            <div className={styles.detailMeta}>
              <span>{user.email}</span>
              <span className={styles.typeBadge}>{user.is_manager ? 'manager' : user.user_type}</span>
            </div>
          </div>
        </div>
        <div className={styles.headerActions}>
          <button className={styles.secondaryBtn} onClick={handleReset}>
            <RefreshCw size={16} /> Resetear contraseña
          </button>
          {user.is_active ? (
            <button className={styles.dangerBtn} onClick={toggleStatus}>
              <Ban size={16} /> Desactivar
            </button>
          ) : (
            <button className={styles.successBtn} onClick={toggleStatus}>
              <CheckCircle2 size={16} /> Activar
            </button>
          )}
        </div>
      </div>

      <div className={styles.kpis}>
        <div className={styles.kpiCard}>
          <div className={styles.kpiIcon} style={{ background: 'linear-gradient(135deg, var(--primary), var(--primary-dark))' }}>
            <Clock size={24} />
          </div>
          <div>
            <span className={styles.kpiValue}>{summary?.hours_this_month?.toFixed(1) ?? '0.0'} h</span>
            <span className={styles.kpiLabel}>Horas este mes</span>
          </div>
        </div>
        <div className={styles.kpiCard}>
          <div className={styles.kpiIcon} style={{ background: 'linear-gradient(135deg, #8b5cf6, #7c3aed)' }}>
            <CheckSquare size={24} />
          </div>
          <div>
            <span className={styles.kpiValue}>{summary?.tasks_completed ?? 0}/{summary?.tasks_assigned ?? 0}</span>
            <span className={styles.kpiLabel}>Tareas completadas</span>
          </div>
        </div>
        <div className={styles.kpiCard}>
          <div className={styles.kpiIcon} style={{ background: 'linear-gradient(135deg, #10b981, #059669)' }}>
            <CalendarClock size={24} />
          </div>
          <div>
            <span className={styles.kpiValue}>{summary?.last_active ? new Date(summary.last_active).toLocaleDateString('es-ES') : '—'}</span>
            <span className={styles.kpiLabel}>Última actividad</span>
          </div>
        </div>
      </div>

      <div className={styles.subTabs}>
        <button className={tab === 'jornadas' ? styles.subTabActive : styles.subTab} onClick={() => setTab('jornadas')}>Jornadas ({work_hours.length})</button>
        <button className={tab === 'tareas' ? styles.subTabActive : styles.subTab} onClick={() => setTab('tareas')}>Tareas ({tasks.length})</button>
      </div>

      {tab === 'jornadas' && (
        work_hours.length === 0 ? (
          <div className={styles.empty}><Clock size={40} /><p>Sin jornadas registradas</p></div>
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr><th>Fecha</th><th>Tipo</th><th>Horas</th><th>Estado</th><th>Actividades</th></tr>
              </thead>
              <tbody>
                {work_hours.map(wh => (
                  <tr key={wh.id}>
                    <td>{wh.work_date ? new Date(wh.work_date).toLocaleDateString('es-ES') : '—'}</td>
                    <td><span className={styles.typeBadge}>{wh.work_type === 'complete' ? 'Jornada' : 'Ausencia'}</span></td>
                    <td>{wh.hours_worked?.toFixed(1)} h</td>
                    <td>
                      <span className={`${styles.badge} ${getWorkHourStatus(wh).className}`}>
                        {getWorkHourStatus(wh).label}
                      </span>
                    </td>
                    <td className={styles.truncate}>{wh.activities || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )
      )}

      {tab === 'tareas' && (
        tasks.length === 0 ? (
          <div className={styles.empty}><CheckSquare size={40} /><p>Sin tareas asignadas</p></div>
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr><th>Tarea</th><th>Tablero</th><th>Estado</th><th>Vencimiento</th></tr>
              </thead>
              <tbody>
                {tasks.map(t => (
                  <tr key={t.id}>
                    <td>{t.title}</td>
                    <td>{t.board_name || '—'}</td>
                    <td>
                      <span className={`${styles.badge} ${t.completed ? styles.badgeActive : styles.badgePending}`}>
                        {t.completed ? 'Finalizada' : t.status}
                      </span>
                    </td>
                    <td>{t.end_date ? formatDateOnly(t.end_date) : '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )
      )}
    </div>
  )
}
