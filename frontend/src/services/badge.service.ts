import api from './client'

export type BadgeKind = 'block' | 'program' | 'merit'

/** Insignia ganada. Es permanente: no depende de la invitación de inducción. */
export interface UserBadge {
  id: number
  user_id: number
  kind: BadgeKind
  source_key: string
  title: string
  description: string
  icon: string
  color: string
  score: number
  program_name: string
  earned_at: string
}

/** Insignia que todavía se puede ganar en la inducción en curso. */
export interface PendingBadge {
  kind: BadgeKind
  title: string
  icon: string
  color: string
}

export interface BadgeOverview {
  earned: UserBadge[]
  pending: PendingBadge[]
}

export const badgeService = {
  /** Mis insignias (ganadas y por ganar). */
  mine: async () => {
    const { data } = await api.get<BadgeOverview>('/me/badges')
    return data
  },

  /** Insignias de otra persona: misma visibilidad que el detalle del usuario. */
  forUser: async (userId: number) => {
    const { data } = await api.get<BadgeOverview>(`/users/${userId}/badges`)
    return data
  },
}
