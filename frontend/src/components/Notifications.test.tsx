import { render, screen, fireEvent, act } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import Notifications from './Notifications'
import { notificationService } from '../services/api'
import { showDesktopNotification } from '../lib/desktopNotifications'
import { playNotificationSound, setNotificationSoundEnabled } from '../lib/notificationSound'

vi.mock('../services/api', () => ({
  notificationService: {
    getAll: vi.fn(),
    getUnreadCount: vi.fn(),
    markAsRead: vi.fn(),
    markAllAsRead: vi.fn(),
  },
}))

const { FAKE_USER } = vi.hoisted(() => ({ FAKE_USER: { id: 1, name: 'Osvell' } }))

vi.mock('../context/AuthContext', () => ({
  useAuth: () => ({ user: FAKE_USER }),
}))

vi.mock('../lib/desktopNotifications', () => ({
  desktopPermission: () => 'granted',
  requestDesktopPermission: vi.fn(),
  showDesktopNotification: vi.fn(),
}))

vi.mock('../lib/notificationSound', () => ({
  isNotificationSoundEnabled: vi.fn(() => true),
  setNotificationSoundEnabled: vi.fn(),
  playNotificationSound: vi.fn(),
}))

class FakeWebSocket {
  static instances: FakeWebSocket[] = []
  static readonly CONNECTING = 0
  static readonly OPEN = 1
  static readonly CLOSING = 2
  static readonly CLOSED = 3

  url: string
  readyState = FakeWebSocket.CONNECTING
  onopen: ((e: Event) => void) | null = null
  onmessage: ((e: MessageEvent) => void) | null = null
  onclose: ((e: CloseEvent) => void) | null = null
  onerror: ((e: Event) => void) | null = null

  constructor(url: string) {
    this.url = url
    FakeWebSocket.instances.push(this)
  }

  /** Simula el handshake exitoso. */
  open() {
    this.readyState = FakeWebSocket.OPEN
    this.onopen?.(new Event('open'))
  }

  /** Simula una caída del socket (nginx, sleep, wifi). */
  drop() {
    this.readyState = FakeWebSocket.CLOSED
    this.onclose?.(new CloseEvent('close'))
  }

  emit(payload: unknown) {
    this.onmessage?.({ data: JSON.stringify(payload) } as MessageEvent)
  }

  close = vi.fn(() => {
    this.readyState = FakeWebSocket.CLOSED
  })
}

/**
 * Deja correr las promesas pendientes (los fetch mockeados) y los efectos.
 * No usamos waitFor de testing-library: solo detecta fake timers si existe el
 * global `jest`, que en vitest no existe, así que bajo vi.useFakeTimers() se
 * cuelga esperando un timer que nunca avanza.
 */
const flush = () => act(async () => { await Promise.resolve() })

/** Monta la campanita y espera el fetch inicial. La conexión se abre de forma
 *  síncrona dentro del efecto de montaje. */
const renderBell = async () => {
  const utils = render(<Notifications />, { wrapper: MemoryRouter })
  await flush()
  return utils
}

const firstSocket = (): FakeWebSocket => {
  expect(FakeWebSocket.instances.length).toBeGreaterThan(0)
  return FakeWebSocket.instances[0]
}

