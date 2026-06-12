import { useEffect, useRef, useState } from 'react'
import styles from '../../pages/SlackChat.module.css'

const BAR_COUNT = 48

const formatClock = (s: number) => {
  if (!isFinite(s)) return '00:00'
  const mm = String(Math.floor(s / 60)).padStart(2, '0')
  const ss = String(Math.floor(s % 60)).padStart(2, '0')
  return `${mm}:${ss}`
}

interface AudioPlayerProps {
  src: string
  /** Duración conocida en segundos (ej: el cronómetro de la grabación).
   * Respaldo para blobs de MediaRecorder sin metadata de duración. */
  durationHint?: number
}

// Custom audio player with a real waveform (decoded from the audio itself).
// Decoding also gives a reliable duration — MediaRecorder webm blobs often
// report Infinity through the native <audio> element.
export function AudioPlayer({ src, durationHint }: AudioPlayerProps) {
  const audioRef = useRef<HTMLAudioElement | null>(null)
  const barsRef = useRef<HTMLDivElement>(null)
  const [playing, setPlaying] = useState(false)
  const [duration, setDuration] = useState(0)
  const [current, setCurrent] = useState(0)
  const [peaks, setPeaks] = useState<number[]>([])

  useEffect(() => {
    let active = true
    const audio = new Audio(src)
    audioRef.current = audio

    const onTime = () => setCurrent(audio.currentTime)
    const onEnded = () => { setPlaying(false); setCurrent(0) }
    // Los webm de MediaRecorder reportan Infinity: saltar a un tiempo enorme
    // fuerza al navegador a calcular la duración real (truco estándar).
    const onDurationChange = () => {
      if (active && isFinite(audio.duration) && audio.duration > 0) {
        setDuration(audio.duration)
      }
    }
    const onLoadedMetadata = () => {
      if (audio.duration === Infinity) {
        const restore = () => {
          if (isFinite(audio.duration) && audio.duration > 0) {
            audio.currentTime = 0
            if (active) { setCurrent(0); setDuration(audio.duration) }
            audio.removeEventListener('durationchange', restore)
          }
        }
        audio.addEventListener('durationchange', restore)
        audio.currentTime = 1e7
      } else {
        onDurationChange()
      }
    }
    audio.addEventListener('timeupdate', onTime)
    audio.addEventListener('ended', onEnded)
    audio.addEventListener('loadedmetadata', onLoadedMetadata)
    audio.addEventListener('durationchange', onDurationChange)

    fetch(src)
      .then(r => r.arrayBuffer())
      .then(buf => {
        const ctx = new AudioContext()
        return ctx.decodeAudioData(buf).then(decoded => {
          if (!active) { ctx.close().catch(() => {}); return }
          setDuration(decoded.duration)
          const data = decoded.getChannelData(0)
          const block = Math.max(1, Math.floor(data.length / BAR_COUNT))
          const raw: number[] = []
          for (let i = 0; i < BAR_COUNT; i++) {
            let sum = 0
            for (let j = 0; j < block; j++) sum += Math.abs(data[i * block + j] || 0)
            raw.push(sum / block)
          }
          const max = Math.max(...raw, 0.0001)
          setPeaks(raw.map(v => Math.max(0.12, v / max)))
          ctx.close().catch(() => {})
        })
      })
      .catch(() => {
        // Decoding failed (unsupported codec): keep a flat waveform, playback still works.
        setPeaks(Array(BAR_COUNT).fill(0.3))
      })

    return () => {
      active = false
      audio.pause()
      audio.removeEventListener('timeupdate', onTime)
      audio.removeEventListener('ended', onEnded)
      audio.removeEventListener('loadedmetadata', onLoadedMetadata)
      audio.removeEventListener('durationchange', onDurationChange)
      audioRef.current = null
    }
  }, [src])

  // Total a mostrar: metadata/decodificación si existen, si no la pista externa.
  const total = duration || durationHint || 0

  const toggle = () => {
    const audio = audioRef.current
    if (!audio) return
    if (playing) {
      audio.pause()
      setPlaying(false)
    } else {
      audio.play().then(() => setPlaying(true)).catch(err => {
        console.error('Error reproduciendo la nota de voz:', err)
        setPlaying(false)
      })
    }
  }

  const seek = (e: React.MouseEvent) => {
    const audio = audioRef.current
    const bars = barsRef.current
    if (!audio || !bars || !total) return
    const rect = bars.getBoundingClientRect()
    const fraction = Math.min(1, Math.max(0, (e.clientX - rect.left) / rect.width))
    audio.currentTime = fraction * total
    setCurrent(audio.currentTime)
  }

  const progress = total ? Math.min(1, current / total) : 0

  return (
    <div className={styles['audio-player']}>
      <button className={styles['audio-play-btn']} onClick={toggle} title={playing ? 'Pausar' : 'Reproducir'}>
        {playing ? '❚❚' : '▶'}
      </button>
      <div className={styles['audio-bars']} ref={barsRef} onClick={seek}>
        {(peaks.length ? peaks : Array(BAR_COUNT).fill(0.25)).map((peak, i) => (
          <span
            key={i}
            className={`${styles['audio-bar']} ${i / BAR_COUNT <= progress ? styles['played'] : ''}`}
            style={{ height: `${Math.round(peak * 100)}%` }}
          />
        ))}
      </div>
      <span className={styles['audio-time']}>
        {formatClock(current)} / {formatClock(total)}
      </span>
    </div>
  )
}
