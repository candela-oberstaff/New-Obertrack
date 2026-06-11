import { useMemo } from 'react'
import { Check, AlertCircle, PlayCircle, HardDrive } from 'lucide-react'
import { TUTORIAL_ICON_NAMES, TutorialIcon } from '../icons'
import { parseVideoUrl, getProviderLabel } from '../utils'
import { Modal, Button } from '../../ui'
import type { CreateTutorialInput, TutorialAudience } from '../../../types'
import styles from '../../../pages/Tutoriales.module.css'

interface TutorialFormModalProps {
  isOpen: boolean
  isEditing: boolean
  isSaving: boolean
  formData: CreateTutorialInput
  setFormData: React.Dispatch<React.SetStateAction<CreateTutorialInput>>
  availableCategories: string[]
  onClose: () => void
  onSubmit: (e: React.FormEvent) => void
}

export function TutorialFormModal({
  isOpen,
  isEditing,
  isSaving,
  formData,
  setFormData,
  availableCategories,
  onClose,
  onSubmit,
}: TutorialFormModalProps) {
  const urlInfo = useMemo(() => parseVideoUrl(formData.google_drive_url), [formData.google_drive_url])
  const urlState: 'idle' | 'valid' | 'invalid' =
    !formData.google_drive_url.trim() ? 'idle' : urlInfo ? 'valid' : 'invalid'

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={isEditing ? 'Editar tutorial' : 'Nuevo tutorial'}
      size="md"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={isSaving}>Cancelar</Button>
          <Button type="submit" form="tutorial-form" loading={isSaving}>
            {isEditing ? 'Guardar cambios' : 'Crear tutorial'}
          </Button>
        </>
      }
    >
      <form onSubmit={onSubmit} id="tutorial-form" className={styles['tutorial-form-body']}>
        <div className={styles['tutorial-form-field']}>
          <label>Título</label>
          <input
            type="text"
            value={formData.title}
            onChange={(e) => setFormData({ ...formData, title: e.target.value })}
            placeholder="Ej: Registrar horas de trabajo"
            required
            autoFocus
          />
        </div>

        <div className={styles['tutorial-form-field']}>
          <label>Descripción</label>
          <textarea
            value={formData.description}
            onChange={(e) => setFormData({ ...formData, description: e.target.value })}
            placeholder="Resumen breve de lo que enseña el tutorial"
            rows={3}
          />
        </div>

        <div className={styles['tutorial-form-field']}>
          <label>Link del video</label>
          <div className={`${styles['tutorial-url-input-wrapper']} ${styles[`url-${urlState}`]}`}>
            <input
              type="url"
              value={formData.google_drive_url}
              onChange={(e) => setFormData({ ...formData, google_drive_url: e.target.value })}
              placeholder="https://drive.google.com/file/d/... o https://youtu.be/..."
              required
            />
            {urlState === 'valid' && urlInfo && (
              <span className={`${styles['tutorial-url-badge']} ${styles[`provider-${urlInfo.provider}`]}`}>
                {urlInfo.provider === 'youtube' ? <PlayCircle size={14} /> : <HardDrive size={14} />}
                {getProviderLabel(urlInfo.provider)}
                <Check size={14} />
              </span>
            )}
            {urlState === 'invalid' && (
              <span className={`${styles['tutorial-url-badge']} ${styles['provider-invalid']}`}>
                <AlertCircle size={14} />
                No válido
              </span>
            )}
          </div>
          <small className={styles['tutorial-form-hint']}>
            Solo se aceptan links de <strong>Google Drive</strong> (<code>/file/d/{'{ID}'}/</code>) o <strong>YouTube</strong> (públicos o no listados). No se permiten otras redes.
          </small>
        </div>

        <div className={styles['tutorial-form-row']}>
          <div className={styles['tutorial-form-field']}>
            <label>Categoría</label>
            <input
              type="text"
              list="tutorial-categories"
              value={formData.category}
              onChange={(e) => setFormData({ ...formData, category: e.target.value })}
              placeholder="Ej: Onboarding, Tareas..."
            />
            <datalist id="tutorial-categories">
              {availableCategories.map((cat) => (
                <option key={cat} value={cat} />
              ))}
            </datalist>
          </div>
          <div className={styles['tutorial-form-field']}>
            <label>Duración (min)</label>
            <input
              type="number"
              min={0}
              value={formData.duration_min}
              onChange={(e) => setFormData({ ...formData, duration_min: Number(e.target.value) })}
            />
          </div>
        </div>

        <div className={styles['tutorial-form-field']}>
          <label>Dirigido a</label>
          <select
            value={formData.audience}
            onChange={(e) => setFormData({ ...formData, audience: e.target.value as TutorialAudience })}
          >
            <option value="all">Todos (empresas y profesionales)</option>
            <option value="empleador">Solo empresas</option>
            <option value="profesional">Solo profesionales</option>
          </select>
          <small className={styles['tutorial-form-hint']}>
            Define el alcance de visibilidad: las empresas y los profesionales solo verán los tutoriales dirigidos a su tipo de cuenta.
          </small>
        </div>

        <div className={styles['tutorial-form-field']}>
          <label>Icono</label>
          <div className={styles['tutorial-icon-grid']}>
            {TUTORIAL_ICON_NAMES.map((name) => (
              <button
                key={name}
                type="button"
                className={`${styles['tutorial-icon-option']} ${formData.icon_name === name ? styles['selected'] : ''}`}
                onClick={() => setFormData({ ...formData, icon_name: name })}
                title={name}
              >
                <TutorialIcon name={name} size={20} />
              </button>
            ))}
          </div>
        </div>

        <label className={styles['tutorial-form-toggle']}>
          <input
            type="checkbox"
            checked={formData.is_active}
            onChange={(e) => setFormData({ ...formData, is_active: e.target.checked })}
          />
          <span>Visible para todos los usuarios</span>
        </label>
      </form>
    </Modal>
  )
}
