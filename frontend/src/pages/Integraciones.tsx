import { useState } from 'react'
import { BarChart3, Bot } from 'lucide-react'
import { useAuth } from '../context/AuthContext'
import AsistentesIA from '../components/Integraciones/AsistentesIA'
import UsoIA from '../components/Integraciones/UsoIA'
import styles from './Integraciones.module.css'

// Integraciones: hoy, los asistentes de IA (servidor MCP de solo lectura).
// Cada persona conecta el suyo; el superadmin además ve el panel "Uso de IA".
export default function Integraciones() {
  const { user } = useAuth()
  const isSuper = !!user?.is_superadmin
  const [tab, setTab] = useState<'asistentes' | 'uso'>('asistentes')

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <h1>Integraciones</h1>
        <p>Conecta tu asistente de IA a Obertrack, siempre en solo lectura.</p>
      </header>

      {isSuper && (
        <nav className={styles.tabs} aria-label="Secciones de Integraciones">
          <button type="button" className={tab === 'asistentes' ? styles.active : ''} onClick={() => setTab('asistentes')}>
            <Bot size={18} /> Asistentes de IA
          </button>
          <button type="button" className={tab === 'uso' ? styles.active : ''} onClick={() => setTab('uso')}>
            <BarChart3 size={18} /> Uso de IA
          </button>
        </nav>
      )}

      {tab === 'uso' && isSuper ? <UsoIA /> : <AsistentesIA />}
    </div>
  )
}
