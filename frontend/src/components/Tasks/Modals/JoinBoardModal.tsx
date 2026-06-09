import type { Board } from '../../../types'
import styles from '../../../pages/Tasks.module.css'
import { Modal, Button } from '../../ui'

interface JoinBoardModalProps {
  isOpen: boolean
  onClose: () => void
  publicBoards: Board[]
  handleJoinBoard: (boardId: number) => void
  isJoiningBoard: boolean
}

export function JoinBoardModal({
  isOpen,
  onClose,
  publicBoards,
  handleJoinBoard,
  isJoiningBoard
}: JoinBoardModalProps) {
  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Unirse a un Tablero" size="md">
      {publicBoards.length === 0 ? (
        <div className={styles['no-data-msg'] || 'no-data-msg'}>
          <p>No hay tableros públicos disponibles para unirse en este momento.</p>
        </div>
      ) : (
        <div className={styles['public-boards-list'] || 'public-boards-list'}>
          {publicBoards.map(board => (
            <div key={board.id} className={styles['public-board-item'] || 'public-board-item'}>
              <div className={styles['board-info'] || 'board-info'}>
                <div className={styles['board-color'] || 'board-color'} style={{ backgroundColor: board.color }}></div>
                <div className={styles['board-text'] || 'board-text'}>
                  <span className={styles['board-name'] || 'board-name'}>{board.name}</span>
                  <span className={styles['board-creator'] || 'board-creator'}>Creado por: {board.creator?.name || 'Sistema'}</span>
                </div>
              </div>
              <Button size="sm" onClick={() => handleJoinBoard(board.id)} loading={isJoiningBoard}>
                Unirse
              </Button>
            </div>
          ))}
        </div>
      )}
    </Modal>
  )
}
