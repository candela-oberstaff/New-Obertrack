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

// Unique id for optimistic messages. Date.now() alone collides when two sends
// land in the same millisecond, which breaks tempId-based dedupe and React keys.
// Prefer crypto.randomUUID (available over https/wss); fall back just in case.
export const newTempId = (): string => {
  try {
    if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
      return `temp-${crypto.randomUUID()}`
    }
  } catch { /* fall through */ }
  return `temp-${Date.now()}-${Math.random().toString(36).slice(2)}`
}

// Nombre a mostrar de un DM: el destinatario; o ambos participantes ("A ↔ B")
// cuando el viewer no participa (supervisión de superadmin); o el nombre crudo.
export const dmContactName = (c: {
  recipient?: { name?: string }
  participants?: { name?: string }[]
  name?: string
}) =>
  c.recipient?.name ||
  (c.participants?.length ? c.participants.map(p => p.name).filter(Boolean).join(' ↔ ') : c.name || '')

// Los canales de soporte se crean como privados con el nombre "Soporte · <nombre> #<id>".
export const isSupportChannel = (c: { type?: string; name?: string }) =>
  c?.type === 'private' && !!c.name && /^Soporte · /.test(c.name)

// Etiqueta limpia para mostrar el canal de soporte: quita el prefijo y el sufijo "#id".
export const supportLabel = (name: string) => name.replace(/^Soporte · /, '').replace(/ #\d+$/, '')

// Etiqueta + color del estado de un ticket de soporte.
export const supportStatusMeta = (status?: string): { label: string; color: string; bg: string } => {
  switch (status) {
    case 'assigned': return { label: 'Asignado', color: '#6d28d9', bg: 'rgba(124,58,237,0.12)' }
    case 'resolved': return { label: 'Resuelto', color: '#15803d', bg: 'rgba(22,163,74,0.12)' }
    default: return { label: 'Sin asignar', color: '#b45309', bg: 'rgba(245,158,11,0.14)' }
  }
}

export const formatTime = (dateString: string) => {
  const date = new Date(dateString)
  const now = new Date()
  const time = date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })

  // Compare by calendar date (year/month/day) so the label doesn't flip just
  // because two timestamps fall within different 24h windows across midnight.
  const isSameDay = (a: Date, b: Date) =>
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()

  if (isSameDay(date, now)) return `Hoy ${time}`

  const yesterday = new Date(now)
  yesterday.setDate(now.getDate() - 1)
  if (isSameDay(date, yesterday)) return `Ayer ${time}`

  // Older dates: include the year only when it differs from the current year, so
  // "5 mar" (this year) stays compact while "5 mar 2024" disambiguates old dates.
  const opts: Intl.DateTimeFormatOptions = { day: 'numeric', month: 'short' }
  if (date.getFullYear() !== now.getFullYear()) opts.year = 'numeric'
  return date.toLocaleDateString('es-ES', opts)
}

const escapeRegExp = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

// Build the mention RegExp once per user list, so callers can memoize it and
// avoid rebuilding it on every render of every message. Returns null when there
// are no names (callers should then render the raw content).
export const buildMentionRegex = (allUsers: User[]): RegExp | null => {
  // Match against real user names (they contain spaces and accents, so \w+ won't do).
  // Longest names first so "Laura Méndez Jr" wins over "Laura Méndez".
  const names = allUsers
    .map(u => u.name)
    .filter(Boolean)
    .sort((a, b) => b.length - a.length)
  if (names.length === 0) return null
  return new RegExp(`@(${names.map(escapeRegExp).join('|')})`, 'gi')
}

// Highlight mentions using a precomputed RegExp (see buildMentionRegex), so the
// regex is built once per user list rather than on every render of every message.
export const highlightMentionsWithRegex = (
  content: string,
  mentionRegex: RegExp | null,
  currentUserName?: string,
): React.ReactNode => {
  if (!mentionRegex) return content
  // Shared regexes are stateful with the /g flag; reset before each scan.
  mentionRegex.lastIndex = 0
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

// Lowercase + strip accents so mention matching is accent/case-insensitive.
// Kept in sync with the backend normalizeMention (channel_messages.go).
const foldMention = (s: string) => s.toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '')

// True when the message text mentions the given user by name as a whole token,
// i.e. "@name" followed by end-of-string or a non-alphanumeric character, so
// "@Ana" does NOT match inside "@Anabel". Mirrors the backend predicate.
export const mentionsUser = (content: string, userName?: string): boolean => {
  if (!userName) return false
  const haystack = foldMention(content)
  const name = foldMention(userName.trim())
  if (!name) return false
  const needle = `@${name}`
  let from = 0
  for (;;) {
    const idx = haystack.indexOf(needle, from)
    if (idx < 0) return false
    const after = haystack[idx + needle.length]
    // End-of-string or a non-alphanumeric character is a valid token boundary.
    if (after === undefined || !/[a-z0-9]/i.test(after)) return true
    from = idx + needle.length
  }
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
