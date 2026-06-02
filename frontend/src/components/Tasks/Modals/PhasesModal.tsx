import React, { useState } from 'react'
import { X, GripVertical, Plus } from 'lucide-react'
import type { Board } from '../../../types'
import styles from '../../../pages/Tasks.module.css'

const DEFAULT_PHASE_COLOR = '#6b7280'

interface PhasesModalProps {
  isOpen: boolean
  onClose: () => void
  selectedBoard: Board | null
  draggingPhase: number | null
  dragOverIdx: number | null
  handlePhaseDragStart: (phaseId: number) => void
  handlePhaseDragEnter: (idx: number) => void
  handlePhaseDragEnd: () => void
  isDraggingPhasesRef: React.MutableRefObject<boolean>
  onAddPhase: (phase: { name: string; color: string }) => Promise<void>
  onRemovePhase: (phaseId: number) => Promise<void>
  isSavingPhase: boolean
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
  isDraggingPhasesRef,
  onAddPhase,
  onRemovePhase,
  isSavingPhase
}: PhasesModalProps) {
  const [newPhaseName, setNewPhaseName] = useState('')
  const [newPhaseColor, setNewPhaseColor] = useState(DEFAULT_PHASE_COLOR)

  if (!isOpen || !selectedBoard) return null

  const colorForInput = (color: string) =>
    color?.startsWith('#') ? color : DEFAULT_PHASE_COLOR

  const handleAdd = async () => {
    if (!newPhaseName.trim() || isSavingPhase) return
    await onAddPhase({ name: newPhaseName.trim(), color: newPhaseColor })
    setNewPhaseName('')
    setNewPhaseColor(DEFAULT_PHASE_COLOR)
  }

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
            >
              <span
                className={styles['drag-handle'] || 'drag-handle'}
                style={{ cursor: 'grab' }}
                onMouseDown={() => handlePhaseDragStart(phase.id)}
              ><GripVertical size={16} /></span>
              <div className={styles['phase-color']} style={{ backgroundColor: phase.color }}></div>
              <span className={styles['phase-name']}>{phase.name}</span>
              <button
                type="button"
                className={styles['phase-delete']}
                onClick={() => onRemovePhase(phase.id)}
                disabled={isSavingPhase || (selectedBoard.phases?.length || 0) <= 1}
                title={(selectedBoard.phases?.length || 0) <= 1 ? 'Debe haber al menos una fase' : 'Eliminar fase'}
              >
                <X size={16} />
              </button>
            </div>
          ))}
        </div>

        <div className={styles['add-phase-form'] || 'add-phase-form'}>
          <input
            type="color"
            value={colorForInput(newPhaseColor)}
            onChange={(e) => setNewPhaseColor(e.target.value)}
            title="Color de la fase"
            style={{ width: '32px', height: '32px', padding: 0, border: 'none', background: 'none', cursor: 'pointer', flexShrink: 0 }}
          />
          <input
            type="text"
            value={newPhaseName}
            onChange={(e) => setNewPhaseName(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); handleAdd() } }}
            placeholder="Nueva fase..."
            style={{ flex: 1, border: '1px solid #e2e8f0', borderRadius: '8px', padding: '8px 12px' }}
          />
          <button
            type="button"
            className={styles['btn-primary']}
            onClick={handleAdd}
            disabled={isSavingPhase || !newPhaseName.trim()}
            style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', whiteSpace: 'nowrap' }}
          >
            <Plus size={16} /> {isSavingPhase ? 'Guardando...' : 'Agregar'}
          </button>
        </div>
      </div>
    </div>
  )
}
