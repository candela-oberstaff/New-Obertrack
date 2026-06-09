import { Message } from '../../../types/chat'
import styles from '../../../pages/SlackChat.module.css'
import { Modal, Button } from '../../ui'

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
    <Modal
      isOpen
      onClose={onClose}
      title="🔍 Buscar mensajes"
      size="md"
      footer={<Button variant="secondary" onClick={onClose}>Cerrar</Button>}
    >
      <div className={styles['search-input-container']}>
        <input
          type="text"
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          placeholder="Escribe para buscar..."
          onKeyDown={(e) => e.key === 'Enter' && onSearch()}
          autoFocus
        />
        <Button onClick={onSearch}>Buscar</Button>
      </div>

      <div className={styles['search-results']}>
        {searchResults.map(msg => (
          <div key={msg.id} className={styles['search-result-item']} onClick={() => { onClose() }}>
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
    </Modal>
  )
}
