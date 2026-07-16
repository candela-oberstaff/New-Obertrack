import { Check } from 'lucide-react'
import type { Board } from '../../../types'
import styles from '../../../pages/Tasks.module.css'
import { Modal, Button } from '../../ui'

interface JoinBoardModalProps {
  isOpen: boolean
  onClose: () => void
  publicBoards: Board[]
  handleRequestJoin: (boardId: number) => void
  isJoiningBoard: boolean
  requestedBoardIds: number[]
}

export function JoinBoardModal({
  isOpen,
  onClose,
  publicBoards,
  handleRequestJoin,
  isJoiningBoard,
  requestedBoardIds,
}: JoinBoardModalProps) {
  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Solicitar unirse a un tablero" size="md">
      <p style={{ color: '#64748b', fontSize: 13, marginTop: 0, marginBottom: 16 }}>
        Los tableros son privados. Tu solicitud queda pendiente hasta que un responsable la apruebe.
      </p>
      {publicBoards.length === 0 ? (
        <div className={styles['no-data-msg'] || 'no-data-msg'}>
          <p>No hay tableros disponibles para solicitar en este momento.</p>
        </div>
      ) : (
        <div className={styles['public-boards-list'] || 'public-boards-list'}>
          {publicBoards.map((board) => {
            const requested = requestedBoardIds.includes(board.id)
            return (
              <div key={board.id} className={styles['public-board-item'] || 'public-board-item'}>
                <div className={styles['board-info'] || 'board-info'}>
                  <div className={styles['board-color'] || 'board-color'} style={{ backgroundColor: board.color }}></div>
                  <div className={styles['board-text'] || 'board-text'}>
                    <span className={styles['board-name'] || 'board-name'}>{board.name}</span>
                    <span className={styles['board-creator'] || 'board-creator'}>Creado por: {board.creator?.name || 'Sistema'}</span>
                  </div>
                </div>
                {requested ? (
                  <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, fontSize: 13, fontWeight: 700, color: '#15803d' }}>
                    <Check size={15} /> Solicitud enviada
                  </span>
                ) : (
                  <Button size="sm" onClick={() => handleRequestJoin(board.id)} loading={isJoiningBoard}>
                    Solicitar unirme
                  </Button>
                )}
              </div>
            )
          })}
        </div>
      )}
    </Modal>
  )
}
