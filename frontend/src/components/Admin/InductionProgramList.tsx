import { useState } from 'react'
import { Plus, Pencil, Trash2, Route, Building2, Layers, Star } from 'lucide-react'

import { useConfirm } from '../ui/ConfirmProvider'
import { useNotification } from '../../context/NotificationContext'
import { inductionService, type InductionBlock, type InductionProgram } from '../../services/induction.service'
import type { TutorialAudienceOption } from '../../types/tutorials'
import InductionProgramEditor from './InductionProgramEditor'
import styles from './InductionSettings.module.css'

interface Props {
  programs: InductionProgram[]
  library: InductionBlock[]
  companies: TutorialAudienceOption[]
  onChanged: () => Promise<void> | void
  /** Lleva a la biblioteca cuando no hay bloques con los que armar un programa. */
  onGoToBlocks?: () => void
}

/**
 * Programas de inducción: secuencias de bloques asignadas por empresa. Uno es
 * el por defecto, que reciben las empresas sin asignación.
 */
export default function InductionProgramList({ programs, library, companies, onChanged, onGoToBlocks }: Props) {
  const { success, error: showError } = useNotification()
  const confirm = useConfirm()

  // null = lista; 'new' = creando; número = editando ese programa.
  const [editing, setEditing] = useState<number | 'new' | null>(null)
  const [busyId, setBusyId] = useState<number | null>(null)

  const handleDelete = async (program: InductionProgram) => {
    const ok = await confirm({
      title: 'Borrar programa',
      message: `Se borrará el programa "${program.name}". Sus bloques siguen en la biblioteca y sus empresas pasan al programa por defecto.`,
      confirmLabel: 'Borrar',
    })
    if (!ok) return
    try {
      await inductionService.deleteProgram(program.id)
      success('Programa borrado.')
      await onChanged()
    } catch (err: any) {
      showError(err?.response?.data?.error ?? 'No se pudo borrar el programa.')
    }
  }

  // Cambiar el programa por defecto desde la lista, sin abrir el editor: es
  // el paso previo obligatorio para poder borrar el que lo era.
  const handleMakeDefault = async (program: InductionProgram) => {
    if (!program.is_active || program.block_count === 0) {
      showError('El programa por defecto debe estar activo y tener al menos un bloque.')
      return
    }
    const ok = await confirm({
      title: 'Programa por defecto',
      message: `"${program.name}" pasará a ser el programa que reciben las empresas sin asignación. El actual dejará de serlo.`,
      confirmLabel: 'Marcar por defecto',
    })
    if (!ok) return
    setBusyId(program.id)
    try {
      await inductionService.updateProgram(program.id, {
        name: program.name,
        description: program.description,
        default_passing_score: program.default_passing_score,
        max_attempts: program.max_attempts,
        is_active: true,
        is_default: true,
      })
      success(`"${program.name}" es ahora el programa por defecto.`)
      await onChanged()
    } catch (err: any) {
      showError(err?.response?.data?.error ?? 'No se pudo cambiar el programa por defecto.')
    } finally {
      setBusyId(null)
    }
  }

  if (editing !== null) {
    return (
      <InductionProgramEditor
        programId={editing === 'new' ? null : editing}
        library={library}
        companies={companies}
        allPrograms={programs}
        onSaved={async (saved) => {
          setEditing(saved.id)
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
          Un programa es la lista ordenada de bloques que recorre el profesional. Asigna programas
          a empresas; las que no tengan uno reciben el programa por defecto.
        </p>
        <button type="button" className={styles.saveBtn} onClick={() => setEditing('new')}>
          <Plus size={16} /> Nuevo programa
        </button>
      </div>

      {library.length === 0 && (
        <div className={styles.warn}>
          <Layers size={16} style={{ flexShrink: 0, marginTop: 1 }} />
          <span>
            Todavía no hay bloques. Un programa se arma con bloques de la biblioteca.{' '}
            {onGoToBlocks && (
              <button type="button" className={styles.linkBtn} onClick={onGoToBlocks}>
                Crear el primer bloque
              </button>
            )}
          </span>
        </div>
      )}

      {programs.length === 0 ? (
        <div className={styles.empty}>Todavía no hay programas. Crea el primero: será el programa por defecto.</div>
      ) : (
        <div className={styles.list}>
          {programs.map((p) => (
            <div key={p.id} className={styles.row}>
              <div className={styles.rowIcon}>
                <Route size={18} />
              </div>
              <div className={styles.rowMain}>
                <div className={styles.rowTitle}>
                  {p.name}
                  {p.is_default && <span className={styles.tagPrimary}>Por defecto</span>}
                  {!p.is_active && <span className={styles.tagWarn}>Apagado</span>}
                </div>
                <div className={styles.rowMeta}>
                  <span className={p.block_count > 0 ? styles.tagOk : styles.tagWarn}>
                    <Layers size={12} style={{ verticalAlign: -2, marginRight: 4 }} />
                    {p.block_count} {p.block_count === 1 ? 'bloque' : 'bloques'}
                  </span>
                  <span className={styles.tag}>
                    <Building2 size={12} style={{ verticalAlign: -2, marginRight: 4 }} />
                    {p.is_default
                      ? p.company_count > 0
                        ? `${p.company_count} asignadas + las demás`
                        : 'Todas las empresas sin asignación'
                      : `${p.company_count} ${p.company_count === 1 ? 'empresa' : 'empresas'}`}
                  </span>
                  <span>
                    Mínimo {p.default_passing_score}% · {p.max_attempts} intentos por bloque
                  </span>
                </div>
              </div>
              <div className={styles.rowActions}>
                {!p.is_default && (
                  <button
                    type="button"
                    className={`${styles.iconBtn} ${styles.iconBtnStar}`}
                    title="Marcar como programa por defecto"
                    aria-label="Marcar como programa por defecto"
                    disabled={busyId === p.id}
                    onClick={() => handleMakeDefault(p)}
                  >
                    <Star size={16} />
                  </button>
                )}
                <button
                  type="button"
                  className={`${styles.iconBtn} ${styles.iconBtnNeutral}`}
                  title="Editar"
                  aria-label={`Editar ${p.name}`}
                  onClick={() => setEditing(p.id)}
                >
                  <Pencil size={16} />
                </button>
                <button
                  type="button"
                  className={styles.iconBtn}
                  title={p.is_default ? 'El programa por defecto no se borra: marca otro por defecto primero' : 'Borrar'}
                  aria-label={`Borrar ${p.name}`}
                  disabled={p.is_default}
                  onClick={() => handleDelete(p)}
                >
                  <Trash2 size={16} />
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
      {programs.some((p) => p.is_default) && programs.length === 1 && (
        <p className={styles.hint}>
          El programa por defecto no se puede borrar. Para reemplazarlo, crea otro y márcalo por
          defecto con la estrella.
        </p>
      )}
    </div>
  )
}
