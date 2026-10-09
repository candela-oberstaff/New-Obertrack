import confetti from 'canvas-confetti'
import { celebrate, DEFAULT_CELEBRATION } from './confetti'

// El paquete real dibuja en un canvas; aquí solo importa qué se le pide.
vi.mock('canvas-confetti', () => ({ default: vi.fn() }))

describe('celebrate', () => {
  beforeEach(() => {
    vi.mocked(confetti).mockClear()
  })

  it('lanza una ráfaga central con los ajustes y los colores de la marca', async () => {
    await celebrate(DEFAULT_CELEBRATION, false)

    expect(confetti).toHaveBeenCalledTimes(1)
    expect(confetti).toHaveBeenCalledWith(
      expect.objectContaining({
        particleCount: DEFAULT_CELEBRATION.particleCount,
        colors: DEFAULT_CELEBRATION.colors,
        origin: { x: 0.5, y: DEFAULT_CELEBRATION.originY },
        // Respeta «reducir movimiento» del sistema.
        disableForReducedMotion: true,
      })
    )
  })

  it('al completar el programa suma los cañones laterales', async () => {
    await celebrate(DEFAULT_CELEBRATION, true)

    const calls = vi.mocked(confetti).mock.calls.map(([options]) => options)
    // Ráfaga central + al menos un par de cañones (izquierdo y derecho).
    expect(calls.length).toBeGreaterThanOrEqual(3)
    expect(calls.some((o) => o?.angle === 60)).toBe(true)
    expect(calls.some((o) => o?.angle === 120)).toBe(true)
  })
})
