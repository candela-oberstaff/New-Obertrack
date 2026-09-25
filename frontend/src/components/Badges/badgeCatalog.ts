import {
  Award,
  Medal,
  Trophy,
  Star,
  Shield,
  ShieldCheck,
  GraduationCap,
  BookOpen,
  Lightbulb,
  Rocket,
  Target,
  Flag,
  Heart,
  Zap,
  Crown,
  Sparkles,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

/**
 * Set cerrado de iconos y colores de las insignias. Es el mismo que valida el
 * backend: lo que no esté aquí cae al icono o color de respaldo.
 */
export const BADGE_ICONS: Record<string, LucideIcon> = {
  Award,
  Medal,
  Trophy,
  Star,
  Shield,
  ShieldCheck,
  GraduationCap,
  BookOpen,
  Lightbulb,
  Rocket,
  Target,
  Flag,
  Heart,
  Zap,
  Crown,
  Sparkles,
}

export const BADGE_ICON_NAMES = Object.keys(BADGE_ICONS)

export interface BadgeColorDef {
  key: string
  label: string
  /** Degradado del medallón. */
  from: string
  to: string
}

export const BADGE_COLORS: BadgeColorDef[] = [
  { key: 'orchid', label: 'Orquídea', from: '#cc33cc', to: '#7a1f7a' },
  { key: 'indigo', label: 'Índigo', from: '#6366f1', to: '#3730a3' },
  { key: 'emerald', label: 'Esmeralda', from: '#10b981', to: '#047857' },
  { key: 'amber', label: 'Ámbar', from: '#f59e0b', to: '#b45309' },
  { key: 'rose', label: 'Rosa', from: '#f43f5e', to: '#9f1239' },
  { key: 'sky', label: 'Cielo', from: '#0ea5e9', to: '#0369a1' },
  { key: 'slate', label: 'Pizarra', from: '#64748b', to: '#334155' },
  { key: 'gold', label: 'Oro', from: '#eab308', to: '#a16207' },
]

export function badgeIcon(name: string | undefined): LucideIcon {
  return (name && BADGE_ICONS[name]) || Award
}

export function badgeColor(key: string | undefined): BadgeColorDef {
  return BADGE_COLORS.find((c) => c.key === key) ?? BADGE_COLORS[0]
}

export const DEFAULT_BLOCK_BADGE = { icon: 'Award', color: 'orchid' }
export const DEFAULT_PROGRAM_BADGE = { icon: 'Trophy', color: 'gold' }
