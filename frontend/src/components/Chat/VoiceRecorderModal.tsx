import { useEffect, useMemo, useRef, useState } from 'react'
import { Modal, Button } from '../ui'
import { AudioPlayer } from './AudioPlayer'
import styles from '../../pages/SlackChat.module.css'

interface VoiceRecorderModalProps {
  isRecording: boolean
  isPaused: boolean
  stream: MediaStream | null
  recordedBlob: Blob | null
  onStart: () => void
  onPause: () => void
  onResume: () => void
  onStop: () => void
  onSend: () => void
  onDiscard: () => void
  onCancel: () => void
}

export function VoiceRecorderModal({
  isRecording,
  isPaused,
  stream,
  recordedBlob,
  onStart,
  onPause,
  onResume,
  onStop,
  onSend,
  onDiscard,
  onCancel
}: VoiceRecorderModalProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const [seconds, setSeconds] = useState(0)

  const uiState: 'idle' | 'recording' | 'paused' | 'preview' =
    recordedBlob ? 'preview' : isRecording ? (isPaused ? 'paused' : 'recording') : 'idle'

  // The timer only advances while actively recording; the total survives into preview.
  useEffect(() => {
    if (!isRecording || isPaused) return
    const interval = setInterval(() => setSeconds(s => s + 1), 1000)
    return () => clearInterval(interval)
  }, [isRecording, isPaused])

  const previewUrl = useMemo(
    () => (recordedBlob ? URL.createObjectURL(recordedBlob) : null),
    [recordedBlob]
  )
  useEffect(() => {
    return () => { if (previewUrl) URL.revokeObjectURL(previewUrl) }
  }, [previewUrl])

  // Live waveform: frequency bars from the mic stream; frozen while paused.
  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    if (!stream || isPaused) {
      if (!stream) {
        ctx.clearRect(0, 0, canvas.width, canvas.height)
        ctx.fillStyle = '#e2e8f0'
        ctx.fillRect(0, canvas.height / 2 - 1, canvas.width, 2)
      }
      return
    }

    const audioCtx = new AudioContext()
    const source = audioCtx.createMediaStreamSource(stream)
    const analyser = audioCtx.createAnalyser()
    analyser.fftSize = 128
    source.connect(analyser)
    const data = new Uint8Array(analyser.frequencyBinCount)
    const barColor = getComputedStyle(canvas).getPropertyValue('--primary').trim() || '#9333ea'

    let raf = 0
    const draw = () => {
      raf = requestAnimationFrame(draw)
      analyser.getByteFrequencyData(data)
      ctx.clearRect(0, 0, canvas.width, canvas.height)
      const barWidth = canvas.width / data.length
      for (let i = 0; i < data.length; i++) {
        const barHeight = Math.max(3, (data[i] / 255) * canvas.height)
        ctx.fillStyle = barColor
        ctx.fillRect(i * barWidth + 1, (canvas.height - barHeight) / 2, Math.max(1, barWidth - 2), barHeight)
      }
    }
    draw()

    return () => {
      cancelAnimationFrame(raf)
      source.disconnect()
      audioCtx.close().catch(() => {})
    }
  }, [stream, isPaused])

  const handleStart = () => { setSeconds(0); onStart() }
  const handleRerecord = () => { setSeconds(0); onDiscard(); onStart() }

  const mm = String(Math.floor(seconds / 60)).padStart(2, '0')
  const ss = String(seconds % 60).padStart(2, '0')

  const hints: Record<typeof uiState, string> = {
    idle: 'Pulsa "Grabar" para comenzar la nota de voz.',
    recording: 'Grabando... puedes pausar o detener para escucharla.',
    paused: 'Grabación en pausa. Reanuda o detén para escucharla.',
    preview: 'Escucha tu nota de voz antes de enviarla.',
  }

  return (
    <Modal
      isOpen
      onClose={onCancel}
      title="🎙️ Nota de voz"
      size="sm"
      footer={
        <>
          {uiState === 'idle' && (
            <>
              <Button variant="secondary" onClick={onCancel}>Cancelar</Button>
              <Button onClick={handleStart}>● Grabar</Button>
            </>
          )}
          {(uiState === 'recording' || uiState === 'paused') && (
            <>
              <Button variant="secondary" onClick={onCancel}>Cancelar</Button>
              {uiState === 'recording'
                ? <Button variant="secondary" onClick={onPause}>⏸ Pausar</Button>
                : <Button variant="secondary" onClick={onResume}>▶ Reanudar</Button>}
              <Button onClick={onStop}>■ Detener</Button>
            </>
          )}
          {uiState === 'preview' && (
            <>
              <Button variant="secondary" onClick={onCancel}>Descartar</Button>
              <Button variant="secondary" onClick={handleRerecord}>↺ Regrabar</Button>
              <Button onClick={onSend}>Enviar</Button>
            </>
          )}
        </>
      }
    >
      <div className={styles['voice-recorder']}>
        {uiState === 'preview' ? (
          <div className={styles['voice-preview']}>
            {previewUrl && <AudioPlayer src={previewUrl} />}
          </div>
        ) : (
          <>
            <div className={styles['voice-wave-wrapper']}>
              <canvas
                ref={canvasRef}
                width={400}
                height={90}
                className={`${styles['voice-wave']} ${uiState === 'paused' ? styles['paused'] : ''}`}
              />
              {uiState === 'paused' && <span className={styles['voice-paused-badge']}>⏸ En pausa</span>}
            </div>
            <div className={styles['voice-timer']}>
              {uiState === 'recording' && <span className={styles['rec-dot']} />}
              {uiState === 'paused' && <span className={`${styles['rec-dot']} ${styles['paused']}`} />}
              {mm}:{ss}
            </div>
          </>
        )}
        <p className={styles['voice-hint']}>{hints[uiState]}</p>
      </div>
    </Modal>
  )
}
