const createFakeOscillator = () => ({
  type: '',
  frequency: { value: 0 },
  connect: vi.fn(),
  start: vi.fn(),
  stop: vi.fn(),
})

const createFakeGain = () => ({
  gain: {
    setValueAtTime: vi.fn(),
    linearRampToValueAtTime: vi.fn(),
    exponentialRampToValueAtTime: vi.fn(),
  },
  connect: vi.fn(),
})

class FakeAudioContext {
  static instances: FakeAudioContext[] = []
  state: AudioContextState = 'running'
  currentTime = 0
  destination = {}
  resume = vi.fn(() => Promise.resolve())
  oscillators: ReturnType<typeof createFakeOscillator>[] = []
  createOscillator = vi.fn(() => {
    const osc = createFakeOscillator()
    this.oscillators.push(osc)
    return osc
  })
  createGain = vi.fn(createFakeGain)

  constructor() {
    FakeAudioContext.instances.push(this)
  }
}

const loadModule = () => import('./notificationSound')

describe('notificationSound', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.resetModules()
    localStorage.clear()
    FakeAudioContext.instances = []
    vi.stubGlobal('AudioContext', FakeAudioContext)
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('plays a two-tone chime', async () => {
    const { playNotificationSound } = await loadModule()

    playNotificationSound()

    const ctx = FakeAudioContext.instances[0]
    expect(ctx.oscillators).toHaveLength(2)
    expect(ctx.oscillators[1].frequency.value).toBeGreaterThan(ctx.oscillators[0].frequency.value)
    expect(ctx.oscillators[0].start).toHaveBeenCalled()
    expect(ctx.oscillators[0].stop).toHaveBeenCalled()
  })

  it('is on by default but stays silent once muted', async () => {
    const { playNotificationSound, setNotificationSoundEnabled, isNotificationSoundEnabled } = await loadModule()

    expect(isNotificationSoundEnabled()).toBe(true)

    setNotificationSoundEnabled(false)
    expect(isNotificationSoundEnabled()).toBe(false)

    playNotificationSound()
    expect(FakeAudioContext.instances).toHaveLength(0)
  })

  it('remembers the mute preference across reloads', async () => {
    const first = await loadModule()
    first.setNotificationSoundEnabled(false)

    vi.resetModules()
    const second = await loadModule()

    expect(second.isNotificationSoundEnabled()).toBe(false)
  })

  it('collapses a burst of notifications into a single chime', async () => {
    const { playNotificationSound } = await loadModule()

    playNotificationSound()
    playNotificationSound()
    playNotificationSound()

    expect(FakeAudioContext.instances[0].oscillators).toHaveLength(2)
  })

  it('plays again once the burst window has passed', async () => {
    const { playNotificationSound } = await loadModule()

    playNotificationSound()
    vi.advanceTimersByTime(2000)
    playNotificationSound()

    expect(FakeAudioContext.instances[0].oscillators).toHaveLength(4)
  })

  it('resumes a context suspended by the autoplay policy', async () => {
    const { playNotificationSound } = await loadModule()

    playNotificationSound()
    const ctx = FakeAudioContext.instances[0]
    ctx.state = 'suspended'

    vi.advanceTimersByTime(2000)
    playNotificationSound()

    expect(ctx.resume).toHaveBeenCalled()
  })

  it('degrades quietly when the browser has no Web Audio support', async () => {
    vi.stubGlobal('AudioContext', undefined)
    vi.stubGlobal('webkitAudioContext', undefined)
    const { playNotificationSound } = await loadModule()

    expect(() => playNotificationSound()).not.toThrow()
  })
})
