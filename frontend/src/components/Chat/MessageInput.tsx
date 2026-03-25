import React, { useRef } from 'react'

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
  sendTypingIndicator
}: MessageInputProps) {
  const fileInputRef = useRef<HTMLInputElement>(null)

  return (
    <div className="chat-input-area">
      <div className="input-toolbar">
        <button className="toolbar-btn" onClick={() => fileInputRef.current?.click()} title="Adjuntar archivo">
          📎
        </button>
        <button 
          className={`toolbar-btn ${isRecording ? 'recording' : ''}`} 
          onClick={isRecording ? stopRecording : startRecording}
          title={isRecording ? "Detener grabación" : "Grabar nota de voz"}
        >
          {isRecording ? '⏹' : '🎤'}
        </button>
        <input 
          type="file" 
          ref={fileInputRef} 
          style={{ display: 'none' }} 
          onChange={onFileUpload}
        />
      </div>
      
      <div className="input-wrapper">
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
        {isUploading && <div className="upload-loader">Subiendo...</div>}
      </div>
    </div>
  )
}
