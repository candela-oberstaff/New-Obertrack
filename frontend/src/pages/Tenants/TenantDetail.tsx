import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Building2, Users, LayoutGrid, CheckSquare, Activity, Ban, CheckCircle2, Mail, Calendar, RefreshCw, ChevronRight } from 'lucide-react'
import { useTenantDetail } from '../../hooks'
import type { EmployeeSummary } from '../../types'
import Avatar from '../../components/Common/Avatar'
import { useConfirm } from '../../components/ui/ConfirmProvider'
import styles from './Tenants.module.css'

export default function TenantDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const tenantId = Number(id)
  const { tenant, employees, activity, isLoading, error, suspendTenant, activateTenant, toggleEmployeeStatus, resetEmployeePassword } = useTenantDetail(tenantId)

  const [tab, setTab] = useState<'resumen' | 'usuarios' | 'actividad'>('resumen')
  const confirm = useConfirm()

  const handleToggleEmployee = async (e: React.MouseEvent, emp: EmployeeSummary) => {
    e.stopPropagation()
    await toggleEmployeeStatus(emp)
  }

  const handleResetEmployee = async (e: React.MouseEvent, emp: EmployeeSummary) => {
    e.stopPropagation()
    const ok = await confirm({
      title: 'Resetear contraseña',
      message: `¿Resetear la contraseña de ${emp.name} a "temporary123"?`,
      confirmLabel: 'Resetear',
      variant: 'primary',
    })
    if (ok) await resetEmployeePassword(emp.id, 'temporary123')
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

      <div className={styles.subTabs}>
        <button className={tab === 'resumen' ? styles.subTabActive : styles.subTab} onClick={() => setTab('resumen')}>Resumen</button>
        <button className={tab === 'usuarios' ? styles.subTabActive : styles.subTab} onClick={() => setTab('usuarios')}>Empleados ({employees.length})</button>
        <button className={tab === 'actividad' ? styles.subTabActive : styles.subTab} onClick={() => setTab('actividad')}>Actividad</button>
      </div>

      {tab === 'resumen' && (
        <div className={styles.kpis}>
          <div className={styles.kpiCard}>
            <div className={styles.kpiIcon} style={{ background: 'linear-gradient(135deg, var(--primary), var(--primary-dark))' }}>
              <Users size={24} />
            </div>
            <div>
              <span className={styles.kpiValue}>{tenant.user_count}</span>
              <span className={styles.kpiLabel}>Usuarios</span>
            </div>
          </div>
          <div className={styles.kpiCard}>
            <div className={styles.kpiIcon} style={{ background: 'linear-gradient(135deg, #f59e0b, #d97706)' }}>
              <LayoutGrid size={24} />
            </div>
            <div>
              <span className={styles.kpiValue}>{tenant.board_count}</span>
              <span className={styles.kpiLabel}>Tableros</span>
            </div>
          </div>
          <div className={styles.kpiCard}>
            <div className={styles.kpiIcon} style={{ background: 'linear-gradient(135deg, #8b5cf6, #7c3aed)' }}>
              <CheckSquare size={24} />
            </div>
            <div>
              <span className={styles.kpiValue}>{tenant.task_count}</span>
              <span className={styles.kpiLabel}>Tareas</span>
            </div>
          </div>
        </div>
      )}

      {tab === 'usuarios' && (
        employees.length === 0 ? (
          <div className={styles.empty}><Users size={40} /><p>Esta empresa no tiene empleados</p></div>
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
                {employees.map(emp => {
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
                          <button
                            className={`${styles.iconBtn} ${emp.is_active ? styles.danger : styles.success}`}
                            onClick={(e) => handleToggleEmployee(e, emp)}
                            title={emp.is_active ? 'Desactivar empleado' : 'Activar empleado'}
                          >
                            {emp.is_active ? <Ban size={16} /> : <CheckCircle2 size={16} />}
                          </button>
                          <button
                            className={styles.iconBtn}
                            onClick={(e) => handleResetEmployee(e, emp)}
                            title="Resetear contraseña"
                          >
                            <RefreshCw size={16} />
                          </button>
                          <ChevronRight size={18} className={styles.chevron} />
                        </div>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )
      )}

      {tab === 'actividad' && (
        activity.length === 0 ? (
          <div className={styles.empty}><Activity size={40} /><p>Sin actividad registrada</p></div>
        ) : (
          <div className={styles.activityList}>
            {activity.map((a, i) => {
              const date = a.timestamp ? new Date(a.timestamp) : null
              const valid = date && !isNaN(date.getTime())
              return (
                <div key={`act-${i}`} className={styles.activityItem}>
                  <div className={styles.activityIcon}><Activity size={16} /></div>
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
    </div>
  )
}
