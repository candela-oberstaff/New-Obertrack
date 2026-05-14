import { useState, useEffect } from 'react'
import { userService } from '../../services/api'
import type { User } from '../../types'
import Avatar from '../Common/Avatar'
import styles from '../../pages/Profile.module.css'

interface TeamPanelProps {
  type: 'manager' | 'employer'
  userId: number
  employerId?: number
}

export function TeamPanel({ type, employerId }: TeamPanelProps) {
  const [teamMembers, setTeamMembers] = useState<User[]>([])
  const [message, setMessage] = useState('')

  const fetchTeam = async () => {
    try {
      if (type === 'manager') {
        const data = await userService.getMyTeam()
        setTeamMembers(data)
      } else {
        const data = await userService.getAll()
        const allUsers = data.data || []
        setTeamMembers(allUsers.filter((u: User) => u.user_type === 'profesional' && u.empleador_id === employerId))
      }
    } catch (error) {
      console.error('Error fetching team:', error)
    }
  }

  useEffect(() => {
    fetchTeam()
  }, [type, employerId])

  const handlePromoteToManager = async (targetUserId: number) => {
    if (!window.confirm('¿Promover a este profesional a Manager?')) return
    try {
      await userService.promoteToManager(targetUserId)
      fetchTeam()
      setMessage('Usuario promovido a Manager')
      setTimeout(() => setMessage(''), 3000)
    } catch (error) {
      setMessage('Error al promover usuario')
    }
  }

  if (teamMembers.length === 0) return null

  return (
    <div className={`${styles['sidebar-card']} ${type === 'manager' ? styles['team-card'] : ''}`}>
      <h3>{type === 'manager' ? '👥 Mi Equipo' : '🏢 Personal de la Empresa'}</h3>
      <p className={styles['team-count']}>
        {teamMembers.length} profesional(es) {type === 'manager' ? 'a mi cargo' : 'registrado(s)'}
      </p>

      {message && <p className={styles['alert']} style={{ padding: '8px', fontSize: '13px', marginTop: '8px', marginBottom: '8px' }}>{message}</p>}

      <div className={styles['team-list']}>
        {teamMembers.map(member => (
          <div key={member.id} className={styles['team-member']}>
            <Avatar 
              src={member.avatar} 
              name={member.name} 
              size="sm" 
            />
            <div className={styles['member-info']}>
              <span className={styles['member-name']}>{member.name}</span>
              <span className={styles['member-role']}>
                {type === 'employer' && member.is_manager ? '👔 Manager' : (member.job_title || '💼 Profesional')}
              </span>
            </div>
            {type === 'employer' && !member.is_manager && (
              <button 
                className={styles['btn-promote']}
                onClick={() => handlePromoteToManager(member.id)}
                title="Promover a Manager"
              >
                ⬆️
              </button>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}
