import styles from '../WhatsApp.module.css'

export default function EmptyState() {
  return (
    <div className={styles.emptyState}>
      <div className={styles.emptyIcon}>
        <svg viewBox="0 0 64 64" fill="none" xmlns="http://www.w3.org/2000/svg" width="80" height="80">
          <circle cx="32" cy="32" r="32" fill="url(#wa-bg)" opacity="0.15"/>
          <path d="M32 10C19.85 10 10 19.85 10 32c0 3.9 1.05 7.56 2.89 10.71L10 54l11.56-2.83A21.85 21.85 0 0 0 32 54c12.15 0 22-9.85 22-22S44.15 10 32 10Z" fill="url(#wa-bg)"/>
          <defs>
            <linearGradient id="wa-bg" x1="10" y1="10" x2="54" y2="54" gradientUnits="userSpaceOnUse">
              <stop stopColor="#25D366"/>
              <stop offset="1" stopColor="#128C7E"/>
            </linearGradient>
          </defs>
        </svg>
      </div>
      <h2 className={styles.emptyTitle}>WhatsApp Web</h2>
      <p className={styles.emptyDesc}>Seleccioná una conversación para ver los mensajes</p>
    </div>
  )
}
