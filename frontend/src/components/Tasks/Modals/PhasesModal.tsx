import React, { useState } from 'react'
import { X, GripVertical, Plus } from 'lucide-react'
import { boardService } from '../../../services/api'
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
  setBoards,
  setSelectedBoard,
  draggingPhase,
  dragOverIdx,
  handlePhaseDragStart,
  handlePhaseDragEnter,
  handlePhaseDragEnd,
  isDraggingPhasesRef
}: PhasesModalProps) {
  const [isDeletingPhase, setIsDeletingPhase] = useState(false)
  const [isAddingPhase, setIsAddingPhase] = useState(false)

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
              <button
                className={`${styles['btn-icon']} ${styles['phase-delete'] || 'phase-delete'}`}
                onClick={(e) => {
                  e.stopPropagation()
                  if (!window.confirm('¿Eliminar esta fase?')) return
                  setIsDeletingPhase(true)
                  boardService.removePhase(selectedBoard.id, phase.id)
                    .then(() => boardService.getAll())
                    .then((boardsRes: Board[]) => {
                      setBoards(boardsRes)
                      const found = boardsRes.find((b: Board) => b.id === selectedBoard.id)
                      if (found) setSelectedBoard(found)
                    })
                    .catch((error: any) => {
                      window.alert(error.response?.data?.error || 'Error al eliminar fase')
                    })
                    .finally(() => setIsDeletingPhase(false))
                }}
                title={isDeletingPhase ? "Eliminando..." : "Eliminar fase"}
                disabled={isDeletingPhase}
              >
                {isDeletingPhase ? '...' : <X size={14} />}
              </button>
            </div>
          ))}
        </div>

        <div className={styles['add-phase-form'] || 'add-phase-form'} onMouseUp={handlePhaseDragEnd}>
          <input
            type="text"
            placeholder="Nombre de la nueva fase"
            id="new-phase-name"
            style={{ flex: 1, padding: '10px', border: '1px solid #e2e8f0', borderRadius: '8px', marginRight: '8px' }}
          />
          <input
            type="color"
            id="new-phase-color"
            defaultValue="#6b7280"
            style={{ width: '40px', height: '40px', border: 'none', cursor: 'pointer' }}
          />
          <button
            className={styles['btn-primary']}
            style={{ marginLeft: '8px' }}
            disabled={isAddingPhase}
            onClick={async () => {
              const nameInput = document.getElementById('new-phase-name') as HTMLInputElement
              const colorInput = document.getElementById('new-phase-color') as HTMLInputElement
              const name = nameInput.value.trim()
              const color = colorInput.value
              if (!name) return
              setIsAddingPhase(true)
              try {
                await boardService.addPhase(selectedBoard.id, { name, color })
                const boardsRes = await boardService.getAll()
                setBoards(boardsRes)
                const found = boardsRes.find(b => b.id === selectedBoard.id)
                if (found) setSelectedBoard(found)
                nameInput.value = ''
              } catch (error) {
                console.error('Error adding phase:', error)
              } finally {
                setIsAddingPhase(false)
              }
            }}
          >
            {isAddingPhase ? 'Agregando...' : <Plus size={18} />}
          </button>
        </div>
      </div>
    </div>
  )
}
