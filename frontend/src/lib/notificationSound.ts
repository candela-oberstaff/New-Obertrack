const SOUND_PREF_KEY = 'notifications_sound'

const FIRST_TONE_HZ = 880
const SECOND_TONE_HZ = 1174.7
const TONE_DURATION = 0.12
const PEAK_GAIN = 0.09
const MIN_INTERVAL_MS = 1500

let audioContext: AudioContext | null = null
let unlockListenerAttached = false
let lastPlayedAt = 0

type AudioContextCtor = typeof AudioContext

function getAudioContextCtor(): AudioContextCtor | null {
  if (typeof window === 'undefined') return null
  return window.AudioContext ?? (window as unknown as { webkitAudioContext?: AudioContextCtor }).webkitAudioContext ?? null
}

function attachUnlockListener(ctx: AudioContext): void {
  if (unlockListenerAttached || typeof document === 'undefined') return
  unlockListenerAttached = true
  const unlock = () => {
    void ctx.resume().catch(() => {})
    document.removeEventListener('pointerdown', unlock)
    document.removeEventListener('keydown', unlock)
  }
  document.addEventListener('pointerdown', unlock)
  document.addEventListener('keydown', unlock)
}

function getContext(): AudioContext | null {
  const Ctor = getAudioContextCtor()
  if (!Ctor) return null
  if (!audioContext) {
    try {
      audioContext = new Ctor()
    } catch {
      return null
    }
    attachUnlockListener(audioContext)
  }
  return audioContext
}

function playTone(ctx: AudioContext, frequency: number, startAt: number): void {
  const oscillator = ctx.createOscillator()
  const gain = ctx.createGain()

  oscillator.type = 'sine'
  oscillator.frequency.value = frequency

  // Un arranque/corte seco produce un click audible, de ahí las rampas.
  // exponentialRamp no admite 0 como destino.
  gain.gain.setValueAtTime(0, startAt)
  gain.gain.linearRampToValueAtTime(PEAK_GAIN, startAt + 0.01)
  gain.gain.exponentialRampToValueAtTime(0.0001, startAt + TONE_DURATION)

  oscillator.connect(gain)
  gain.connect(ctx.destination)
  oscillator.start(startAt)
  oscillator.stop(startAt + TONE_DURATION)
}

export function isNotificationSoundEnabled(): boolean {
  if (typeof localStorage === 'undefined') return true
  return localStorage.getItem(SOUND_PREF_KEY) !== 'off'
}

export function setNotificationSoundEnabled(enabled: boolean): void {
  if (typeof localStorage === 'undefined') return
  localStorage.setItem(SOUND_PREF_KEY, enabled ? 'on' : 'off')
}

export function playNotificationSound(): void {
  if (!isNotificationSoundEnabled()) return

  const now = Date.now()
  if (now - lastPlayedAt < MIN_INTERVAL_MS) return
  lastPlayedAt = now

  const ctx = getContext()
  if (!ctx) return

  if (ctx.state === 'suspended') void ctx.resume().catch(() => {})

  try {
    const startAt = ctx.currentTime
    playTone(ctx, FIRST_TONE_HZ, startAt)
    playTone(ctx, SECOND_TONE_HZ, startAt + TONE_DURATION * 0.8)
  } catch {
    // Un contexto cerrado no debe romper el manejo de la notificación.
  }
}
