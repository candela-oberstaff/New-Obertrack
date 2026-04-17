import React from 'react'
import type { User } from '../../types'

// Singleton for AudioContext to avoid multiple instances causing issues
let globalAudioContext: AudioContext | null = null

export const getAudioContext = () => {
  if (!globalAudioContext) {
    globalAudioContext = new (window.AudioContext || (window as any).webkitAudioContext)()
  }
  return globalAudioContext
}

export const formatTime = (dateString: string) => {
  const date = new Date(dateString)
  const now = new Date()
  const diff = now.getTime() - date.getTime()
  if (diff < 3600000) return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  if (diff < 86400000) return `Hoy ${date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}`
  return date.toLocaleDateString('es-ES', { day: 'numeric', month: 'short' })
}

export const highlightMentions = (content: string, allUsers: User[]): React.ReactNode => {
  const mentionRegex = /@(\w+)/g
  const parts: React.ReactNode[] = []
  let lastIndex = 0
  let match

  while ((match = mentionRegex.exec(content)) !== null) {
    if (match.index > lastIndex) {
      parts.push(content.slice(lastIndex, match.index))
    }
    const mentionedName = match[1].toLowerCase()
    const mentionedUser = allUsers.find(u => u.name.toLowerCase().replace(/\s+/g, '') === mentionedName)
    if (mentionedUser) {
      parts.push(React.createElement('span', { key: match.index, className: 'mention-highlight' }, `@${match[1]}`))
    } else {
      parts.push(`@${match[1]}`)
    }
    lastIndex = mentionRegex.lastIndex
  }

  if (lastIndex < content.length) {
    parts.push(content.slice(lastIndex))
  }

  return parts.length > 0 ? parts : content
}

export const playNotificationSound = () => {
  try {
    const audioContext = getAudioContext()
    if (audioContext.state === 'suspended') {
      audioContext.resume()
    }

    const oscillator = audioContext.createOscillator()
    const gainNode = audioContext.createGain()
    oscillator.connect(gainNode)
    gainNode.connect(audioContext.destination)
    oscillator.frequency.value = 800
    oscillator.type = 'sine'
    gainNode.gain.setValueAtTime(0.1, audioContext.currentTime)
    gainNode.gain.exponentialRampToValueAtTime(0.01, audioContext.currentTime + 0.3)
    oscillator.start(audioContext.currentTime)
    oscillator.stop(audioContext.currentTime + 0.3)
  } catch (e) {
    console.error('Error playing notification sound:', e)
  }
}

export const USER_COLORS = [
  '#3b82f6', // blue
  '#ef4444', // red
  '#10b981', // emerald
  '#f59e0b', // amber
  '#8b5cf6', // violet
  '#ec4899', // pink
  '#06b6d4', // cyan
  '#f97316', // orange
  '#84cc16', // lime
  '#6366f1'  // indigo
]

export const getUserColor = (name: string): string => {
  if (!name) return USER_COLORS[0]
  let hash = 0
  for (let i = 0; i < name.length; i++) {
    hash = name.charCodeAt(i) + ((hash << 5) - hash)
  }
  const index = Math.abs(hash) % USER_COLORS.length
  return USER_COLORS[index]
}
