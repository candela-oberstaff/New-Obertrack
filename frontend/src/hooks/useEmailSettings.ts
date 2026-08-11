import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { settingsService } from '../services/api'

const EMAIL_TYPES_KEY = ['settings', 'email-types'] as const

/**
 * Catálogo de correos del sistema con su interruptor y el envío de muestras
 * (Configuración → Correos).
 */
export function useEmailSettings() {
  const qc = useQueryClient()

  const typesQ = useQuery({
    queryKey: EMAIL_TYPES_KEY,
    queryFn: () => settingsService.getEmailTypes(),
  })

  const toggleMut = useMutation({
    mutationFn: ({ key, enabled }: { key: string; enabled: boolean }) =>
      settingsService.setEmailEnabled(key, enabled),
    onSuccess: () => qc.invalidateQueries({ queryKey: EMAIL_TYPES_KEY }),
  })

  const toggleAllMut = useMutation({
    mutationFn: (enabled: boolean) => settingsService.setAllEmailsEnabled(enabled),
    onSuccess: () => qc.invalidateQueries({ queryKey: EMAIL_TYPES_KEY }),
  })

  const testMut = useMutation({
    mutationFn: ({ key, email }: { key: string; email?: string }) =>
      settingsService.sendEmailTest(key, email),
  })

  return {
    types: typesQ.data ?? [],
    isLoading: typesQ.isLoading,
    toggle: toggleMut.mutateAsync,
    togglingKey: toggleMut.isPending ? toggleMut.variables?.key : undefined,
    toggleAll: toggleAllMut.mutateAsync,
    isTogglingAll: toggleAllMut.isPending,
    sendTest: testMut.mutateAsync,
    testingKey: testMut.isPending ? testMut.variables?.key : undefined,
  }
}
