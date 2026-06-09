import { Modal, Button } from '../../ui'
import { Select } from '../../ui/Select'
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
    <Modal
      isOpen
      onClose={onClose}
      title="Crear canal"
      size="sm"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Cancelar</Button>
          <Button onClick={onCreate} disabled={!newChannel.name.trim()}>Crear canal</Button>
        </>
      }
    >
      <div className={styles['form-group']}>
        <label>Nombre del canal</label>
        <input
          type="text"
          value={newChannel.name}
          onChange={(e) => setNewChannel({ ...newChannel, name: e.target.value })}
          placeholder="ej: general"
          autoFocus
        />
      </div>
      <div className={styles['form-group']}>
        <label>Descripción (opcional)</label>
        <input
          type="text"
          value={newChannel.description}
          onChange={(e) => setNewChannel({ ...newChannel, description: e.target.value })}
          placeholder="¿De qué trata este canal?"
        />
      </div>
      <div className={styles['form-group']}>
        <label>Tipo</label>
        <Select
          fullWidth
          value={newChannel.type}
          onChange={(v) => setNewChannel({ ...newChannel, type: v as 'public' | 'private' })}
          options={[
            { value: 'public', label: 'Público' },
            { value: 'private', label: 'Privado' },
          ]}
        />
      </div>
    </Modal>
  )
}
