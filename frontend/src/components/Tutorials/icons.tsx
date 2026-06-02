import {
  PlayCircle,
  Clock,
  CheckSquare,
  Inbox,
  MessageCircle,
  FileText,
  Settings,
  Wrench,
  Activity,
  User,
  Bell,
  Calendar,
  BarChart3,
  BookOpen,
  Video,
  HelpCircle,
  Users,
  Mail,
  Building2,
  Briefcase,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

export const TUTORIAL_ICONS: Record<string, LucideIcon> = {
  PlayCircle,
  Clock,
  CheckSquare,
  Inbox,
  MessageCircle,
  FileText,
  Settings,
  Wrench,
  Activity,
  User,
  Bell,
  Calendar,
  BarChart3,
  BookOpen,
  Video,
  HelpCircle,
  Users,
  Mail,
  Building2,
  Briefcase,
}

export const TUTORIAL_ICON_NAMES = Object.keys(TUTORIAL_ICONS)

interface TutorialIconProps {
  name: string
  size?: number
  className?: string
}

export function TutorialIcon({ name, size = 22, className }: TutorialIconProps) {
  const Icon = TUTORIAL_ICONS[name] || PlayCircle
  return <Icon size={size} className={className} />
}
