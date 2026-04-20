import { useEffect, useRef, useState } from 'react'
import { useAuth } from '../context/AuthContext'
import styles from './GoogleChat.module.css'

const GOOGLE_CHAT_URL = 'https://chat.google.com'

type WindowStatus = 'opening' | 'open' | 'closed' | 'blocked'

export default function GoogleChat() {
  const chatWindowRef = useRef<Window | null>(null)
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const [status, setStatus] = useState<WindowStatus>('opening')
  const [openCount, setOpenCount] = useState(0)

  const { user } = useAuth()

  const openChatWindow = () => {
    const width = Math.min(1200, window.screen.availWidth - 100)
    const height = Math.min(820, window.screen.availHeight - 100)
    const left = (window.screen.availWidth - width) / 2
    const top = (window.screen.availHeight - height) / 2

    const authUrl = user?.email 
      ? `${GOOGLE_CHAT_URL}?authuser=${encodeURIComponent(user.email)}` 
      : GOOGLE_CHAT_URL

    const win = window.open(
      authUrl,
      'google-chat-window',
      `width=${width},height=${height},left=${left},top=${top},toolbar=no,menubar=no,scrollbars=yes,resizable=yes,status=no`
    )

    if (!win) {
      setStatus('blocked')
      return
    }

    chatWindowRef.current = win
    setStatus('open')
    setOpenCount(c => c + 1)

    // Poll to detect when the popup is closed
    if (pollRef.current) clearInterval(pollRef.current)
    pollRef.current = setInterval(() => {
      if (chatWindowRef.current?.closed) {
        setStatus('closed')
        clearInterval(pollRef.current!)
      }
    }, 800)
  }

  const focusChatWindow = () => {
    if (chatWindowRef.current && !chatWindowRef.current.closed) {
      chatWindowRef.current.focus()
    } else {
      openChatWindow()
    }
  }

  useEffect(() => {
    openChatWindow()
    return () => {
      if (pollRef.current) clearInterval(pollRef.current)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return (
    <div className={styles.page}>
      {/* Animated background */}
      <div className={styles.bgOrbs}>
        <div className={styles.orb1} />
        <div className={styles.orb2} />
        <div className={styles.orb3} />
      </div>

      <div className={styles.card}>
        {/* Google Chat logo */}
        <div className={styles.logoWrap}>
          <svg viewBox="0 0 48 48" className={styles.logo} fill="none" xmlns="http://www.w3.org/2000/svg">
            <rect width="48" height="48" rx="12" fill="url(#gc-grad)" />
            <path d="M24 10C16.268 10 10 16.268 10 24c0 3.866 1.566 7.366 4.1 9.9L12 38l5.1-1.9A13.92 13.92 0 0 0 24 38c7.732 0 14-6.268 14-14S31.732 10 24 10Z" fill="white" fillOpacity=".9"/>
            <path d="M18 23h8M18 27h5" stroke="#1a73e8" strokeWidth="2" strokeLinecap="round"/>
            <defs>
              <linearGradient id="gc-grad" x1="0" y1="0" x2="48" y2="48" gradientUnits="userSpaceOnUse">
                <stop stopColor="#1a73e8"/>
                <stop offset="1" stopColor="#0d47a1"/>
              </linearGradient>
            </defs>
          </svg>
        </div>

        <h1 className={styles.title}>Google Chat</h1>
        <p className={styles.subtitle}>Tu espacio de trabajo en Google Workspace</p>

        {/* Status indicator */}
        <div className={styles.statusBadge} data-status={status}>
          <span className={styles.statusDot} />
          <span className={styles.statusText}>
            {status === 'opening' && 'Abriendo Google Chat...'}
            {status === 'open' && 'Google Chat está abierto'}
            {status === 'closed' && 'La ventana fue cerrada'}
            {status === 'blocked' && 'Ventana emergente bloqueada'}
          </span>
        </div>

        {/* Main action */}
        {status === 'open' ? (
          <button
            id="btn-focus-google-chat"
            className={styles.btnPrimary}
            onClick={focusChatWindow}
          >
            <svg viewBox="0 0 20 20" fill="currentColor" className={styles.btnIcon}>
              <path d="M11 3a1 1 0 100 2h2.586l-6.293 6.293a1 1 0 101.414 1.414L15 6.414V9a1 1 0 102 0V4a1 1 0 00-1-1h-5z"/>
              <path d="M5 5a2 2 0 00-2 2v8a2 2 0 002 2h8a2 2 0 002-2v-3a1 1 0 10-2 0v3H5V7h3a1 1 0 000-2H5z"/>
            </svg>
            Traer al frente
          </button>
        ) : (
          <button
            id="btn-open-google-chat"
            className={styles.btnPrimary}
            onClick={openChatWindow}
          >
            <svg viewBox="0 0 20 20" fill="currentColor" className={styles.btnIcon}>
              <path fillRule="evenodd" d="M18 10c0 3.866-3.582 7-8 7a8.841 8.841 0 01-4.083-.98L2 17l1.338-3.123C2.493 12.767 2 11.434 2 10c0-3.866 3.582-7 8-7s8 3.134 8 7zM7 9H5v2h2V9zm8 0h-2v2h2V9zM9 9h2v2H9V9z" clipRule="evenodd"/>
            </svg>
            {status === 'blocked' ? 'Abrir en nueva pestaña' : 'Abrir Google Chat'}
          </button>
        )}

        {status === 'blocked' && (
          <div className={styles.blockedNote}>
            <svg viewBox="0 0 20 20" fill="currentColor" className={styles.infoIcon}>
              <path fillRule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clipRule="evenodd"/>
            </svg>
            Tu navegador bloqueó la ventana emergente. Permití las ventanas emergentes para este sitio o usá el botón para abrir en una nueva pestaña.
          </div>
        )}

        {/* Info grid */}
        <div className={styles.infoGrid}>
          <div className={styles.infoItem}>
            <svg viewBox="0 0 20 20" fill="currentColor" className={styles.infoItemIcon}>
              <path d="M13 6a3 3 0 11-6 0 3 3 0 016 0zM18 8a2 2 0 11-4 0 2 2 0 014 0zM14 15a4 4 0 00-8 0v3h8v-3zM6 8a2 2 0 11-4 0 2 2 0 014 0zM16 18v-3a5.972 5.972 0 00-.75-2.906A3.005 3.005 0 0119 15v3h-3zM4.75 12.094A5.973 5.973 0 004 15v3H1v-3a3 3 0 013.75-2.906z"/>
            </svg>
            <div>
              <span className={styles.infoLabel}>Spaces</span>
              <span className={styles.infoDesc}>Canales y grupos de trabajo</span>
            </div>
          </div>
          <div className={styles.infoItem}>
            <svg viewBox="0 0 20 20" fill="currentColor" className={styles.infoItemIcon}>
              <path fillRule="evenodd" d="M18 5v8a2 2 0 01-2 2h-5l-5 4v-4H4a2 2 0 01-2-2V5a2 2 0 012-2h12a2 2 0 012 2zM7 8H5v2h2V8zm2 0h2v2H9V8zm6 0h-2v2h2V8z" clipRule="evenodd"/>
            </svg>
            <div>
              <span className={styles.infoLabel}>Mensajes directos</span>
              <span className={styles.infoDesc}>Chats privados 1 a 1</span>
            </div>
          </div>
          <div className={styles.infoItem}>
            <svg viewBox="0 0 20 20" fill="currentColor" className={styles.infoItemIcon}>
              <path fillRule="evenodd" d="M4 4a2 2 0 012-2h4.586A2 2 0 0112 2.586L15.414 6A2 2 0 0116 7.414V16a2 2 0 01-2 2H6a2 2 0 01-2-2V4z" clipRule="evenodd"/>
            </svg>
            <div>
              <span className={styles.infoLabel}>Archivos</span>
              <span className={styles.infoDesc}>Integrado con Google Drive</span>
            </div>
          </div>
          <div className={styles.infoItem}>
            <svg viewBox="0 0 20 20" fill="currentColor" className={styles.infoItemIcon}>
              <path d="M2 6a2 2 0 012-2h6a2 2 0 012 2v8a2 2 0 01-2 2H4a2 2 0 01-2-2V6zM14.553 7.106A1 1 0 0014 8v4a1 1 0 00.553.894l2 1A1 1 0 0018 13V7a1 1 0 00-1.447-.894l-2 1z"/>
            </svg>
            <div>
              <span className={styles.infoLabel}>Google Meet</span>
              <span className={styles.infoDesc}>Videollamadas integradas</span>
            </div>
          </div>
        </div>

        {openCount > 0 && (
          <p className={styles.sessionNote}>
            Sesión iniciada {user?.email ? `como ${user.email} ` : ''}· Google Chat se abre usando tu cuenta de Google Workspace
          </p>
        )}
      </div>
    </div>
  )
}
