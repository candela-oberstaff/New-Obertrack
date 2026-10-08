import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { tutorialService } from '../../../services/api'
import type { Tutorial } from '../../../types'

/**
 * Novedades publicadas para quien las lee, de la más reciente a la más
 * antigua, y cuáles todavía no ha abierto. Usa las mismas claves de caché que
 * useTutorials: al abrir una en Novedades se marca vista y el contador del
 * menú y del dashboard bajan solos.
 *
 * enabled = false para quien no tiene el módulo (CS, analista de IT): así no
 * se pide nada que el servidor vaya a negar.
 */
export function useNovedadesFeed(enabled = true) {
  const { data: tutorials, isLoading } = useQuery({
    queryKey: ['tutorials'],
    queryFn: async () => (await tutorialService.getAll()) || [],
    enabled,
  })
  const { data: viewed } = useQuery({
    queryKey: ['tutorial-views'],
    queryFn: async () => {
      try {
        return (await tutorialService.getMyViews()) || []
      } catch {
        return []
      }
    },
    enabled,
  })

  return useMemo(() => {
    const now = Date.now()
    const viewedIds = new Set<number>(viewed ?? [])
    const published = (tutorials ?? [])
      .filter(
        (t: Tutorial) =>
          t.is_active &&
          (!t.publish_at || new Date(t.publish_at).getTime() <= now) &&
          (!t.expires_at || new Date(t.expires_at).getTime() > now)
      )
      .sort((a, b) => publishedAt(b) - publishedAt(a))
    const unseen = published.filter((t) => !viewedIds.has(t.id))
    return { items: published, unseen, viewedIds, isLoading: enabled && isLoading }
  }, [tutorials, viewed, isLoading, enabled])
}

/** Cuándo salió: el anuncio si lo hubo, si no la creación. */
export function publishedAt(t: Tutorial): number {
  return new Date(t.announced_at || t.publish_at || t.created_at).getTime()
}
