import { useState } from 'react'
import { Plus, Pencil, Trash2, Layers, Video, ListChecks } from 'lucide-react'

import { useConfirm } from '../ui/ConfirmProvider'
import { useNotification } from '../../context/NotificationContext'
import { inductionService, type InductionBlock, type InductionProgram } from '../../services/induction.service'
import type { Tutorial } from '../../types/tutorials'
import InductionBlockEditor from './InductionBlockEditor'
import styles from './InductionSettings.module.css'

interface Props {
  blocks: InductionBlock[]
  tutorials: Tutorial[]
  /** Mínimo por defecto del programa por defecto, para mostrar el efectivo. */
  fallbackPassingScore: number
  /** Programas existentes, para copiar sus insignias. */
  programs?: InductionProgram[]
  onChanged: () => Promise<void> | void
}

/**
 * Biblioteca de bloques: cada uno es un video más su cuestionario, y puede
 * usarse en cualquier programa. Desde aquí se crean, editan y borran.
 */
export default function InductionBlockLibrary({ blocks, tutorials, fallbackPassingScore, programs = [], onChanged }: Props) {
  const { success, error: showError } = useNotification()
  const confirm = useConfirm()

  // null = lista; 'new' = creando; objeto = editando.
  const [editing, setEditing] = useState<InductionBlock | 'new' | null>(null)

  const handleDelete = async (block: InductionBlock) => {
    if (block.program_names.length > 0) {
      showError(`Este bloque está en uso en: ${block.program_names.join(', ')}. Quítalo de esos programas antes de borrarlo.`)
      return
    }
    const ok = await confirm({
      title: 'Borrar bloque',
      message: `Se borrará el bloque "${block.name}". Su cuestionario se conserva en Encuestas.`,
      confirmLabel: 'Borrar',
    })
    if (!ok) return
    try {
      await inductionService.deleteBlock(block.id)
      success('Bloque borrado.')
      await onChanged()
    } catch (err: any) {
      showError(err?.response?.data?.error ?? 'No se pudo borrar el bloque.')
    }
  }

  if (editing) {
    return (
      <InductionBlockEditor
        block={editing === 'new' ? null : editing}
        tutorials={tutorials}
        fallbackPassingScore={fallbackPassingScore}
        allBlocks={blocks}
        allPrograms={programs}
        onSaved={async (saved) => {
          setEditing(saved)
          await onChanged()
        }}
        onBack={() => setEditing(null)}
      />
    )
  }

  return (
    <div>
      <div className={styles.toolbar}>
        <p className={styles.intro}>
          Un bloque es un video con su propio cuestionario. Se arma una vez y se reutiliza en los
          programas que haga falta.
        </p>
        <button type="button" className={styles.saveBtn} onClick={() => setEditing('new')}>
          <Plus size={16} /> Nuevo bloque
        </button>
      </div>

      {blocks.length === 0 ? (
        <div className={styles.empty}>
          Todavía no hay bloques. Crea el primero para armar un programa.
          <div style={{ marginTop: 12 }}>
            <button type="button" className={styles.ghostBtn} onClick={() => setEditing('new')}>
              <Plus size={16} /> Crear el primer bloque
            </button>
          </div>
        </div>
      ) : (
        <div className={styles.list}>
          {blocks.map((b) => {
            const scorable = b.question_count > 0
            return (
              <div key={b.id} className={styles.row}>
                <div className={styles.rowIcon}>
                  <Layers size={18} />
                </div>
                <div className={styles.rowMain}>
                  <div className={styles.rowTitle}>
                    {b.name}
                    {b.passing_score !== null && b.passing_score !== undefined && (
                      <span className={styles.tag}>Mínimo {b.passing_score}%</span>
                    )}
                  </div>
                  <div className={styles.rowMeta}>
                    <span className={styles.tag}>
                      <Video size={12} style={{ verticalAlign: -2, marginRight: 4 }} />
                      {b.tutorial_title || 'Sin video'}
                    </span>
                    <span className={scorable ? styles.tagOk : styles.tagWarn}>
                      <ListChecks size={12} style={{ verticalAlign: -2, marginRight: 4 }} />
                      {b.question_count} {b.question_count === 1 ? 'pregunta' : 'preguntas'}
                    </span>
                    {b.program_names.length > 0 ? (
                      <span>En: {b.program_names.join(', ')}</span>
                    ) : (
                      <span>Sin usar en ningún programa</span>
                    )}
                  </div>
                </div>
                <div className={styles.rowActions}>
                  <button
                    type="button"
                    className={`${styles.iconBtn} ${styles.iconBtnNeutral}`}
                    title="Editar"
                    aria-label={`Editar ${b.name}`}
                    onClick={() => setEditing(b)}
                  >
                    <Pencil size={16} />
                  </button>
                  <button
                    type="button"
                    className={styles.iconBtn}
                    title={b.program_names.length > 0 ? 'En uso: quítalo de sus programas para borrarlo' : 'Borrar'}
                    aria-label={`Borrar ${b.name}`}
                    disabled={b.program_names.length > 0}
                    onClick={() => handleDelete(b)}
                  >
                    <Trash2 size={16} />
                  </button>
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
