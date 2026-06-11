import type { ComponentType } from 'react'
import {
  Activity, Clock, CheckCircle2, AlertTriangle, XCircle, CheckSquare, UserPlus, LogIn,
} from 'lucide-react'
import styles from './ActivityFeed.module.css'

interface ActivityItem {
  id?: number
  type?: string
  description?: string
  details?: string
  user?: string
  company?: string
  created_at?: string
  timestamp?: string
}

type Visual = { icon: ComponentType<{ size?: number }>; color: string; bg: string }

const GREEN = '#10b981'
const AMBER = '#f59e0b'
const RED = '#ef4444'
const VIOLET = '#8b5cf6'
const BLUE = '#3b82f6'
const GRAY = '#64748b'

function tint(hex: string) {
  return hex + '1f' // ~12% alpha
}

// Maps an activity to an icon + colour. Works today with work-hour events and is
// ready for richer event types (tasks, approvals, auth…) when the feed grows.
function visualFor(a: ActivityItem): Visual {
  const text = (a.description || a.details || '').toLowerCase()
  const type = (a.type || '').toLowerCase()
  if (text.includes('ausencia')) return { icon: AlertTriangle, color: AMBER, bg: tint(AMBER) }
  if (text.includes('jornada') || type === 'work_hour') return { icon: Clock, color: GREEN, bg: tint(GREEN) }
  if (text.includes('aprob')) return { icon: CheckCircle2, color: GREEN, bg: tint(GREEN) }
  if (text.includes('rechaz')) return { icon: XCircle, color: RED, bg: tint(RED) }
  if (text.includes('tarea') || text.includes('tablero')) return { icon: CheckSquare, color: VIOLET, bg: tint(VIOLET) }
  if (text.includes('usuario') || text.includes('cuenta') || text.includes('empresa')) return { icon: UserPlus, color: BLUE, bg: tint(BLUE) }
  if (text.includes('sesión') || text.includes('sesion') || text.includes('ingres')) return { icon: LogIn, color: GRAY, bg: tint(GRAY) }
  return { icon: Activity, color: GRAY, bg: tint(GRAY) }
}

const AVATAR_COLORS = ['#8b5cf6', '#3b82f6', '#06b6d4', '#10b981', '#f59e0b', '#ef4444', '#ec4899', '#6366f1']

function avatarColor(name = ''): string {
  let h = 0
  for (let i = 0; i < name.length; i++) h = (h * 31 + name.charCodeAt(i)) >>> 0
  return AVATAR_COLORS[h % AVATAR_COLORS.length]
}

function initials(name = ''): string {
  const parts = name.trim().split(/\s+/).filter(Boolean)
  if (parts.length === 0) return '?'
  return (parts[0][0] + (parts[1]?.[0] || '')).toUpperCase()
}

function when(a: ActivityItem): Date | null {
  const raw = a.created_at || a.timestamp
  if (!raw) return null
  const d = new Date(raw)
  return Number.isNaN(d.getTime()) ? null : d
}

function relativeTime(d: Date): string {
  const diff = (Date.now() - d.getTime()) / 1000
  if (diff < 60) return 'hace un momento'
  if (diff < 3600) return `hace ${Math.floor(diff / 60)} min`
  if (diff < 86400) return `hace ${Math.floor(diff / 3600)} h`
  if (diff < 172800) return 'ayer'
  return d.toLocaleDateString('es-ES', { day: '2-digit', month: 'short' })
}

function dayLabel(d: Date): string {
  const today = new Date(); today.setHours(0, 0, 0, 0)
  const day = new Date(d); day.setHours(0, 0, 0, 0)
  const diffDays = Math.round((today.getTime() - day.getTime()) / 86400000)
  if (diffDays <= 0) return 'Hoy'
  if (diffDays === 1) return 'Ayer'
  return d.toLocaleDateString('es-ES', { weekday: 'long', day: '2-digit', month: 'long' })
}

// Capitalised description after a bold name reads better lowercased.
function phrase(a: ActivityItem): string {
  const t = a.description || a.details || 'registró actividad'
  return t.charAt(0).toLowerCase() + t.slice(1)
}

export function ActivityFeed({ items }: { items: ActivityItem[] }) {
  // Group consecutive items by day, preserving the incoming (desc) order.
  const groups: { label: string; items: ActivityItem[] }[] = []
  for (const a of items) {
    const d = when(a)
    const label = d ? dayLabel(d) : 'Sin fecha'
    const last = groups[groups.length - 1]
    if (last && last.label === label) last.items.push(a)
    else groups.push({ label, items: [a] })
  }

  return (
    <div className={styles.feed}>
      {groups.map((group) => (
        <div key={group.label} className={styles.group}>
          <div className={styles.dayHeader}>{group.label}</div>
          <div className={styles.rows}>
            {group.items.map((a, i) => {
              const v = visualFor(a)
              const d = when(a)
              const Icon = v.icon
              return (
                <div key={a.id ?? `${group.label}-${i}`} className={styles.row}>
                  <div className={styles.avatarWrap}>
                    <span className={styles.avatar} style={{ background: avatarColor(a.user) }}>
                      {initials(a.user)}
                    </span>
                    <span className={styles.badge} style={{ background: v.color }} aria-hidden>
                      <Icon size={11} />
                    </span>
                  </div>

                  <div className={styles.body}>
                    <p className={styles.text}>
                      <strong>{a.user || 'Sistema'}</strong> {phrase(a)}
                    </p>
                    <div className={styles.sub}>
                      {a.company && a.company !== '-' && (
                        <span className={styles.chip} style={{ color: v.color, background: v.bg }}>{a.company}</span>
                      )}
                      {d && <span className={styles.time} title={d.toLocaleString('es-ES')}>{relativeTime(d)}</span>}
                    </div>
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      ))}
    </div>
  )
}
