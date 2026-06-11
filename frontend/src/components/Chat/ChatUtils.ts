import React from 'react'
import type { User } from '../../types'
import styles from '../../pages/SlackChat.module.css'

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

const escapeRegExp = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

export const highlightMentions = (content: string, allUsers: User[], currentUserName?: string): React.ReactNode => {
  // Match against real user names (they contain spaces and accents, so \w+ won't do).
  // Longest names first so "Laura Méndez Jr" wins over "Laura Méndez".
  const names = allUsers
    .map(u => u.name)
    .filter(Boolean)
    .sort((a, b) => b.length - a.length)
  if (names.length === 0) return content

  const mentionRegex = new RegExp(`@(${names.map(escapeRegExp).join('|')})`, 'gi')
  const parts: React.ReactNode[] = []
  let lastIndex = 0
  let match

  while ((match = mentionRegex.exec(content)) !== null) {
    if (match.index > lastIndex) {
      parts.push(content.slice(lastIndex, match.index))
    }
    const isMe = !!currentUserName && match[1].toLowerCase() === currentUserName.toLowerCase()
    parts.push(React.createElement(
      'span',
      { key: match.index, className: `${styles['mention-highlight']} ${isMe ? styles['mention-me'] : ''}` },
      match[0]
    ))
    lastIndex = mentionRegex.lastIndex
  }

  if (lastIndex < content.length) {
    parts.push(content.slice(lastIndex))
  }

  return parts.length > 0 ? parts : content
}

// True when the message text mentions the given user by name.
export const mentionsUser = (content: string, userName?: string): boolean => {
  if (!userName) return false
  return content.toLowerCase().includes(`@${userName.toLowerCase()}`)
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
  'var(--primary)', // pink
  '#ef4444', // red
  '#10b981', // emerald
  '#f59e0b', // amber
  '#8b5cf6', // violet
  '#ec4899', // pink
  '#06b6d4', // cyan
  '#f97316', // orange
  '#84cc16', // lime
  '#db2777'  // dark pink instead of indigo
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
