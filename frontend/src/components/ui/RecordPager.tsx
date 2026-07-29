import { useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { locateRecord, type RecordScope } from '../../lib/recordNav'
import './RecordPager.css'

export interface RecordPagerProps {
  /** Secuencia que dejó anotada el listado de origen. */
  scope: RecordScope
  /** Registro que se está viendo ahora. */
  currentId: number
  /** Ruta del detalle para un id dado. */
  toPath: (id: number) => string
  /** Singular de lo que se recorre, para los tooltips ("empresa", "profesional"). */
  noun?: string
  className?: string
}

/**
 * Paginador "‹ 3 de 12 ›" para recorrer registros desde su ficha, en el mismo
 * orden en que estaban en el listado.
 *
 * No se pinta si el registro no pertenece a la secuencia anotada o si esta tiene
 * un solo elemento: en ese caso no hay nada que recorrer y un par de flechas
 * muertas solo estorban.
 */
export function RecordPager({ scope, currentId, toPath, noun = 'registro', className = '' }: RecordPagerProps) {
  const navigate = useNavigate()
  const { prevId, nextId, position, total } = useMemo(
    () => locateRecord(scope, currentId),
    [scope, currentId],
  )

  if (position === 0 || total < 2) return null

  return (
    <div className={`ui-record-pager ${className}`} role="group" aria-label={`Navegar entre ${noun}s`}>
      <button
        type="button"
        className="ui-record-pager__btn"
        onClick={() => prevId !== null && navigate(toPath(prevId))}
        disabled={prevId === null}
        title={prevId !== null ? `Ver ${noun} anterior` : `Es el primer ${noun} de la lista`}
        aria-label={`Ver ${noun} anterior`}
      >
        <ChevronLeft size={16} />
      </button>
      <span className="ui-record-pager__count" aria-live="polite">
        {position} de {total}
      </span>
      <button
        type="button"
        className="ui-record-pager__btn"
        onClick={() => nextId !== null && navigate(toPath(nextId))}
        disabled={nextId === null}
        title={nextId !== null ? `Ver ${noun} siguiente` : `Es el último ${noun} de la lista`}
        aria-label={`Ver ${noun} siguiente`}
      >
        <ChevronRight size={16} />
      </button>
    </div>
  )
}
