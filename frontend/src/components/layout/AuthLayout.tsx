import React from 'react'
import styles from '../../pages/Auth.module.css'

interface AuthLayoutProps {
  children: React.ReactNode;
  title?: string;
  subtitle?: string;
  isRegister?: boolean;
}

export default function AuthLayout({ children, title, subtitle, isRegister = false }: AuthLayoutProps) {
  return (
    <div className={styles['auth-container']}>
      <div className={`${styles['auth-card']} ${isRegister ? styles['register-card'] : ''}`}>
        <img src="/logos/Vertical_Blanco.png" alt="Obertrack" className={styles['auth-logo']} />
        <p className={styles['auth-tagline']}>Remote Work Tracking</p>
        
        {title && (
          isRegister ? (
            <div className={styles['auth-header']}>
              <h2>{title}</h2>
              {subtitle && <p>{subtitle}</p>}
            </div>
          ) : (
            <h2>{title}</h2>
          )
        )}

        {children}
      </div>
    </div>
  )
}
