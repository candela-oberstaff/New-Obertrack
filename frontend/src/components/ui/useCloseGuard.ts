import { useCallback, useRef } from 'react'
import { useConfirmOptional } from './ConfirmProvider'

/**
 * Protege el trabajo sin guardar al cerrar un diálogo.
 *
 * Un clic fuera del modal o un Escape son fáciles de pulsar por accidente, y
 * hasta ahora se llevaban por delante lo que estuvieras escribiendo. Este hook
 * devuelve un cierre que primero pide confirmación —pero SOLO si hay algo que
 * perder—, para no estorbar cuando el diálogo está intacto.
 *
 * Sirve tanto para <Modal> como para los overlays hechos a mano:
 *
 *   const requestClose = useCloseGuard(() => title !== '' || blocks.length > 0, onClose)
 *   <div className="overlay" onClick={requestClose}>
 */
/**
 * Indica si `value` cambió desde que el componente se montó.
 *
 * Evita tener que pasarle a cada modal el estado original por separado: guarda
 * una foto del formulario al abrirse y compara contra ella. Sirve igual para
 * crear (la foto es el formulario vacío) que para editar (la foto son los
 * datos cargados), así cerrar sin tocar nada nunca pregunta.
 */
export function useDirtySnapshot(value: unknown): boolean {
  const baseline = useRef<string | null>(null)
  const current = JSON.stringify(value ?? null)
  if (baseline.current === null) baseline.current = current
  return current !== baseline.current
}

export function useCloseGuard(
  /** Si hay cambios sin guardar. Se lee en el momento del cierre. */
  isDirty: boolean | (() => boolean),
  onClose: () => void,
  options?: { title?: string; message?: string; confirmLabel?: string; cancelLabel?: string }
): () => void {
  const confirm = useConfirmOptional()

  // Por referencia: el cierre se dispara desde handlers que no deben re-crearse
  // en cada tecla que escribe el usuario.
  const dirtyRef = useRef(isDirty)
  dirtyRef.current = isDirty
  const onCloseRef = useRef(onClose)
  onCloseRef.current = onClose
  const optionsRef = useRef(options)
  optionsRef.current = options

  return useCallback(() => {
    const current = dirtyRef.current
    const dirty = typeof current === 'function' ? current() : current

    if (!dirty) {
      onCloseRef.current()
      return
    }

    // Sin proveedor de confirmación (tests, montajes aislados) no se puede
    // preguntar: se respeta el cierre en vez de dejar el diálogo atascado.
    if (!confirm) {
      onCloseRef.current()
      return
    }

    const o = optionsRef.current
    void confirm({
      title: o?.title ?? '¿Descartar lo que hiciste?',
      message: o?.message ?? 'Hay cambios sin guardar. Si cierras ahora se perderán.',
      confirmLabel: o?.confirmLabel ?? 'Descartar y cerrar',
      cancelLabel: o?.cancelLabel ?? 'Seguir editando',
      variant: 'danger',
    }).then(ok => {
      if (ok) onCloseRef.current()
    })
  }, [confirm])
}
