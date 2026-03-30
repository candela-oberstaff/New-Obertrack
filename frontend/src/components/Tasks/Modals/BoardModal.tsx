import { X, Plus } from 'lucide-react'
import type { User } from '../../../types'
import styles from '../../../pages/Tasks.module.css'

interface BoardModalProps {
  isOpen: boolean
  onClose: () => void
  newBoardData: any
  setNewBoardData: (data: any) => void
  assigneeSearch: string
  setAssigneeSearch: (val: string) => void
  filteredUsers: User[]
  newBoardPhaseSearch: string
  setNewBoardPhaseSearch: (val: string) => void
  onSubmit: () => void
  isCreatingBoard: boolean
}

export function BoardModal({
  isOpen,
  onClose,
  newBoardData,
  setNewBoardData,
  assigneeSearch,
  setAssigneeSearch,
  filteredUsers,
  newBoardPhaseSearch,
  setNewBoardPhaseSearch,
  onSubmit,
  isCreatingBoard
}: BoardModalProps) {
  if (!isOpen) return null

  const handlePhaseAdd = () => {
    if (newBoardPhaseSearch.trim()) {
      setNewBoardData({
        ...newBoardData,
        phases: [...newBoardData.phases, { name: newBoardPhaseSearch.trim(), color: '#6b7280' }]
      })
      setNewBoardPhaseSearch('')
    }
  }

  return (
    <div className={styles['modal-overlay']} onClick={onClose}>
      <div className={`${styles['modal']} ${styles['board-modal']}`} onClick={(e) => e.stopPropagation()}>
        <h2>Crear Tablero</h2>
        <form onSubmit={(e) => { e.preventDefault(); onSubmit() }}>
          <div className={styles['form-group']}>
            <label>Nombre del tablero</label>
            <input
              type="text"
              value={newBoardData.name}
              onChange={(e) => setNewBoardData({ ...newBoardData, name: e.target.value })}
              placeholder="Ej: Marketing, IT, Diseño..."
              required
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
                        {user.name.split(' ').map(n => n[0]).join('').substring(0, 2).toUpperCase()}
                      </div>
                      <div className={styles['member-info']}>
                        <span className={styles['member-name']}>{user.name}</span>
                        <span className={styles['member-role']}>{user.user_type === 'empleado' ? 'Profesional' : user.user_type}</span>
                      </div>
                    </div>
                  </label>
                ))}
            </div>
          </div>

          <div className={styles['form-group']}>
            <label>Fases del tablero</label>
            <div className={styles['phases-list']} style={{ maxHeight: '180px', overflowY: 'auto', marginBottom: '12px' }}>
              {newBoardData.phases.map((phase: any, idx: number) => (
                <div key={idx} className={styles['phase-item']}>
                  <div className={styles['phase-color']} style={{ backgroundColor: phase.color }}></div>
                  <span className={styles['phase-name']}>{phase.name}</span>
                  {newBoardData.phases.length > 1 && (
                    <button
                      type="button"
                      className={`${styles['btn-icon']} ${styles['phase-delete'] || 'phase-delete'}`}
                      onClick={() => {
                        setNewBoardData({
                          ...newBoardData,
                          phases: newBoardData.phases.filter((_: any, i: number) => i !== idx)
                        })
                      }}
                      title="Eliminar fase"
                    >
                      <X size={14} />
                    </button>
                  )}
                </div>
              ))}
            </div>
            <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
              <input
                type="text"
                placeholder="Nueva fase..."
                value={newBoardPhaseSearch}
                onChange={(e) => setNewBoardPhaseSearch(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault()
                    handlePhaseAdd()
                  }
                }}
                style={{ flex: 1, padding: '8px 12px', border: '1px solid #e2e8f0', borderRadius: '8px' }}
              />
              <input
                type="color"
                defaultValue="#6b7280"
                style={{ width: '36px', height: '36px', border: 'none', cursor: 'pointer', padding: '2px' }}
              />
              <button
                type="button"
                className={styles['btn-primary']}
                style={{ padding: '8px 12px' }}
                onClick={handlePhaseAdd}
              >
                <Plus size={16} />
              </button>
            </div>
          </div>

          <div className={styles['modal-actions']}>
            <button type="button" onClick={onClose}>Cancelar</button>
            <button type="submit" className={styles['btn-primary']} disabled={isCreatingBoard}>
              {isCreatingBoard ? 'Creando...' : 'Crear Tablero'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
