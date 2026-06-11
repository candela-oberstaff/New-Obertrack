import React, { useRef, useState, useEffect } from 'react'
import { PaperclipIcon, MicIcon, SendIcon, SmileIcon } from './Icons'
import { EmojiGifPicker } from './EmojiGifPicker'
import { VoiceRecorderModal } from './VoiceRecorderModal'
import styles from '../../pages/SlackChat.module.css'

interface MessageInputProps {
  newMessage: string
  setNewMessage: (msg: string) => void
  onKeyDown: (e: React.KeyboardEvent) => void
  onFileUpload: (e: React.ChangeEvent<HTMLInputElement>) => void
  isUploading: boolean
  isRecording: boolean
  isPaused: boolean
  recordingStream: MediaStream | null
  recordedBlob: Blob | null
  startRecording: () => void
  stopRecording: () => void
  pauseRecording: () => void
  resumeRecording: () => void
  cancelRecording: () => void
  discardRecording: () => void
  sendRecording: () => void
  sendTypingIndicator: () => void
  onSend: () => void
  onSendGif: (url: string, title?: string) => void
}

export function MessageInput({
  newMessage,
  setNewMessage,
  onKeyDown,
  onFileUpload,
  isUploading,
  isRecording,
  isPaused,
  recordingStream,
  recordedBlob,
  startRecording,
  stopRecording,
  pauseRecording,
  resumeRecording,
  cancelRecording,
  discardRecording,
  sendRecording,
  sendTypingIndicator,
  onSend,
  onSendGif
}: MessageInputProps) {
  const fileInputRef = useRef<HTMLInputElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const wrapperRef = useRef<HTMLDivElement>(null)
  const [pickerTab, setPickerTab] = useState<'emoji' | 'gif' | null>(null)
  const [showVoiceModal, setShowVoiceModal] = useState(false)

  // Close the picker when clicking anywhere outside the composer.
  useEffect(() => {
    if (!pickerTab) return
    const onDocMouseDown = (e: MouseEvent) => {
      if (wrapperRef.current && !wrapperRef.current.contains(e.target as Node)) {
        setPickerTab(null)
      }
    }
    document.addEventListener('mousedown', onDocMouseDown)
    return () => document.removeEventListener('mousedown', onDocMouseDown)
  }, [pickerTab])

  const insertEmoji = (emoji: string) => {
    const ta = textareaRef.current
    const start = ta?.selectionStart ?? newMessage.length
    const end = ta?.selectionEnd ?? newMessage.length
    setNewMessage(newMessage.slice(0, start) + emoji + newMessage.slice(end))
    requestAnimationFrame(() => {
      if (!ta) return
      ta.focus()
      const pos = start + emoji.length
      ta.setSelectionRange(pos, pos)
    })
  }

  const togglePicker = (tab: 'emoji' | 'gif') => {
    setPickerTab(prev => (prev === tab ? null : tab))
  }

  return (
    <div className={styles['chat-input-area']}>
      <div className={styles['input-wrapper']} ref={wrapperRef}>
        {pickerTab && (
          <EmojiGifPicker
            tab={pickerTab}
            setTab={setPickerTab}
            onSelectEmoji={insertEmoji}
            onSelectGif={(url, title) => { onSendGif(url, title); setPickerTab(null) }}
          />
        )}

        <textarea
          ref={textareaRef}
          value={newMessage}
          onChange={(e) => {
            setNewMessage(e.target.value)
            sendTypingIndicator()
          }}
          onKeyDown={onKeyDown}
          placeholder="Escribe un mensaje..."
          rows={1}
        />

        <div className={styles['input-toolbar']}>
          <div className={styles['toolbar-left']}>
            <button className={styles['toolbar-btn']} onClick={() => fileInputRef.current?.click()} title="Adjuntar archivo">
              <PaperclipIcon />
            </button>
            <button
              className={`${styles['toolbar-btn']} ${isRecording ? styles['recording'] : ''}`}
              onClick={() => setShowVoiceModal(true)}
              title="Grabar nota de voz"
            >
              <MicIcon />
            </button>
            <button
              className={`${styles['toolbar-btn']} ${pickerTab === 'emoji' ? styles['active'] : ''}`}
              onClick={() => togglePicker('emoji')}
              title="Insertar emoji"
            >
              <SmileIcon />
            </button>
            <button
              className={`${styles['toolbar-btn']} ${styles['gif-btn']} ${pickerTab === 'gif' ? styles['active'] : ''}`}
              onClick={() => togglePicker('gif')}
              title="Enviar GIF"
            >
              GIF
            </button>
          </div>

          <button
            className={styles['send-btn']}
            onClick={onSend}
            disabled={!newMessage.trim() && !isUploading}
            title="Enviar mensaje"
          >
            <SendIcon />
          </button>
        </div>

        <input
          type="file"
          ref={fileInputRef}
          style={{ display: 'none' }}
          onChange={onFileUpload}
        />
        {isUploading && <div className={styles['upload-loader'] || 'upload-loader'}>Subiendo...</div>}
      </div>

      {showVoiceModal && (
        <VoiceRecorderModal
          isRecording={isRecording}
          isPaused={isPaused}
          stream={recordingStream}
          recordedBlob={recordedBlob}
          onStart={startRecording}
          onPause={pauseRecording}
          onResume={resumeRecording}
          onStop={stopRecording}
          onSend={() => { sendRecording(); setShowVoiceModal(false) }}
          onDiscard={discardRecording}
          onCancel={() => { cancelRecording(); setShowVoiceModal(false) }}
        />
      )}
    </div>
  )
}
