import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Pencil, Trash2, Plus } from 'lucide-react'
import { Modal, Button } from '../ui'
import { adminService, type EmergencyTemplate } from '../../services/admin.service'
import styles from './EmergencyTemplatesModal.module.css'

interface Props {
  isOpen: boolean
  onClose: () => void
}

interface FormState {
  title: string
  subject: string
  body: string
}

const EMPTY: FormState = { title: '', subject: '', body: '' }

export function EmergencyTemplatesModal({ isOpen, onClose }: Props) {
  const qc = useQueryClient()
  const [editingId, setEditingId] = useState<number | null>(null)
  const [form, setForm] = useState<FormState>(EMPTY)
  const [error, setError] = useState<string | null>(null)

  const { data, isLoading } = useQuery({
    queryKey: ['emergency-templates'],
    queryFn: adminService.getEmergencyTemplates,
  })
  const templates = data?.templates ?? []

  const reset = () => {
    setEditingId(null)
    setForm(EMPTY)
    setError(null)
  }

  const invalidate = () => qc.invalidateQueries({ queryKey: ['emergency-templates'] })

  const saveMutation = useMutation({
    mutationFn: () =>
      editingId == null
        ? adminService.createEmergencyTemplate(form)
        : adminService.updateEmergencyTemplate(editingId, form),
    onSuccess: () => {
      invalidate()
      reset()
    },
    onError: () => setError('No se pudo guardar la plantilla.'),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => adminService.deleteEmergencyTemplate(id),
    onSuccess: (_res, id) => {
      invalidate()
      if (editingId === id) reset()
    },
  })

  const startEdit = (t: EmergencyTemplate) => {
    setEditingId(t.id)
    setForm({ title: t.title, subject: t.subject, body: t.body })
    setError(null)
  }

  const canSave = form.title.trim() && form.subject.trim() && form.body.trim()

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Gestionar plantillas" size="md">
      <div className={styles.wrap}>
        <div className={styles.list}>
          {isLoading ? (
            <p className={styles.empty}>Cargando…</p>
          ) : templates.length === 0 ? (
            <p className={styles.empty}>No hay plantillas guardadas todavía.</p>
          ) : (
            templates.map((t) => (
              <div key={t.id} className={`${styles.item} ${editingId === t.id ? styles.active : ''}`}>
                <div className={styles.itemMain}>
                  <div className={styles.itemTitle}>{t.title}</div>
                  <div className={styles.itemSubject}>{t.subject}</div>
                </div>
                <div className={styles.itemActions}>
                  <button type="button" className={styles.iconBtn} onClick={() => startEdit(t)} aria-label="Editar">
                    <Pencil size={15} />
                  </button>
                  <button
                    type="button"
                    className={`${styles.iconBtn} ${styles.danger}`}
                    onClick={() => deleteMutation.mutate(t.id)}
                    disabled={deleteMutation.isPending}
                    aria-label="Eliminar"
                  >
                    <Trash2 size={15} />
                  </button>
                </div>
              </div>
            ))
          )}
        </div>

        <div className={styles.form}>
          <span className={styles.formHead}>{editingId == null ? 'Nueva plantilla' : 'Editar plantilla'}</span>

          <label className={styles.label}>Título</label>
          <input
            className={styles.input}
            value={form.title}
            onChange={(e) => setForm((f) => ({ ...f, title: e.target.value }))}
          />

          <label className={styles.label}>Asunto</label>
          <input
            className={styles.input}
            value={form.subject}
            onChange={(e) => setForm((f) => ({ ...f, subject: e.target.value }))}
          />

          <label className={styles.label}>Mensaje</label>
          <textarea
            className={styles.textarea}
            rows={5}
            value={form.body}
            onChange={(e) => setForm((f) => ({ ...f, body: e.target.value }))}
          />

          {error && <p className={styles.error}>{error}</p>}

          <div className={styles.formActions}>
            {editingId != null && (
              <Button variant="ghost" onClick={reset}>
                Cancelar
              </Button>
            )}
            <Button
              leftIcon={editingId == null ? <Plus size={15} /> : undefined}
              onClick={() => saveMutation.mutate()}
              loading={saveMutation.isPending}
              disabled={!canSave}
            >
              {editingId == null ? 'Crear plantilla' : 'Guardar cambios'}
            </Button>
          </div>
        </div>
      </div>
    </Modal>
  )
}
