import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { userService } from '../../services/api'
import type { User } from '../../types'
import Avatar from '../Common/Avatar'
import { useConfirm } from '../ui/ConfirmProvider'
import styles from '../../pages/Profile.module.css'
import { Search, Building, Shield, Briefcase, ArrowUp, ArrowDown, Users, MessageSquare } from 'lucide-react'

interface TeamPanelProps {
  type: 'manager' | 'employer'
  userId?: number
  employerId?: number
}

export function TeamPanel({ type }: TeamPanelProps) {
  const navigate = useNavigate()
  const [teamMembers, setTeamMembers] = useState<User[]>([])
  const [searchQuery, setSearchQuery] = useState('')
  const [message, setMessage] = useState('')
  const confirm = useConfirm()

  const fetchTeam = async () => {
    try {
      if (type === 'manager') {
        const data = await userService.getMyTeam()
        setTeamMembers(data)
      } else {
        const data = await userService.getEmployees()
        setTeamMembers(data)
      }
    } catch (error) {
      console.error('Error fetching team:', error)
    }
  }

  useEffect(() => {
    fetchTeam()
  }, [type])

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
      await userService.promoteToManager(targetUserId)
      fetchTeam()
      setMessage(isAlreadyManager ? 'Rol de Manager removido' : 'Usuario promovido a Manager')
      setTimeout(() => setMessage(''), 3000)
    } catch (error) {
      setMessage('Error al cambiar rol del usuario')
    }
  }

  if (teamMembers.length === 0) return null

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
            </div>
            <button 
              className={styles['btn-message']}
              onClick={() => navigate(`/chat?userId=${member.id}`)}
              title="Enviar mensaje directo"
              style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', padding: '6px', marginRight: '4px' }}
            >
              <MessageSquare size={14} style={{ color: '#4f46e5' }} />
            </button>
            {type === 'employer' && (
              <button 
                className={styles['btn-promote']}
                onClick={() => handlePromoteToManager(member.id, member.is_manager || false)}
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
    </div>
  )
}
