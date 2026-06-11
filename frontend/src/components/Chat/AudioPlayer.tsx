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
}

// Custom audio player with a real waveform (decoded from the audio itself).
// Decoding also gives a reliable duration — MediaRecorder webm blobs often
// report Infinity through the native <audio> element.
export function AudioPlayer({ src }: AudioPlayerProps) {
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
    audio.addEventListener('timeupdate', onTime)
    audio.addEventListener('ended', onEnded)

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
      audioRef.current = null
    }
  }, [src])

  const toggle = () => {
    const audio = audioRef.current
    if (!audio) return
    if (playing) { audio.pause(); setPlaying(false) }
    else { audio.play(); setPlaying(true) }
  }

  const seek = (e: React.MouseEvent) => {
    const audio = audioRef.current
    const bars = barsRef.current
    if (!audio || !bars || !duration) return
    const rect = bars.getBoundingClientRect()
    const fraction = Math.min(1, Math.max(0, (e.clientX - rect.left) / rect.width))
    audio.currentTime = fraction * duration
    setCurrent(audio.currentTime)
  }

  const progress = duration ? current / duration : 0

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
        {formatClock(current)} / {formatClock(duration)}
      </span>
    </div>
  )
}
