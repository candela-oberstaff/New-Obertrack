import { Message } from '../../../types/chat'

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
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content search" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h2>🔍 Buscar mensajes</h2>
          <button className="close-btn" onClick={onClose}>×</button>
        </div>

        <div className="search-input-container">
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Escribe para buscar..."
            onKeyDown={(e) => e.key === 'Enter' && onSearch()}
            autoFocus
          />
          <button className="btn-primary" onClick={onSearch}>Buscar</button>
        </div>
        
        <div className="search-results">
          {searchResults.map(msg => (
            <div key={msg.id} className="search-result-item" onClick={() => { onClose(); }}>
              <div className="search-result-header">
                <span className="search-result-author">{msg.user?.name}</span>
                <span className="search-result-time">{formatTime(msg.created_at)}</span>
              </div>
              <p className="search-result-content">{msg.content}</p>
            </div>
          ))}
          {searchQuery && searchResults.length === 0 && (
            <p className="no-results">No se encontraron mensajes</p>
          )}
        </div>
        
        <div className="modal-actions">
          <button onClick={onClose}>Cerrar</button>
        </div>
      </div>
    </div>
  )
}
