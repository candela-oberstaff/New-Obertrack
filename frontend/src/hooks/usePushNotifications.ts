import { useCallback, useEffect, useState } from 'react'
import { pushService } from '../services/push.service'

// urlBase64ToUint8Array convierte la clave pública VAPID (base64 URL-safe) al
// formato que exige PushManager.subscribe.
function urlBase64ToUint8Array(base64String: string): Uint8Array {
  const padding = '='.repeat((4 - (base64String.length % 4)) % 4)
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/')
  const raw = window.atob(base64)
  const output = new Uint8Array(raw.length)
  for (let i = 0; i < raw.length; i++) output[i] = raw.charCodeAt(i)
  return output
}

async function ensureSubscribed(): Promise<boolean> {
  const registration = await navigator.serviceWorker.register('/sw.js')
  const publicKey = await pushService.getPublicKey()
  if (!publicKey) return false

  let subscription = await registration.pushManager.getSubscription()
  if (!subscription) {
    subscription = await registration.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: urlBase64ToUint8Array(publicKey) as BufferSource,
    })
  }
  // Reenvía la suscripción SIEMPRE (idempotente en el backend): así también se
  // reasigna el endpoint cuando otro usuario inicia sesión en este navegador.
  await pushService.subscribe(subscription.toJSON())
  return true
}

/**
 * usePushNotifications gestiona Web Push del navegador:
 *  - con permiso ya concedido, renueva la suscripción en silencio al entrar;
 *  - expone `enable()` para pedir el permiso desde un gesto del usuario;
 *  - `canPrompt` indica si vale la pena ofrecer el aviso (soportado + permiso
 *    aún sin decidir + no descartado antes).
 */
export function usePushNotifications(loggedIn: boolean) {
  const supported = typeof window !== 'undefined' && 'serviceWorker' in navigator && 'PushManager' in window && 'Notification' in window
  const [permission, setPermission] = useState<NotificationPermission>(supported ? Notification.permission : 'denied')
  const [dismissed, setDismissed] = useState(() => localStorage.getItem('push_prompt_dismissed') === '1')

  // Permiso ya concedido: re-suscribe en silencio (el endpoint del navegador
  // puede rotar, y el backend reasigna la fila al usuario actual).
  useEffect(() => {
    if (!loggedIn || !supported || Notification.permission !== 'granted') return
    ensureSubscribed().catch(() => { /* best-effort: sin push se sigue con campanita/correo */ })
  }, [loggedIn, supported])

  const enable = useCallback(async () => {
    if (!supported) return false
    const result = await Notification.requestPermission()
    setPermission(result)
    if (result !== 'granted') return false
    try {
      return await ensureSubscribed()
    } catch {
      return false
    }
  }, [supported])

  const dismiss = useCallback(() => {
    setDismissed(true)
    try { localStorage.setItem('push_prompt_dismissed', '1') } catch { /* sin storage: se re-ofrece la próxima */ }
  }, [])

  return {
    supported,
    permission,
    canPrompt: supported && loggedIn && permission === 'default' && !dismissed,
    enable,
    dismiss,
  }
}
