import React, { useRef } from 'react'
import { PaperclipIcon, MicIcon, StopIcon, SendIcon } from './Icons'
import styles from '../../pages/SlackChat.module.css'

interface MessageInputProps {
  newMessage: string
  setNewMessage: (msg: string) => void
  onKeyDown: (e: React.KeyboardEvent) => void
  onFileUpload: (e: React.ChangeEvent<HTMLInputElement>) => void
  isUploading: boolean
  isRecording: boolean
  startRecording: () => void
  stopRecording: () => void
  sendTypingIndicator: () => void
  onSend: () => void
}

export function MessageInput({
  newMessage,
  setNewMessage,
  onKeyDown,
  onFileUpload,
  isUploading,
  isRecording,
  startRecording,
  stopRecording,
  sendTypingIndicator,
  onSend
}: MessageInputProps) {
  const fileInputRef = useRef<HTMLInputElement>(null)

  return (
    <div className={styles['chat-input-area']}>
      <div className={styles['input-wrapper']}>
        <textarea
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
              onClick={isRecording ? stopRecording : startRecording}
              title={isRecording ? "Detener grabación" : "Grabar nota de voz"}
            >
              {isRecording ? <StopIcon /> : <MicIcon />}
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
    </div>
  )
}
