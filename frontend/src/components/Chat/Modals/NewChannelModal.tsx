import styles from '../../../pages/SlackChat.module.css'

interface NewChannelModalProps {
  newChannel: { name: string; description: string; type: 'public' | 'private' }
  setNewChannel: (channel: any) => void
  onClose: () => void
  onCreate: () => void
}

export function NewChannelModal({
  newChannel,
  setNewChannel,
  onClose,
  onCreate
}: NewChannelModalProps) {
  return (
    <div className={styles['modal-overlay']} onClick={onClose}>
      <div className={styles['modal-content']} onClick={e => e.stopPropagation()}>
        <div className={styles['modal-header']}>
          <h2>Crear canal</h2>
          <button className={styles['close-btn']} onClick={onClose}>×</button>
        </div>
        
        <div className={styles['form-group']}>
          <label>Nombre del canal</label>
          <input
            type="text"
            value={newChannel.name}
            onChange={(e) => setNewChannel({...newChannel, name: e.target.value})}
            placeholder="ej: general"
          />
        </div>
        <div className={styles['form-group']}>
          <label>Descripción (opcional)</label>
          <input
            type="text"
            value={newChannel.description}
            onChange={(e) => setNewChannel({...newChannel, description: e.target.value})}
            placeholder="¿De qué trata este canal?"
          />
        </div>
        <div className={styles['form-group']}>
          <label>Tipo</label>
          <select
            value={newChannel.type}
            onChange={(e) => setNewChannel({...newChannel, type: e.target.value as 'public' | 'private'})}
          >
            <option value="public">Público</option>
            <option value="private">Privado</option>
          </select>
        </div>
        <div className={styles['modal-actions']}>
          <button onClick={onClose}>Cancelar</button>
          <button className={styles['btn-primary']} onClick={onCreate}>Crear canal</button>
        </div>
      </div>
    </div>
  )
}
