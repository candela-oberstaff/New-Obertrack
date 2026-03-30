import { X } from 'lucide-react'
import type { Board } from '../../../types'
import styles from '../../../pages/Tasks.module.css'

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
  if (!isOpen) return null

  return (
    <div className={styles['modal-overlay']} onClick={onClose}>
      <div className={`${styles['modal']} ${styles['join-board-modal']}`} onClick={(e) => e.stopPropagation()}>
        <div className={styles['modal-header']}>
          <h2>Unirse a un Tablero</h2>
          <button className={styles['close-btn']} onClick={onClose}><X size={20} /></button>
        </div>
        <div className={styles['modal-body'] || 'modal-body'}>
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
                  <button
                    className={`${styles['btn-primary']} ${styles['btn-sm'] || 'btn-sm'}`}
                    onClick={() => handleJoinBoard(board.id)}
                    disabled={isJoiningBoard}
                  >
                    {isJoiningBoard ? 'Uniendo...' : 'Unirse'}
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
