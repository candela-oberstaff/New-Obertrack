import React from 'react'
import { X, Paperclip } from 'lucide-react'
import { RichTextEditor } from '../RichTextEditor'
import type { User } from '../../../types'
import styles from '../../../pages/Tasks.module.css'

interface NewTaskModalProps {
  isOpen: boolean
  onClose: () => void
  newTaskData: any
  setNewTaskData: React.Dispatch<React.SetStateAction<any>>
  isCreatingTask: boolean
  onSubmit: (e: React.FormEvent) => void
  assigneeSearch: string
  setAssigneeSearch: (val: string) => void
  filteredUsers: User[]
}

export function NewTaskModal({
  isOpen,
  onClose,
  newTaskData,
  setNewTaskData,
  isCreatingTask,
  onSubmit,
  assigneeSearch,
  setAssigneeSearch,
  filteredUsers
}: NewTaskModalProps) {
  if (!isOpen) return null

  return (
    <div className={styles['modal-overlay']} onClick={onClose}>
      <div className={styles['new-task-modal']} onClick={(e) => e.stopPropagation()}>
        <div className={styles['new-task-header']}>
          <h2>Crear nueva tarea</h2>
          <button className={styles['close-btn']} onClick={onClose}>
            <X size={24} />
          </button>
        </div>

        <form onSubmit={onSubmit} id="create-task-form" className={styles['new-task-content']}>
          <div className={styles['task-form-main']}>
            <div className={styles['form-field']}>
              <label>Título</label>
              <input
                type="text"
                value={newTaskData.title}
                onChange={(e) => setNewTaskData({ ...newTaskData, title: e.target.value })}
                placeholder="¿Qué necesitas hacer?"
                className={styles['title-field']}
                required
                autoFocus
              />
            </div>

            <div className={styles['form-field']}>
              <label>Descripción</label>
              <RichTextEditor
                value={newTaskData.description}
                onChange={(value) => setNewTaskData({ ...newTaskData, description: value })}
                placeholder="Agrega detalles, listas, enlaces..."
              />
            </div>
          </div>

          <div className={styles['task-form-side']}>
            <div className={styles['settings-card']}>
              <h3>Configuración</h3>

              <div className={styles['setting-item']}>
                <label>Prioridad</label>
                <select
                  value={newTaskData.priority}
                  onChange={(e) => setNewTaskData({ ...newTaskData, priority: e.target.value })}
                  className={styles['setting-select']}
                >
                  <option value="low">Baja</option>
                  <option value="medium">Media</option>
                  <option value="high">Alta</option>
                  <option value="urgent">Urgente</option>
                </select>
              </div>

              <div className={styles['setting-item']}>
                <label>Fecha límite</label>
                <input
                  type="date"
                  value={newTaskData.end_date}
                  onChange={(e) => setNewTaskData({ ...newTaskData, end_date: e.target.value })}
                  className={styles['setting-input']}
                />
              </div>

              <div className={styles['setting-item']}>
                <label>Adjuntos</label>
                <div className={styles['new-task-attachments']}>
                  <input
                    type="file"
                    id="new-task-file-input"
                    multiple
                    onChange={(e) => {
                      const files = Array.from(e.target.files || [])
                      setNewTaskData((prev: any) => ({
                        ...prev,
                        attachments: [...prev.attachments, ...files]
                      }))
                    }}
                    style={{ display: 'none' }}
                  />
                  <button
                    type="button"
                    className={`${styles['btn-secondary']} ${styles['btn-sm']}`}
                    onClick={() => document.getElementById('new-task-file-input')?.click()}
                    style={{ width: '100%', marginBottom: '8px' }}
                  >
                    <Paperclip size={14} /> Seleccionar archivos
                  </button>

                  {newTaskData.attachments.length > 0 && (
                    <div className={styles['selected-files-preview']}>
                      {newTaskData.attachments.map((file: File, idx: number) => (
                        <div key={idx} className={styles['selected-file-item']}>
                          <span className={styles['file-name']} title={file.name}>{file.name}</span>
                          <button
                            type="button"
                            className={styles['remove-file-btn']}
                            onClick={() => {
                              setNewTaskData((prev: any) => ({
                                ...prev,
                                attachments: prev.attachments.filter((_: any, i: number) => i !== idx)
                              }))
                            }}
                          >
                            <X size={14} />
                          </button>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            </div>

            <div className={styles['assign-card']}>
              <h3>Asignar a</h3>
              <input
                type="text"
                placeholder="Buscar usuario..."
                value={assigneeSearch}
                onChange={(e) => setAssigneeSearch(e.target.value)}
                className={styles['assignee-search']}
              />
              <div className={styles['assignees-scroll']}>
                {filteredUsers.length === 0 ? (
                  <p className={styles['no-users']}>No hay usuarios disponibles</p>
                ) : (
                  filteredUsers.map((user) => (
                    <label key={user.id} className={styles['assign-option']}>
                      <input
                        type="checkbox"
                        checked={newTaskData.assignees.includes(user.id)}
                        onChange={(e) => {
                          if (e.target.checked) {
                            setNewTaskData({
                              ...newTaskData,
                              assignees: [...newTaskData.assignees, user.id],
                            })
                          } else {
                            setNewTaskData({
                              ...newTaskData,
                              assignees: newTaskData.assignees.filter((id: number) => id !== user.id),
                            })
                          }
                        }}
                      />
                      <span className={styles['user-chip']}>
                        <span className={styles['chip-avatar']}>{user.name?.charAt(0).toUpperCase()}</span>
                        <span className={styles['chip-name']}>{user.name}</span>
                      </span>
                    </label>
                  ))
                )}
              </div>
            </div>
          </div>
        </form>

        <div className={styles['new-task-footer']}>
          <button type="button" onClick={onClose} className={styles['btn-cancel']} disabled={isCreatingTask}>
            Cancelar
          </button>
          <button type="submit" form="create-task-form" className={styles['btn-create']} disabled={isCreatingTask}>
            {isCreatingTask ? 'Creando...' : 'Crear tarea'}
          </button>
        </div>
      </div>
    </div>
  )
}
