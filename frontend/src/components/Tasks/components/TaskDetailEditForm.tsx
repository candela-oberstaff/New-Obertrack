import { useState, useEffect } from 'react'
import type { Task, User, TaskPriority, TaskStatus } from '../../../types'
import { RichTextEditor } from '../RichTextEditor'
import { ColumnType } from '../types'
import { Select } from '../../ui/Select'
import { X, Check } from 'lucide-react'

interface TaskDetailEditFormProps {
  task: Task
  users: User[]
  columns: ColumnType[]
  onCancel: () => void
  onSave: (data: {
    title: string
    description: string
    priority: TaskPriority
    status: TaskStatus
    start_date: string | undefined
    end_date: string | undefined
    assignees: number[]
  }) => Promise<void>
  isUpdating: boolean
  styles: any
}

export function TaskDetailEditForm({
  task,
  users,
  columns,
  onCancel,
  onSave,
  isUpdating,
  styles
}: TaskDetailEditFormProps) {
  const [assigneeSearch, setAssigneeSearch] = useState('')
  const [formData, setFormData] = useState({
    title: '',
    description: '',
    priority: 'medium' as TaskPriority,
    status: 'por_hacer' as TaskStatus,
    start_date: '',
    end_date: '',
    assignees: [] as number[],
  })

  useEffect(() => {
    setFormData({
      title: task.title,
      description: task.description || '',
      priority: task.priority,
      status: task.status,
      start_date: task.start_date?.split('T')[0] || '',
      end_date: task.end_date?.split('T')[0] || '',
      assignees: task.assignees?.map(a => a.id) || [],
    })
  }, [task])

  const handleSave = async () => {
    await onSave({
      title: formData.title,
      description: formData.description,
      priority: formData.priority,
      status: formData.status,
      start_date: formData.start_date || undefined,
      end_date: formData.end_date || undefined,
      assignees: formData.assignees,
    })
  }

  return (
    <div className={styles['edit-form']}>
      <div className={styles['form-group']}>
        <label>Título</label>
        <input
          type="text"
          value={formData.title}
          onChange={(e) => setFormData({ ...formData, title: e.target.value })}
        />
      </div>
      <div className={styles['form-group']}>
        <label>Descripción</label>
        <RichTextEditor
          value={formData.description}
          onChange={(value) => setFormData({ ...formData, description: value })}
        />
      </div>
      <div className={styles['form-row']}>
        <div className={styles['form-group']}>
          <label>Prioridad</label>
          <Select
            fullWidth
            value={formData.priority}
            onChange={(v) => setFormData({ ...formData, priority: v as TaskPriority })}
            options={[
              { value: 'low', label: 'Baja' },
              { value: 'medium', label: 'Media' },
              { value: 'high', label: 'Alta' },
              { value: 'urgent', label: 'Urgente' },
            ]}
          />
        </div>
        <div className={styles['form-group']}>
          <label>Estado</label>
          <Select
            fullWidth
            value={formData.status}
            onChange={(v) => setFormData({ ...formData, status: v as TaskStatus })}
            options={columns.map(col => ({ value: col.id, label: col.title }))}
          />
        </div>
      </div>
      <div className={styles['form-row']}>
        <div className={styles['form-group']}>
          <label>Fecha inicio</label>
          <input
            type="date"
            value={formData.start_date}
            onChange={(e) => setFormData({ ...formData, start_date: e.target.value })}
          />
        </div>
        <div className={styles['form-group']}>
          <label>Fecha límite</label>
          <input
            type="date"
            value={formData.end_date}
            onChange={(e) => setFormData({ ...formData, end_date: e.target.value })}
          />
        </div>
      </div>
      <div className={styles['form-group']}>
        <label>Asignados</label>
        <div className={styles['assignees-edit-list'] || 'assignees-edit-list'}>
          {formData.assignees.length > 0 && (
            <div className={styles['assigned-chips'] || 'assigned-chips'}>
              {formData.assignees.map((id) => {
                const user = users.find(u => u.id === id)
                if (!user) return null
                return (
                  <div key={user.id} className={styles['assignee-chip'] || 'assignee-chip'}>
                    <span>{user.name || user.email}</span>
                    <button
                      type="button"
                      className={styles['remove-assignee'] || 'remove-assignee'}
                      onClick={() => setFormData({
                        ...formData,
                        assignees: formData.assignees.filter(aid => aid !== user.id)
                      })}
                    >
                      <X size={14} />
                    </button>
                  </div>
                )
              })}
            </div>
          )}
          <div className={styles['assignee-dropdown-wrapper'] || 'assignee-dropdown-wrapper'}>
            <input
              type="text"
              className={styles['assignee-search'] || 'assignee-search'}
              placeholder="Buscar usuario..."
              value={assigneeSearch}
              onChange={(e) => setAssigneeSearch(e.target.value)}
            />
            <div className={styles['assignee-dropdown'] || 'assignee-dropdown'}>
              {users
                .filter(u =>
                  u.name?.toLowerCase().includes(assigneeSearch.toLowerCase()) ||
                  u.email?.toLowerCase().includes(assigneeSearch.toLowerCase())
                )
                .sort((a, b) => (a.name || a.email).localeCompare(b.name || b.email))
                .map(user => {
                  const isAssigned = formData.assignees.includes(user.id)
                  return (
                    <div
                      key={user.id}
                      className={`${styles['assignee-option'] || 'assignee-option'} ${isAssigned ? (styles['assigned'] || 'assigned') : ''}`}
                      onClick={() => {
                        if (isAssigned) {
                          setFormData({
                            ...formData,
                            assignees: formData.assignees.filter(aid => aid !== user.id)
                          })
                        } else {
                          setFormData({
                            ...formData,
                            assignees: [...formData.assignees, user.id]
                          })
                        }
                      }}
                    >
                      <div className={styles['chip-avatar'] || 'chip-avatar'}>
                        {user.name?.charAt(0).toUpperCase() || '?'}
                      </div>
                      <span className={styles['assignee-name'] || 'assignee-name'}>
                        {user.name || user.email}
                      </span>
                      {isAssigned && (
                        <span className={styles['assignee-check'] || 'assignee-check'}>
                          <Check size={14} />
                        </span>
                      )}
                    </div>
                  )
                })}
            </div>
          </div>
        </div>
      </div>
      <div className={styles['form-actions']}>
        <button onClick={onCancel} disabled={isUpdating}>Cancelar</button>
        <button className={styles['btn-primary']} onClick={handleSave} disabled={isUpdating}>
          {isUpdating ? 'Guardando...' : 'Guardar'}
        </button>
      </div>
    </div>
  )
}
