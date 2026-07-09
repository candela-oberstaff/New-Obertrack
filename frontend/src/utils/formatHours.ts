export function formatHours(dec: number | null | undefined): string {
  const total = dec || 0
  const h = Math.floor(total)
  const m = Math.round((total - h) * 60)
  if (h === 0 && m === 0) return '0h'
  if (m === 0) return `${h}h`
  if (h === 0) return `${m}m`
  return `${h}h ${m}m`
}
