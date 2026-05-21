import React from 'react'
import { X, GripVertical } from 'lucide-react'
import type { Board } from '../../../types'
import styles from '../../../pages/Tasks.module.css'

interface PhasesModalProps {
  isOpen: boolean
  onClose: () => void
  selectedBoard: Board | null
  setBoards: (boards: Board[]) => void
  setSelectedBoard: (board: Board) => void
  draggingPhase: number | null
  dragOverIdx: number | null
  handlePhaseDragStart: (phaseId: number) => void
  handlePhaseDragEnter: (idx: number) => void
  handlePhaseDragEnd: () => void
  isDraggingPhasesRef: React.MutableRefObject<boolean>
}

export function PhasesModal({
  isOpen,
  onClose,
  selectedBoard,
  draggingPhase,
  dragOverIdx,
  handlePhaseDragStart,
  handlePhaseDragEnter,
  handlePhaseDragEnd,
  isDraggingPhasesRef
}: PhasesModalProps) {
  if (!isOpen || !selectedBoard) return null

  return (
    <div className={styles['modal-overlay']} onClick={onClose} onMouseUp={handlePhaseDragEnd}>
      <div className={`${styles['modal']} ${styles['board-modal']} ${styles['phases-modal'] || 'phases-modal'}`} onClick={(e) => e.stopPropagation()} onMouseUp={handlePhaseDragEnd}>
        <div className={styles['board-modal-header'] || 'board-modal-header'}>
          <h2>Gestionar Fases</h2>
          <button className={styles['close-btn']} onClick={onClose}><X size={20} /></button>
        </div>
        <p className={styles['board-subtitle'] || 'board-subtitle'}>{selectedBoard.name}</p>

        <div
          className={styles['phases-list']}
          onMouseMove={(e) => {
            if (!isDraggingPhasesRef.current) return
            const target = e.target as HTMLElement
            const phaseItem = target.closest(`.${styles['phase-item'] || 'phase-item'}`)
            if (phaseItem) {
              const idx = parseInt(phaseItem.getAttribute('data-idx') || '-1')
              if (idx >= 0) handlePhaseDragEnter(idx)
            }
          }}
          onMouseUp={handlePhaseDragEnd}
          onMouseLeave={(e) => {
            if (isDraggingPhasesRef.current) {
              const relatedTarget = e.relatedTarget as HTMLElement
              if (!relatedTarget || !e.currentTarget.contains(relatedTarget)) {
                handlePhaseDragEnd()
              }
            }
          }}
        >
          {selectedBoard.phases?.map((phase: any, idx: number) => (
            <div
              key={phase.id}
              data-idx={idx}
              className={`${styles['phase-item']} ${draggingPhase === phase.id ? (styles['dragging'] || 'dragging') : ''} ${dragOverIdx === idx ? (styles['drag-over'] || 'drag-over') : ''}`}
              onMouseDown={() => handlePhaseDragStart(phase.id)}
            >
              <span className={styles['drag-handle'] || 'drag-handle'} style={{ cursor: 'grab' }}><GripVertical size={16} /></span>
              <div className={styles['phase-color']} style={{ backgroundColor: phase.color }}></div>
              <span className={styles['phase-name']}>{phase.name}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
