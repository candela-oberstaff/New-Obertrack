import type { CSSProperties } from 'react'
import './Skeleton.css'

export interface SkeletonProps {
  width?: number | string
  height?: number | string
  /** Border radius in px (defaults to 8). Use a large value + square size for circles. */
  radius?: number | string
  circle?: boolean
  className?: string
  style?: CSSProperties
}

/** A single shimmering placeholder block. */
export function Skeleton({ width, height = 14, radius = 8, circle, className = '', style }: SkeletonProps) {
  return (
    <span
      className={`ui-skeleton ${className}`}
      style={{
        width,
        height,
        borderRadius: circle ? '50%' : radius,
        ...style,
      }}
      aria-hidden
    />
  )
}

export interface SkeletonTextProps {
  lines?: number
  className?: string
}

/** A stack of text-line skeletons (last line is shorter). */
export function SkeletonText({ lines = 3, className = '' }: SkeletonTextProps) {
  return (
    <span className={`ui-skeleton-text ${className}`}>
      {Array.from({ length: lines }).map((_, i) => (
        <Skeleton key={i} height={12} width={i === lines - 1 ? '60%' : '100%'} />
      ))}
    </span>
  )
}
