import { useCallback, useEffect, useState } from 'react'
import { X, Zap } from 'lucide-react'
import type { Board } from '../../../types'
import { BoardRecipes } from '../../Workflows/BoardRecipes'
import overlay from '../../../pages/Tasks.module.css'
import styles from './BoardAutomationsModal.module.css'

// Automatizaciones del tablero abierto. Es un modal y no una pantalla porque no hay
// nada que elegir al entrar: el tablero ya está decidido y cada interruptor guarda
// solo, así que no hay borrador que perder ni botón de guardar que pulsar. Se cierra
// tocando fuera o con Escape, como el resto de modales del tablero.

interface BoardAutomationsModalProps {
  isOpen: boolean
  onClose: () => void
  selectedBoard: Board | null
  /** Cuántas quedan encendidas, para el distintivo del botón que abre esto. Se avisa
   *  a cada cambio y no sólo al cerrar: el interruptor guarda solo, así que el número
   *  de fuera tiene que moverse a la vez que el de dentro. */
  onActiveCountChange?: (active: number) => void
}

export function BoardAutomationsModal({ isOpen, onClose, selectedBoard, onActiveCountChange }: BoardAutomationsModalProps) {
  const [count, setCount] = useState<{ active: number; total: number } | null>(null)

  // Estable: la lista lo tiene como dependencia de un efecto y una función nueva en
  // cada render la haría recargarse en bucle.
  const handleCount = useCallback((active: number, total: number) => {
    setCount({ active, total })
    onActiveCountChange?.(active)
  }, [onActiveCountChange])

  useEffect(() => {
    if (!isOpen) return
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [isOpen, onClose])

  if (!isOpen || !selectedBoard) return null

  return (
    <div className={overlay['modal-overlay']} onClick={onClose}>
      <div
        className={styles['panel']}
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-label={`Automatizaciones de ${selectedBoard.name}`}
      >
        <div className={styles['header']}>
          <span className={styles['badge']} aria-hidden="true">
            <Zap size={19} />
          </span>
          <div className={styles['heading']}>
            <h2>Automatizaciones</h2>
            <p>
              {selectedBoard.name}
              {count && count.total > 0 && (
                <> · <strong>{count.active}</strong> de {count.total} activas</>
              )}
            </p>
          </div>
          <button className={styles['close']} onClick={onClose} aria-label="Cerrar">
            <X size={20} />
          </button>
        </div>

        <div className={styles['body']}>
          <BoardRecipes
            boardId={selectedBoard.id}
            phases={selectedBoard.phases ?? []}
            onCountChange={handleCount}
          />
        </div>

        <div className={styles['footer']}>
          Los cambios se guardan solos y valen sólo para este tablero.
        </div>
      </div>
    </div>
  )
}
