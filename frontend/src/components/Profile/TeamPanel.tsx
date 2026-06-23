import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { userService, employerService } from '../../services/api'
import Avatar from '../Common/Avatar'
import { useConfirm } from '../ui/ConfirmProvider'
import { Modal } from '../ui'
import { Select } from '../ui/Select'
import { ExpedienteModal } from '../Admin/ExpedienteModal'
import EmploymentManagersEditor from '../Admin/EmploymentManagersEditor'
import styles from '../../pages/Profile.module.css'
import { Search, Building, Shield, Briefcase, ArrowUp, ArrowDown, Users, MessageSquare, FileText } from 'lucide-react'

interface TeamPanelProps {
  type: 'manager' | 'employer'
  userId?: number
  employerId?: number
}

export function TeamPanel({ type }: TeamPanelProps) {
  const navigate = useNavigate()
  const qc = useQueryClient()
  const [searchQuery, setSearchQuery] = useState('')
  const [message, setMessage] = useState('')
  const confirm = useConfirm()
  // Expediente abierto (empleo resuelto en la empresa del empleador).
  const [expedienteEmp, setExpedienteEmp] = useState<any | null>(null)
  // Modal multi-manager (empleador): miembro objetivo + empleo resuelto.
  const [managersTarget, setManagersTarget] = useState<{ id: number; name: string } | null>(null)
  const [managersEmp, setManagersEmp] = useState<{ id: number; company_id: number } | null>(null)
  const [managersError, setManagersError] = useState('')
  // Bloqueo al degradar un manager con equipo (paridad con el flujo superadmin).
  const [blockTarget, setBlockTarget] = useState<{ member: any; reports: any[] } | null>(null)
  const [reassignTo, setReassignTo] = useState<number | ''>('')
  const [reassignBusy, setReassignBusy] = useState(false)

  // Flag de features: solo en el panel de empleador habilitamos multi-manager.
  const { data: features } = useQuery({
    queryKey: ['features'],
    queryFn: () => employerService.getFeatures(),
    enabled: type === 'employer',
    staleTime: 5 * 60 * 1000,
  })
  const multiManager = features?.multi_manager_reads === true

  const openExpediente = async (memberId: number) => {
    try {
      setExpedienteEmp(await employerService.resolveEmployment(memberId))
    } catch {
      setMessage('Este profesional no tiene un empleo activo en tu empresa')
      setTimeout(() => setMessage(''), 3000)
    }
  }

  // Abre el modal de managers: resuelve el empleo del miembro en MI empresa.
  // El EmploymentView trae .id y .company_id en el primer nivel.
  const openManagers = async (member: { id: number; name: string }) => {
    setManagersTarget(member)
    setManagersEmp(null)
    setManagersError('')
    try {
      const emp = await employerService.getEmployerEmployment(member.id)
      setManagersEmp({ id: Number(emp.id), company_id: Number(emp.company_id) })
    } catch {
      setManagersError('Este profesional no tiene un empleo activo en tu empresa.')
    }
  }

  const closeManagers = () => {
    setManagersTarget(null)
    setManagersEmp(null)
    setManagersError('')
  }

  const { data: teamMembers = [] } = useQuery({
    queryKey: ['team', type],
    queryFn: () => (type === 'manager' ? userService.getMyTeam() : userService.getEmployees()),
  })

  const fetchTeam = () => qc.invalidateQueries({ queryKey: ['team', type] })

  const handlePromoteToManager = async (targetUserId: number, isAlreadyManager: boolean) => {
    const ok = await confirm({
      title: isAlreadyManager ? 'Quitar rol de Manager' : 'Promover a Manager',
      message: isAlreadyManager 
        ? '¿Quitar el rol de Manager a este profesional?' 
        : '¿Promover a este profesional a Manager?',
      confirmLabel: isAlreadyManager ? 'Quitar' : 'Promover',
      variant: isAlreadyManager ? 'danger' : 'primary',
    })
    if (!ok) return
    try {
      await userService.promoteToManager(targetUserId, !isAlreadyManager)
      fetchTeam()
      setMessage(isAlreadyManager ? 'Rol de Manager removido' : 'Usuario promovido a Manager')
      setTimeout(() => setMessage(''), 3000)
    } catch (err: any) {
      setMessage(err?.response?.data?.error ?? 'Error al cambiar rol del usuario')
      setTimeout(() => setMessage(''), 3000)
    }
  }

  // Antes de degradar: trae el equipo del manager; si tiene gente a cargo, muestra
  // el bloqueo (lista + reasignar) en vez de quitarle el rol directo.
  const handleRemoveManagerClick = async (member: any) => {
    try {
      const reports = await employerService.getManagerReports(member.id)
      if (Array.isArray(reports) && reports.length > 0) {
        setReassignTo(''); setBlockTarget({ member, reports })
        return
      }
    } catch { /* si falla la consulta, el backend igual valida con 409 */ }
    handlePromoteToManager(member.id, true)
  }

  const submitReassign = async () => {
    if (!blockTarget) return
    setReassignBusy(true)
    try {
      await userService.reassignTeam(blockTarget.member.id, reassignTo === '' ? null : Number(reassignTo))
      fetchTeam()
      setBlockTarget(null)
      setMessage('Equipo reasignado')
      setTimeout(() => setMessage(''), 3000)
    } catch (err: any) {
      setMessage(err?.response?.data?.error ?? 'No se pudo reasignar el equipo')
      setTimeout(() => setMessage(''), 3000)
    } finally { setReassignBusy(false) }
  }

  const handleAssignManager = async (memberId: number, value: string) => {
    const managerId = value === '' ? null : Number(value)
    try {
      await userService.assignToManager(memberId, managerId)
      fetchTeam()
      setMessage(managerId ? 'Manager asignado' : 'Manager removido')
      setTimeout(() => setMessage(''), 3000)
    } catch (err: any) {
      setMessage(err?.response?.data?.error ?? 'Error al asignar manager')
      setTimeout(() => setMessage(''), 3000)
    }
  }

  if (teamMembers.length === 0) return null

  // Managers del tenant derivados del propio dataset (solo aplica en panel de empleador).
  const managers = teamMembers.filter(m => m.is_manager)
  // Opciones para el editor multi-manager (el editor excluye al propio miembro).
  const managerOptions = managers.map(m => ({ id: m.id, name: m.name }))

  const filteredMembers = teamMembers.filter(member =>
    (member.name || '').toLowerCase().includes(searchQuery.toLowerCase()) || 
    (member.email || '').toLowerCase().includes(searchQuery.toLowerCase())
  )

  return (
    <div className={`${styles['sidebar-card']} ${type === 'manager' ? styles['team-card'] : ''}`}>
      <h3 className={styles['sidebar-header-title']}>
        {type === 'manager' ? (
          <>
            <Users size={16} style={{ color: '#06b6d4' }} /> Mi Equipo
          </>
        ) : (
          <>
            <Building size={16} style={{ color: '#4f46e5' }} /> Personal de la Empresa
          </>
        )}
      </h3>
      <p className={styles['team-count']}>
        {teamMembers.length} profesional(es) {type === 'manager' ? 'a mi cargo' : 'registrado(s)'}
      </p>

      {teamMembers.length > 0 && (
        <div className={styles['search-bar-container']}>
          <Search size={14} className={styles['search-icon']} />
          <input 
            type="text" 
            placeholder="Buscar por nombre o email..." 
            value={searchQuery}
            onChange={e => setSearchQuery(e.target.value)}
            className={styles['search-input']}
          />
        </div>
      )}

      {message && <p className={styles['alert']} style={{ padding: '8px', fontSize: '13px', marginTop: '8px', marginBottom: '8px' }}>{message}</p>}

      <div className={styles['team-list']}>
        {filteredMembers.map(member => (
          <div key={member.id} className={styles['team-member']}>
            <Avatar 
              src={member.avatar} 
              name={member.name} 
              size="sm" 
            />
            <div className={styles['member-info']}>
              <span className={styles['member-name']}>{member.name}</span>
              <span className={styles['member-role']} style={{ display: 'flex', alignItems: 'center', gap: '4px', marginTop: '2px' }}>
                {member.is_manager ? (
                  <>
                    <Shield size={12} style={{ color: '#f59e0b', flexShrink: 0 }} /> Manager
                  </>
                ) : (
                  <>
                    <Briefcase size={12} style={{ color: '#64748b', flexShrink: 0 }} /> {member.job_title || 'Profesional'}
                  </>
                )}
              </span>
              {type === 'employer' && !member.is_manager && !multiManager && (
                managers.length === 0 ? (
                  <span style={{ fontSize: '11px', color: '#94a3b8', marginTop: '4px' }}>
                    Sin managers disponibles
                  </span>
                ) : (
                  <select
                    value={member.manager_id ?? ''}
                    onChange={e => handleAssignManager(member.id, e.target.value)}
                    title="Asignar manager"
                    style={{
                      marginTop: '4px',
                      maxWidth: '100%',
                      fontSize: '12px',
                      padding: '2px 4px',
                      borderRadius: '6px',
                      border: '1px solid #e2e8f0',
                      color: '#334155',
                      background: '#fff',
                    }}
                  >
                    <option value="">Sin manager</option>
                    {managers
                      .filter(mgr => mgr.id !== member.id)
                      .map(mgr => (
                        <option key={mgr.id} value={mgr.id}>{mgr.name}</option>
                      ))}
                  </select>
                )
              )}
            </div>
            <button 
              className={styles['btn-message']}
              onClick={() => navigate(`/chat?userId=${member.id}`)}
              title="Enviar mensaje directo"
              style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', padding: '6px', marginRight: '4px' }}
            >
              <MessageSquare size={14} style={{ color: '#4f46e5' }} />
            </button>
            {type === 'employer' && multiManager && (
              <button
                className={styles['btn-message']}
                onClick={() => openManagers({ id: member.id, name: member.name })}
                title="Gestionar managers"
                style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', padding: '6px', marginRight: '4px' }}
              >
                <Shield size={14} style={{ color: '#f59e0b' }} />
              </button>
            )}
            {type === 'employer' && (
              <button
                className={styles['btn-message']}
                onClick={() => openExpediente(member.id)}
                title="Expediente"
                style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', padding: '6px', marginRight: '4px' }}
              >
                <FileText size={14} style={{ color: '#7c3aed' }} />
              </button>
            )}
            {type === 'employer' && (
              <button
                className={styles['btn-promote']}
                onClick={() => (member.is_manager ? handleRemoveManagerClick(member) : handlePromoteToManager(member.id, false))}
                title={member.is_manager ? "Quitar rol de Manager" : "Promover a Manager"}
                style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', padding: '6px' }}
              >
                {member.is_manager ? <ArrowDown size={14} style={{ color: '#ef4444' }} /> : <ArrowUp size={14} style={{ color: '#10b981' }} />}
              </button>
            )}
          </div>
        ))}
        {filteredMembers.length === 0 && (
          <p style={{ textAlign: 'center', fontSize: '12px', color: '#64748b', padding: '12px 0' }}>
            No se encontraron profesionales
          </p>
        )}
      </div>

      {expedienteEmp && (
        <ExpedienteModal
          userId={0}
          employment={expedienteEmp}
          canManage
          employerMode
          onClose={() => setExpedienteEmp(null)}
        />
      )}

      {managersTarget && (
        <Modal
          isOpen
          onClose={closeManagers}
          size="md"
          title={`Gestionar managers de ${managersTarget.name}`}
        >
          {managersError ? (
            <p style={{ fontSize: '13px', color: '#64748b', margin: 0 }}>{managersError}</p>
          ) : managersEmp ? (
            <EmploymentManagersEditor
              mode="employer"
              userId={managersTarget.id}
              employmentId={managersEmp.id}
              companyId={managersEmp.company_id}
              managerOptions={managerOptions}
              onChanged={fetchTeam}
            />
          ) : (
            <p style={{ fontSize: '13px', color: '#94a3b8', margin: 0 }}>Cargando empleo…</p>
          )}
        </Modal>
      )}

      {blockTarget && (
        <Modal isOpen onClose={() => setBlockTarget(null)} size="md" title="No puedes quitar el rol todavía">
          <p style={{ fontSize: '13px', color: '#475569', marginTop: 0 }}>
            <strong>{blockTarget.member.name}</strong> tiene {blockTarget.reports.length} profesional(es) a su cargo. Reasigna su equipo antes de quitarle el rol de Manager.
          </p>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '6px', maxHeight: 220, overflowY: 'auto', marginBottom: '14px' }}>
            {blockTarget.reports.map((r: any) => (
              <div key={r.id} style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '6px 8px', border: '1px solid #e2e8f0', borderRadius: '8px' }}>
                <Avatar src={r.avatar} name={r.name} size="sm" />
                <div style={{ minWidth: 0 }}>
                  <div style={{ fontWeight: 600, fontSize: '13px', color: '#0f172a' }}>{r.name}</div>
                  <div style={{ fontSize: '11px', color: '#94a3b8', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{(r.job_title || 'Profesional')} · {r.email}</div>
                </div>
              </div>
            ))}
          </div>
          <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: '#334155', marginBottom: '6px' }}>Reasignar todo el equipo a:</label>
          <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
            <div style={{ flex: 1 }}>
              <Select
                fullWidth
                clearable
                placeholder="Sin manager (desasignar)"
                value={reassignTo}
                onChange={v => setReassignTo(v === '' || v == null ? '' : Number(v))}
                options={managerOptions.filter(m => m.id !== blockTarget.member.id).map(m => ({ value: m.id, label: m.name }))}
              />
            </div>
            <button
              onClick={submitReassign}
              disabled={reassignBusy}
              style={{ padding: '8px 14px', borderRadius: '8px', border: 'none', background: 'var(--primary, #7c3aed)', color: '#fff', fontWeight: 700, fontSize: '13px', cursor: reassignBusy ? 'progress' : 'pointer' }}
            >
              Reasignar
            </button>
          </div>
        </Modal>
      )}
    </div>
  )
}
