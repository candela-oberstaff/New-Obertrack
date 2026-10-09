// El paquete real dibuja en un canvas; aquí solo importa cómo se crea y qué se le pide.
const { fire, create } = vi.hoisted(() => {
  const fire = vi.fn()
  return { fire, create: vi.fn(() => fire) }
})
vi.mock('canvas-confetti', () => ({ default: { create } }))

type ConfettiModule = typeof import('./confetti')
let celebrate: ConfettiModule['celebrate']
let DEFAULT_CELEBRATION: ConfettiModule['DEFAULT_CELEBRATION']

describe('celebrate', () => {
  beforeEach(async () => {
    fire.mockClear()
    create.mockClear()
    // El cargador guarda la instancia en memoria: módulo nuevo en cada prueba.
    vi.resetModules()
    ;({ celebrate, DEFAULT_CELEBRATION } = await import('./confetti'))
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('crea el lanzador SIN Worker (la CSP de producción bloquea los worker blob:)', async () => {
    await celebrate(DEFAULT_CELEBRATION, false)

    expect(create).toHaveBeenCalledTimes(1)
    expect(create).toHaveBeenCalledWith(undefined, expect.objectContaining({ useWorker: false }))
  })

  it('reutiliza la misma instancia en celebraciones sucesivas', async () => {
    await celebrate(DEFAULT_CELEBRATION, false)
    await celebrate(DEFAULT_CELEBRATION, false)

    expect(create).toHaveBeenCalledTimes(1)
    expect(fire).toHaveBeenCalledTimes(2)
  })

  it('lanza una ráfaga central con los ajustes y los colores de la marca', async () => {
    await celebrate(DEFAULT_CELEBRATION, false)

    expect(fire).toHaveBeenCalledTimes(1)
    expect(fire).toHaveBeenCalledWith(
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
    vi.useFakeTimers({ toFake: ['Date', 'requestAnimationFrame', 'cancelAnimationFrame'] })

    await celebrate(DEFAULT_CELEBRATION, true)
    await vi.advanceTimersByTimeAsync(2000)

    const calls = fire.mock.calls.map(([options]) => options as { angle?: number })
    // Ráfaga central + varios pares de cañones (izquierdo y derecho) durante ~1,8 s.
    expect(calls.length).toBeGreaterThanOrEqual(3)
    expect(calls.some((o) => o?.angle === 60)).toBe(true)
    expect(calls.some((o) => o?.angle === 120)).toBe(true)
  })

  it('si el paquete no carga, no hace nada ni rompe', async () => {
    create.mockImplementationOnce(() => {
      throw new Error('sin red')
    })

    await expect(celebrate(DEFAULT_CELEBRATION, false)).resolves.toBeUndefined()
    expect(fire).not.toHaveBeenCalled()
  })
})
