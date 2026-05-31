import { Message } from '../../../types/chat'
import styles from '../../../pages/SlackChat.module.css'

interface SearchModalProps {
  searchQuery: string
  setSearchQuery: (query: string) => void
  searchResults: Message[]
  onSearch: () => void
  onClose: () => void
  formatTime: (date: string) => string
}

export function SearchModal({
  searchQuery,
  setSearchQuery,
  searchResults,
  onSearch,
  onClose,
  formatTime
}: SearchModalProps) {
  return (
    <div className={styles['modal-overlay']} onClick={onClose}>
      <div className={`${styles['modal-content']} ${styles['search']}`} onClick={e => e.stopPropagation()}>
        <div className={styles['modal-header']}>
          <h2>🔍 Buscar mensajes</h2>
          <button className={styles['close-btn']} onClick={onClose}>×</button>
        </div>

        <div className={styles['search-input-container']}>
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Escribe para buscar..."
            onKeyDown={(e) => e.key === 'Enter' && onSearch()}
            autoFocus
          />
          <button className={styles['btn-primary']} onClick={onSearch}>Buscar</button>
        </div>
        
        <div className={styles['search-results']}>
          {searchResults.map(msg => (
            <div key={msg.id} className={styles['search-result-item']} onClick={() => { onClose(); }}>
              <div className={styles['search-result-header']}>
                <span className={styles['search-result-author']}>{msg.user?.name}</span>
                <span className={styles['search-result-time']}>{formatTime(msg.created_at)}</span>
              </div>
              <p className={styles['search-result-content']}>{msg.content}</p>
            </div>
          ))}
          {searchQuery && searchResults.length === 0 && (
            <p className={styles['no-results']}>No se encontraron mensajes</p>
          )}
        </div>
        
        <div className={styles['modal-actions']}>
          <button onClick={onClose}>Cerrar</button>
        </div>
      </div>
    </div>
  )
}
