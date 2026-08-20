import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { tutorialService } from '../../services/api'
import { useAuth } from '../../context/AuthContext'
import { NovedadOverlay } from './NovedadOverlay'
import type { Tutorial } from '../../types'

/**
 * Aviso de novedades a pantalla completa. Al entrar a la app pregunta qué
 * novedades anunciadas no ha visto todavía esta persona y las pone delante de
 * todo, una a una: publicar algo no sirve de nada si hay que acordarse de
 * entrar a la sección a buscarlo.
 *
 * Cerrar el aviso cuenta como haberla visto (la misma marca que pone abrir la
 * tarjeta en /novedades), así que no vuelve a aparecer. Las que exigen
 * confirmación solo se apagan con el acuse explícito.
 *
 * Vive montado en el Layout: aparece en cualquier pantalla, no solo al entrar.
 * La presentación está en NovedadOverlay, que el formulario reusa para
 * previsualizar.
 */
export function NovedadAnnouncer() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const qc = useQueryClient()
  // IDs ya despachados en esta sesión. Se llevan aparte de la lista del
  // servidor porque esa se recarga sola: con un índice, una recarga a mitad de
  // la cola haría reaparecer lo que ya se cerró.
  const [dismissed, setDismissed] = useState<number[]>([])

  const { data: pending } = useQuery({
    queryKey: ['tutorial-pending'],
    queryFn: () => tutorialService.getPending(),
    enabled: !!user,
    // Se repregunta sola: quien ya estaba dentro cuando se publicó la novedad
    // la ve sin tener que recargar, aunque el aviso en vivo no llegue.
    refetchInterval: 60_000,
    refetchOnWindowFocus: true,
    staleTime: 30_000,
    retry: false,
  })

  const queue: Tutorial[] = (pending ?? []).filter(t => !dismissed.includes(t.id))
  const current = queue[0]
  // Posición dentro de la tanda: lo ya cerrado más lo que queda. Se cuenta así
  // y no sobre la lista del servidor porque esa se vacía a medida que se
  // registran las vistas, y el contador daría marcha atrás.
  const total = dismissed.length + queue.length
  const position = dismissed.length + 1

  // Marca la novedad como vista y pasa a la siguiente. El registro es
  // best-effort: si falla, el aviso simplemente reaparecerá en la próxima
  // entrada, que es preferible a bloquear el cierre.
  const acknowledge = useCallback(async (tutorial: Tutorial) => {
    setDismissed(prev => (prev.includes(tutorial.id) ? prev : [...prev, tutorial.id]))
    try {
      // Origen 'anuncio': así las métricas distinguen a quien se enteró por el
      // aviso de quien fue a buscarla a la sección. El acuse solo se sella en
      // las novedades que lo exigen.
      await tutorialService.recordView(tutorial.id, 'anuncio', tutorial.require_ack)
      qc.setQueryData<number[]>(['tutorial-views'], old => Array.from(new Set([...(old ?? []), tutorial.id])))
    } catch (error) {
      console.error('No se pudo registrar la vista de la novedad:', error)
    }
  }, [qc])

  // Una novedad publicada con la app abierta llega por WebSocket a la
  // campanita; aquí se aprovecha ese mismo aviso para releer los pendientes al
  // instante, sin esperar a la siguiente pasada del intervalo.
  useEffect(() => {
    const onPublished = () => { void qc.invalidateQueries({ queryKey: ['tutorial-pending'] }) }
    window.addEventListener('novedad-published', onPublished)
    return () => window.removeEventListener('novedad-published', onPublished)
  }, [qc])

  // Cada aparición se cuenta una sola vez por novedad y sesión. Es lo que
  // sostiene el tope de veces: sin esto, recargar la página lo esquivaría.
  const reported = useRef<Set<number>>(new Set())
  useEffect(() => {
    if (!current || reported.current.has(current.id)) return
    reported.current.add(current.id)
    void tutorialService.recordShow(current.id).catch(() => {})
  }, [current])

  if (!current) return null

  const handleCTA = () => {
    void tutorialService.recordClick(current.id).catch(() => {})
    const isExternal = /^https?:\/\//.test(current.cta_url)
    // Con un acuse pendiente el aviso no se puede cerrar, así que el destino se
    // abre aparte: la persona actúa y vuelve a confirmar.
    if (isExternal || current.require_ack) {
      window.open(current.cta_url, '_blank', 'noopener,noreferrer')
      return
    }
    void acknowledge(current)
    navigate(current.cta_url)
  }

  // "Ver todas" lleva a la sección: ahí quedan las demás, así que se cierra la
  // cola entera en lugar de encadenar avisos encima de la página.
  const goToNovedades = () => {
    void acknowledge(current)
    setDismissed(queue.map(t => t.id))
    navigate('/novedades')
  }

  return (
    <NovedadOverlay
      tutorial={current}
      position={position}
      total={total}
      onDismiss={() => void acknowledge(current)}
      onCTA={handleCTA}
      onSeeAll={goToNovedades}
    />
  )
}
