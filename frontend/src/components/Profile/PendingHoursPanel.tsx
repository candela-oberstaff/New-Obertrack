import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { workHourService } from '../../services/api'
import styles from '../../pages/Profile.module.css'

const MONTHS_ES = ['Ene', 'Feb', 'Mar', 'Abr', 'May', 'Jun', 'Jul', 'Ago', 'Sep', 'Oct', 'Nov', 'Dic']

export function PendingHoursPanel() {
  const qc = useQueryClient()
  const [selectedPending, setSelectedPending] = useState<number[]>([])
  const [isLoadingPending, setIsLoadingPending] = useState(false)
  const [message, setMessage] = useState('')

  const { data: pendingHours = [] } = useQuery({
    queryKey: ['work-hours-pending'],
    queryFn: () => workHourService.getPending(),
  })

  const handleApproveHours = async () => {
    if (selectedPending.length === 0) return
    setIsLoadingPending(true)
    setMessage('')
    try {
      await workHourService.approve(selectedPending)
      setSelectedPending([])
      await qc.invalidateQueries({ queryKey: ['work-hours-pending'] })
      setMessage('Horas aprobadas!')
      setTimeout(() => setMessage(''), 3000)
    } catch (error) {
      setMessage('Error al aprobar horas')
    } finally {
      setIsLoadingPending(false)
    }
  }

  const toggleSelectPending = (id: number) => {
    setSelectedPending(prev =>
      prev.includes(id) ? prev.filter(i => i !== id) : [...prev, id]
    )
  }

  if (pendingHours.length === 0) return null

  return (
    <div className={styles['sidebar-card']}>
      <h3>Horas Pendientes</h3>
      <p className={styles['pending-count']}>{pendingHours.length} registro(s) sin aprobar</p>
      
      {message && <p className={styles['alert']} style={{ padding: '8px', fontSize: '13px' }}>{message}</p>}

      <div className={styles['pending-list']}>
        {pendingHours.slice(0, 5).map(wh => (
          <div key={wh.id} className={styles['pending-item']} onClick={() => toggleSelectPending(wh.id)}>
            <input 
              type="checkbox" 
              checked={selectedPending.includes(wh.id)}
              readOnly
            />
            <div className={styles['pending-info']}>
              <span className={styles['pending-user']}>{wh.user?.name || 'Usuario'}</span>
              <span className={styles['pending-date']}>
                {new Date(wh.work_date).getDate()} {MONTHS_ES[new Date(wh.work_date).getMonth()]}
              </span>
              <span className={`${styles['pending-type']} ${styles[wh.work_type] || wh.work_type}`}>
                {wh.work_type === 'complete' ? 'Completa' : 'Ausencia'}
              </span>
            </div>
          </div>
        ))}
      </div>

      {pendingHours.length > 5 && (
        <p className={styles['more-pending']}>+ {pendingHours.length - 5} más</p>
      )}

      {selectedPending.length > 0 && (
        <button 
          className={styles['btn-approve']} 
          onClick={handleApproveHours}
          disabled={isLoadingPending}
        >
          {isLoadingPending ? 'Aprobando...' : `Aprobar (${selectedPending.length})`}
        </button>
      )}
    </div>
  )
}
