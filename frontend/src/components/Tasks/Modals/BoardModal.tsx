import { Plus, X } from 'lucide-react'
import type { User } from '../../../types'
import styles from '../../../pages/Tasks.module.css'
import { Modal, Button } from '../../ui'

const DEFAULT_PHASE_COLOR = '#6b7280'

interface PhaseInput {
  name: string
  color: string
}

interface BoardFormData {
  name: string
  description: string
  color: string
  member_ids: number[]
  phases: PhaseInput[]
}

interface BoardModalProps {
  isOpen: boolean
  onClose: () => void
  newBoardData: BoardFormData
  setNewBoardData: (data: BoardFormData) => void
  assigneeSearch: string
  setAssigneeSearch: (val: string) => void
  filteredUsers: User[]
  newBoardPhaseSearch: string
  setNewBoardPhaseSearch: (val: string) => void
  onSubmit: (data: BoardFormData) => Promise<void>
  isCreatingBoard: boolean
}

export function BoardModal(props: BoardModalProps) {
  const {
    isOpen,
    onClose,
    newBoardData,
    setNewBoardData,
    assigneeSearch,
    setAssigneeSearch,
    filteredUsers,
    onSubmit,
    isCreatingBoard
  } = props

  // For the native color input (only understands hex), fall back to a default
  // when a phase color is a CSS variable like 'var(--primary)'.
  const colorForInput = (color: string) =>
    color?.startsWith('#') ? color : DEFAULT_PHASE_COLOR

  const updatePhase = (idx: number, patch: Partial<PhaseInput>) => {
    const phases = newBoardData.phases.map((p, i) => (i === idx ? { ...p, ...patch } : p))
    setNewBoardData({ ...newBoardData, phases })
  }

  const addPhase = () => {
    setNewBoardData({
      ...newBoardData,
      phases: [...newBoardData.phases, { name: '', color: DEFAULT_PHASE_COLOR }],
    })
  }

  const removePhase = (idx: number) => {
    setNewBoardData({
      ...newBoardData,
      phases: newBoardData.phases.filter((_, i) => i !== idx),
    })
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Crear Tablero"
      size="md"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Cancelar</Button>
          <Button type="submit" form="board-form" loading={isCreatingBoard}>Crear Tablero</Button>
        </>
      }
    >
      <form id="board-form" onSubmit={(e) => { e.preventDefault(); onSubmit(newBoardData) }}>
        <div className={styles['form-group']}>
          <label>Nombre del tablero</label>
          <input
            type="text"
            value={newBoardData.name}
            onChange={(e) => setNewBoardData({ ...newBoardData, name: e.target.value })}
            placeholder="Ej: Marketing, IT, Diseño..."
            required
            autoFocus
          />
        </div>
        <div className={styles['form-group']}>
          <label>Descripción</label>
          <textarea
            value={newBoardData.description}
            onChange={(e) => setNewBoardData({ ...newBoardData, description: e.target.value })}
            placeholder="Descripción opcional..."
            rows={2}
          />
        </div>
        <div className={styles['form-group']}>
          <label>Color</label>
          <input
            type="color"
            value={newBoardData.color}
            onChange={(e) => setNewBoardData({ ...newBoardData, color: e.target.value })}
          />
        </div>
        <div className={styles['form-group']}>
          <label>Agregar miembros (opcional)</label>
          <input
            type="text"
            placeholder="Buscar usuarios..."
            value={assigneeSearch}
            onChange={(e) => setAssigneeSearch(e.target.value)}
            style={{ width: '100%', padding: '8px 12px', marginBottom: '8px', border: '1px solid #e2e8f0', borderRadius: '8px' }}
          />
          <div className={styles['members-select']} style={{ maxHeight: '200px', overflowY: 'auto' }}>
            {filteredUsers
              .slice()
              .sort((a, b) => a.name.localeCompare(b.name))
              .map(user => (
                <label key={user.id} className={styles['member-checkbox']}>
                  <div className={styles['left-section']}>
                    <input
                      type="checkbox"
                      checked={newBoardData.member_ids.includes(user.id)}
                      onChange={(e) => {
                        if (e.target.checked) {
                          setNewBoardData({
                            ...newBoardData,
                            member_ids: [...newBoardData.member_ids, user.id]
                          })
                        } else {
                          setNewBoardData({
                            ...newBoardData,
                            member_ids: newBoardData.member_ids.filter((id: number) => id !== user.id)
                          })
                        }
                      }}
                    />
                    <span className={styles['checkbox-custom']}></span>
                    <div className={styles['member-avatar']}>
                      {user.name.split(' ').map((n: string) => n[0]).join('').substring(0, 2).toUpperCase()}
                    </div>
                    <div className={styles['member-info']}>
                      <span className={styles['member-name']}>{user.name}</span>
                      <span className={styles['member-role']}>
                        {user.user_type === 'empleado' || user.user_type === 'profesional'
                          ? 'Profesional'
                          : user.user_type === 'empleador'
                          ? 'Empresa'
                          : user.user_type === 'superadmin'
                          ? 'Super Admin'
                          : user.user_type}
                      </span>
                    </div>
                  </div>
                </label>
              ))}
          </div>
        </div>

        <div className={styles['form-group']}>
          <label>Fases del tablero</label>
          <div className={styles['phases-list']} style={{ maxHeight: '220px', overflowY: 'auto', marginBottom: '12px' }}>
            {newBoardData.phases.map((phase: PhaseInput, idx: number) => (
              <div key={idx} className={styles['phase-item']} style={{ cursor: 'default' }}>
                <input
                  type="color"
                  value={colorForInput(phase.color)}
                  onChange={(e) => updatePhase(idx, { color: e.target.value })}
                  title="Color de la fase"
                  style={{ width: '32px', height: '32px', padding: 0, border: 'none', background: 'none', cursor: 'pointer', flexShrink: 0 }}
                />
                <input
                  type="text"
                  value={phase.name}
                  onChange={(e) => updatePhase(idx, { name: e.target.value })}
                  placeholder="Nombre de la fase"
                  className={styles['phase-name']}
                  style={{ border: '1px solid #e2e8f0', borderRadius: '6px', padding: '6px 10px' }}
                />
                <button
                  type="button"
                  className={styles['phase-delete']}
                  onClick={() => removePhase(idx)}
                  disabled={newBoardData.phases.length <= 1}
                  title={newBoardData.phases.length <= 1 ? 'Debe haber al menos una fase' : 'Eliminar fase'}
                >
                  <X size={16} />
                </button>
              </div>
            ))}
          </div>
          <Button type="button" variant="secondary" size="sm" onClick={addPhase} leftIcon={<Plus size={16} />}>
            Agregar fase
          </Button>
        </div>
      </form>
    </Modal>
  )
}