describe('Notifications', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    FakeWebSocket.instances = []
    vi.stubGlobal('WebSocket', FakeWebSocket)
    vi.mocked(notificationService.getAll).mockResolvedValue([])
    vi.mocked(notificationService.getUnreadCount).mockResolvedValue(0)
    vi.mocked(showDesktopNotification).mockClear()
    vi.mocked(playNotificationSound).mockClear()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
    vi.clearAllMocks()
  })

  // El bug de campo: el socket se caía (timeout de nginx a los 3600s, suspensión
  // del equipo) y nunca volvía, así que el usuario dejaba de recibir avisos el
  // resto de la sesión sin ningún error visible.
  it('reconnects after the socket drops', async () => {
    await renderBell()
    const socket = firstSocket()

    act(() => socket.open())
    act(() => socket.drop())

    expect(FakeWebSocket.instances).toHaveLength(1)

    // El backoff del primer intento es 1000ms + hasta 1000ms de jitter.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2100)
    })

    expect(FakeWebSocket.instances).toHaveLength(2)
  })

  it('resyncs the list when the socket comes back, so nothing missed while down is lost', async () => {
    await renderBell()
    const socket = firstSocket()

    act(() => socket.open())
    await flush()
    // El fetch inicial del montaje; la primera conexión no debe duplicarlo.
    expect(notificationService.getAll).toHaveBeenCalledTimes(1)

    act(() => socket.drop())
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2100)
    })

    act(() => FakeWebSocket.instances[1].open())
    await flush()
    expect(notificationService.getAll).toHaveBeenCalledTimes(2)
  })

  it('backs off exponentially instead of hammering a restarting backend', async () => {
    await renderBell()
    const socket = firstSocket()
    act(() => socket.open())

    // 1er intento: ~1-2s.
    act(() => socket.drop())
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2100)
    })
    expect(FakeWebSocket.instances).toHaveLength(2)

    // 2do intento: ~2-3s. A los 2.1s todavía no debería haber reconectado.
    act(() => FakeWebSocket.instances[1].drop())
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500)
    })
    expect(FakeWebSocket.instances).toHaveLength(2)

    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000)
    })
    expect(FakeWebSocket.instances).toHaveLength(3)
  })

  it('stops reconnecting once unmounted', async () => {
    const { unmount } = await renderBell()
    const socket = firstSocket()

    act(() => socket.open())
    act(() => socket.drop())

    unmount()

    await act(async () => {
      await vi.advanceTimersByTimeAsync(10000)
    })

    expect(FakeWebSocket.instances).toHaveLength(1)
  })

  it('refetches the list when the bell is opened', async () => {
    await renderBell()
    expect(notificationService.getAll).toHaveBeenCalledTimes(1)

    fireEvent.click(screen.getByTitle('Notificaciones'))
    await flush()

    // Antes la lista se traía solo al montar: el badge subía y el dropdown
    // mostraba una foto vieja.
    expect(notificationService.getAll).toHaveBeenCalledTimes(2)
  })

  it('raises an OS notification for a task assigned to you', async () => {
    await renderBell()
    const socket = firstSocket()
    act(() => socket.open())

    act(() =>
      socket.emit({
        type: 'task_assigned',
        data: { id: 42, title: 'Nueva tarea', message: 'Revisar informe', data: '{"link":"/tasks?task=42"}' },
      })
    )

    await flush()
    expect(showDesktopNotification).toHaveBeenCalledTimes(1)
    expect(vi.mocked(showDesktopNotification).mock.calls[0][0]).toMatchObject({
      title: 'Nueva tarea',
      body: 'Revisar informe',
    })
  })

  // task_updated le llega al empleador ante cualquier cambio de cualquier tarea:
  // como toast del SO sería spam y la gente terminaría bloqueando el permiso.
  it('does not raise an OS notification for high-volume update events', async () => {
    await renderBell()
    const socket = firstSocket()
    act(() => socket.open())

    act(() =>
      socket.emit({
        type: 'task_updated',
        data: { id: 43, title: 'Tarea actualizada', message: 'Cambió el estado' },
      })
    )

    await flush()
    expect(showDesktopNotification).not.toHaveBeenCalled()
    expect(playNotificationSound).not.toHaveBeenCalled()
  })

  it('plays a sound when a task is assigned to you', async () => {
    await renderBell()
    const socket = firstSocket()
    act(() => socket.open())

    act(() =>
      socket.emit({
        type: 'task_assigned',
        data: { id: 42, title: 'Nueva tarea', message: 'Revisar informe' },
      })
    )

    await flush()
    expect(playNotificationSound).toHaveBeenCalledTimes(1)
  })

  it('persists the mute preference when the sound is toggled off', async () => {
    await renderBell()
    fireEvent.click(screen.getByTitle('Notificaciones'))
    await flush()

    fireEvent.click(screen.getByLabelText('Silenciar notificaciones'))
    await flush()

    expect(setNotificationSoundEnabled).toHaveBeenCalledWith(false)
    // Apagarlo no debe reproducir la muestra; encenderlo sí.
    expect(playNotificationSound).not.toHaveBeenCalled()
    expect(screen.getByLabelText('Activar sonido')).toBeInTheDocument()
  })
})
