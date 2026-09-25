import { badgeColor, badgeIcon } from './badgeCatalog'
import styles from './Badges.module.css'

interface Props {
  icon: string
  color: string
  size?: 'sm' | 'md' | 'lg'
  /** Por ganar: se pinta en gris. */
  locked?: boolean
  /** Brillo animado (recién ganada). */
  shine?: boolean
  title?: string
}

const ICON_SIZE = { sm: 16, md: 24, lg: 36 }

/** Medallón de una insignia: círculo con degradado y el icono en blanco. */
export function BadgeMedallion({ icon, color, size = 'md', locked = false, shine = false, title }: Props) {
  const Icon = badgeIcon(icon)
  const c = badgeColor(color)
  return (
    <span
      className={`${styles.medallion} ${styles[size]} ${locked ? styles.locked : ''} ${shine ? styles.shine : ''}`}
      style={locked ? undefined : { background: `linear-gradient(145deg, ${c.from}, ${c.to})` }}
      title={title}
      aria-label={title}
      role="img"
    >
      <Icon size={ICON_SIZE[size]} strokeWidth={2.2} />
    </span>
  )
}
