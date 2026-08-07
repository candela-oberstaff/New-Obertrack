import type { CSSProperties } from 'react'
import { GlobalSearchHit } from '../../../types/chat'
import styles from '../../../pages/SlackChat.module.css'
import { Modal, Button } from '../../ui'

export type SearchScope = 'channel' | 'all'

interface SearchModalProps {
  searchQuery: string
  setSearchQuery: (query: string) => void
  searchResults: GlobalSearchHit[]
  /** Alcance de la búsqueda: el canal abierto o todas las conversaciones. */
  scope: SearchScope
  setScope: (scope: SearchScope) => void
  /** False cuando no hay canal abierto (solo queda la búsqueda global). */
  canSearchChannel: boolean
  onSearch: () => void
  /** Abre el canal del resultado (búsqueda global) y resalta el mensaje. */
  onOpenResult: (hit: GlobalSearchHit) => void
  onClose: () => void
  formatTime: (date: string) => string
}

const scopePill = (active: boolean): CSSProperties => ({
  border: active ? 'none' : '1px solid #e2e8f0',
  background: active ? '#7c3aed' : '#fff',
  color: active ? '#fff' : '#334155',
  borderRadius: 999,
  padding: '5px 12px',
  fontSize: 12,
  fontWeight: 700,
  cursor: 'pointer',
  width: 'auto',
})

export function SearchModal({
  searchQuery,
  setSearchQuery,
  searchResults,
  scope,
  setScope,
  canSearchChannel,
  onSearch,
  onOpenResult,
  onClose,
  formatTime
}: SearchModalProps) {
  const channelLabel = (hit: GlobalSearchHit) => {
    if (scope === 'channel') return null
    const name = hit.channel_type === 'direct' ? 'Mensaje directo' : `#${hit.channel_name || 'canal'}`
    return <span style={{ fontSize: 11, fontWeight: 700, color: '#7c3aed', marginRight: 8 }}>{name}</span>
  }

  return (
    <Modal
      isOpen
      onClose={onClose}
      title="🔍 Buscar mensajes"
      size="md"
      footer={<Button variant="secondary" onClick={onClose}>Cerrar</Button>}
    >
      <div style={{ display: 'flex', gap: 8, marginBottom: 10 }}>
        {canSearchChannel && (
          <button style={scopePill(scope === 'channel')} onClick={() => setScope('channel')}>
            Este canal
          </button>
        )}
        <button style={scopePill(scope === 'all')} onClick={() => setScope('all')}>
          Todas mis conversaciones
        </button>
      </div>

      <div className={styles['search-input-container']}>
        <input
          type="text"
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          placeholder={scope === 'all' ? 'Buscar en todas tus conversaciones...' : 'Escribe para buscar...'}
          onKeyDown={(e) => e.key === 'Enter' && onSearch()}
          autoFocus
        />
        <Button onClick={onSearch}>Buscar</Button>
      </div>

      <div className={styles['search-results']}>
        {searchResults.map(msg => (
          <div
            key={msg.id}
            className={styles['search-result-item']}
            onClick={() => onOpenResult(msg)}
            title={scope === 'all' ? 'Abrir la conversación' : undefined}
          >
            <div className={styles['search-result-header']}>
              <span className={styles['search-result-author']}>
                {channelLabel(msg)}
                {msg.user?.name}
              </span>
              <span className={styles['search-result-time']}>{formatTime(msg.created_at)}</span>
            </div>
            <p className={styles['search-result-content']}>{msg.content}</p>
          </div>
        ))}
        {searchQuery && searchResults.length === 0 && (
          <p className={styles['no-results']}>No se encontraron mensajes</p>
        )}
      </div>
    </Modal>
  )
}
