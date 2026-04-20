// import React from 'react'
import type { LucideIcon } from 'lucide-react'
import styles from '../../pages/Reports.module.css'

interface StatCardProps {
  icon: LucideIcon
  iconColorClass: string
  label: string
  value: string | number
  progressText?: string
  progressColorClass?: string
}

export function StatCard({
  icon: Icon,
  iconColorClass,
  label,
  value,
  progressText,
  progressColorClass
}: StatCardProps) {
  return (
    <div className={styles['stat-card-modern']}>
      <div className={`${styles['stat-icon']} ${styles[iconColorClass]}`}>
        <Icon size={24} />
      </div>
      <div className={styles['stat-info']}>
        <span className={styles['stat-label']}>{label}</span>
        <span className={styles['stat-value']}>{value}</span>
        {progressText && (
          <span className={`${styles['stat-progress']} ${progressColorClass ? styles[progressColorClass] : ''}`}>
            {progressText}
          </span>
        )}
      </div>
    </div>
  )
}
