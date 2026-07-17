const ICON_URL = '/logos/Isotipo_Color.png'

export type DesktopPermission = NotificationPermission | 'unsupported'

function isSupported(): boolean {
  return typeof window !== 'undefined' && 'Notification' in window
}

export function desktopPermission(): DesktopPermission {
  if (!isSupported()) return 'unsupported'
  return Notification.permission
}

export async function requestDesktopPermission(): Promise<DesktopPermission> {
  if (!isSupported()) return 'unsupported'
  try {
    return await Notification.requestPermission()
  } catch {
    return Notification.permission
  }
}

interface ShowOptions {
  title: string
  body: string
  tag?: string
  onClick?: () => void
}

export function showDesktopNotification({ title, body, tag, onClick }: ShowOptions): void {
  if (!isSupported() || Notification.permission !== 'granted') return
  if (typeof document !== 'undefined' && document.hasFocus()) return

  try {
    const notification = new Notification(title, {
      body,
      icon: ICON_URL,
      tag,
    })
    notification.onclick = () => {
      window.focus()
      notification.close()
      onClick?.()
    }
  } catch {
    // Saludos a la persona que vea esto jejejeje - Osvell
  }
}
