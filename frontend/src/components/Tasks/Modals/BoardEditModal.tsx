import { useEffect, useState } from 'react'
import type { Board } from '../../../types'
import styles from '../../../pages/Tasks.module.css'
import { Modal, Button } from '../../ui'
import { BoardColorPicker } from '../components/BoardColorPicker'

/**
 * Editar un tablero ya creado: nombre y color.
 *
 * Antes esto era un `prompt()` de una sola línea, así que lo único que se podía
 * cambiar era el nombre — el color quedaba congelado en el que se eligió al
 * crearlo, aunque el servidor siempre aceptó cambiarlo. Es un modal y no un
 * prompt precisamente por eso: en un cuadro de texto no cabe un selector de
 * color.
 */

export interface BoardEditValues {
  name: string
  color: string
}

interface BoardEditModalProps {
  isOpen: boolean
  board: Board | null
  onClose: () => void
  onSubmit: (values: BoardEditValues) => Promise<void>
  isSaving?: boolean
}

const FALLBACK_COLOR = '#3b82f6'

export function BoardEditModal({ isOpen, board, onClose, onSubmit, isSaving = false }: BoardEditModalProps) {
  const [name, setName] = useState('')
  const [color, setColor] = useState(FALLBACK_COLOR)

  // Al abrirlo con otro tablero hay que repoblar el formulario: el modal se
  // monta una sola vez y se reutiliza para todos.
  useEffect(() => {
    if (isOpen && board) {
      setName(board.name ?? '')
      setColor(board.color || FALLBACK_COLOR)
    }
  }, [isOpen, board])

  const trimmed = name.trim()
  const dirty = !!board && (trimmed !== (board.name ?? '') || color !== (board.color || FALLBACK_COLOR))

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!trimmed || !dirty) return
    await onSubmit({ name: trimmed, color })
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      isDirty={dirty}
      title="Editar tablero"
      size="sm"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Cancelar</Button>
          <Button type="submit" form="board-edit-form" loading={isSaving} disabled={!trimmed || !dirty}>
            Guardar
          </Button>
        </>
      }
    >
      <form id="board-edit-form" onSubmit={handleSubmit}>
        <div className={styles['form-group']}>
          <label>Nombre del tablero</label>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Nombre del tablero"
            required
            autoFocus
          />
        </div>
        <div className={styles['form-group']}>
          <label>Color</label>
          <BoardColorPicker value={color} onChange={setColor} />
        </div>
      </form>
    </Modal>
  )
}
