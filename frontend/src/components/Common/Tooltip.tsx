import React, { useState } from 'react'
import { Info, X } from 'lucide-react'
import styles from './Tooltip.module.css'

interface TooltipProps {
  content: string
  size?: number
  style?: React.CSSProperties
}

export default function Tooltip({ content, size = 14, style }: TooltipProps) {
  const [isOpen, setIsOpen] = useState(false)

  const handleTriggerClick = (e: React.MouseEvent) => {
    // Only intercept and show mobile modal on screens <= 768px
    if (window.innerWidth <= 768) {
      e.preventDefault()
      e.stopPropagation()
      setIsOpen(true)
    }
  }

  return (
    <>
      <span 
        className={styles['tooltip-trigger']}
        onClick={handleTriggerClick}
        title={content} // Native tooltip works automatically on desktop
        style={{ ...style, display: 'inline-flex', alignItems: 'center', verticalAlign: 'middle', cursor: 'help' }}
      >
        <Info size={size} style={{ marginLeft: '6px', color: '#94a3b8' }} />
      </span>

      {isOpen && (
        <div className={styles['mobile-tooltip-overlay']} onClick={() => setIsOpen(false)}>
          <div className={styles['mobile-tooltip-card']} onClick={(e) => e.stopPropagation()}>
            <div className={styles['mobile-tooltip-header']}>
              <div className={styles['info-title']}>
                <Info size={18} style={{ color: 'var(--primary, #cc33cc)' }} />
                <span>Información</span>
              </div>
              <button className={styles['close-btn']} onClick={() => setIsOpen(false)} aria-label="Cerrar">
                <X size={16} />
              </button>
            </div>
            <div className={styles['mobile-tooltip-body']}>
              {content}
            </div>
          </div>
        </div>
      )}
    </>
  )
}
