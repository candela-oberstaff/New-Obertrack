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

// Con este flag puesto, el usuario APAGÓ el push a propósito: la renovación
// automática al entrar no debe volver a suscribirlo.
const PUSH_DISABLED_KEY = 'push_disabled'

/**
 * usePushNotifications gestiona Web Push del navegador:
 *  - con permiso ya concedido, renueva la suscripción en silencio al entrar
 *    (salvo que el usuario la haya desactivado desde su Perfil);
 *  - expone `enable()` / `disable()` para el interruptor del Perfil;
 *  - `subscribed` refleja si ESTE navegador está suscrito;
 *  - `canPrompt` indica si vale la pena ofrecer el aviso (soportado + permiso
 *    aún sin decidir + no descartado antes).
 */
export function usePushNotifications(loggedIn: boolean) {
  const supported = typeof window !== 'undefined' && 'serviceWorker' in navigator && 'PushManager' in window && 'Notification' in window
  const [permission, setPermission] = useState<NotificationPermission>(supported ? Notification.permission : 'denied')
  const [dismissed, setDismissed] = useState(() => localStorage.getItem('push_prompt_dismissed') === '1')
  // null = todavía verificando; luego true/false según este navegador.
  const [subscribed, setSubscribed] = useState<boolean | null>(supported ? null : false)

  // Permiso ya concedido: re-suscribe en silencio (el endpoint del navegador
  // puede rotar, y el backend reasigna la fila al usuario actual) — salvo que
  // el usuario lo haya desactivado explícitamente desde su Perfil.
  useEffect(() => {
    if (!loggedIn || !supported) return
    if (Notification.permission !== 'granted' || localStorage.getItem(PUSH_DISABLED_KEY) === '1') {
      setSubscribed(false)
      return
    }
    ensureSubscribed()
      .then(ok => setSubscribed(ok))
      .catch(() => setSubscribed(false)) // best-effort: sin push se sigue con campanita/correo
  }, [loggedIn, supported])

  const enable = useCallback(async () => {
    if (!supported) return false
    const result = await Notification.requestPermission()
    setPermission(result)
    if (result !== 'granted') return false
    try {
      const ok = await ensureSubscribed()
      if (ok) {
        try { localStorage.removeItem(PUSH_DISABLED_KEY) } catch { /* sin storage */ }
      }
      setSubscribed(ok)
      return ok
    } catch {
      setSubscribed(false)
      return false
    }
  }, [supported])

  // disable da de baja la suscripción de ESTE navegador (backend + navegador)
  // y deja el flag para que el auto-suscribir del login no la reviva.
  const disable = useCallback(async () => {
    try { localStorage.setItem(PUSH_DISABLED_KEY, '1') } catch { /* sin storage */ }
    setSubscribed(false)
    try {
      const registration = await navigator.serviceWorker.getRegistration('/sw.js')
      const sub = await registration?.pushManager.getSubscription()
      if (sub) {
        try { await pushService.unsubscribe(sub.endpoint) } catch { /* la limpieza del backend es best-effort */ }
        await sub.unsubscribe()
      }
    } catch { /* sin registro que dar de baja */ }
  }, [])

  const dismiss = useCallback(() => {
    setDismissed(true)
    try { localStorage.setItem('push_prompt_dismissed', '1') } catch { /* sin storage: se re-ofrece la próxima */ }
  }, [])

  return {
    supported,
    permission,
    subscribed,
    canPrompt: supported && loggedIn && permission === 'default' && !dismissed,
    enable,
    disable,
    dismiss,
  }
}
