export type VideoProvider = 'drive' | 'youtube'

const DRIVE_FILE_ID_REGEX = /\/file\/d\/([a-zA-Z0-9_-]+)/
const YOUTUBE_ID_REGEX = /(?:youtube\.com\/(?:watch\?(?:[^&]*&)*v=|embed\/|v\/|shorts\/)|youtu\.be\/)([a-zA-Z0-9_-]{11})/

export interface VideoUrlInfo {
  provider: VideoProvider
  videoId: string
  embedUrl: string
}

export function parseVideoUrl(rawUrl: string): VideoUrlInfo | null {
  const url = rawUrl.trim()
  if (!url) return null

  if (url.includes('drive.google.com')) {
    const match = url.match(DRIVE_FILE_ID_REGEX)
    if (!match) return null
    return {
      provider: 'drive',
      videoId: match[1],
      embedUrl: `https://drive.google.com/file/d/${match[1]}/preview`,
    }
  }

  if (url.includes('youtube.com') || url.includes('youtu.be')) {
    const match = url.match(YOUTUBE_ID_REGEX)
    if (!match) return null
    return {
      provider: 'youtube',
      videoId: match[1],
      embedUrl: `https://www.youtube.com/embed/${match[1]}`,
    }
  }

  return null
}

/** Miniatura del video: YouTube y Drive la publican por id. */
export function videoThumbnailUrl(url: string): string | null {
  const info = parseVideoUrl(url)
  if (!info) return null
  return info.provider === 'youtube'
    ? `https://img.youtube.com/vi/${info.videoId}/mqdefault.jpg`
    : `https://drive.google.com/thumbnail?id=${info.videoId}&sz=w320`
}

export function buildEmbedUrl(url: string): string | null {
  return parseVideoUrl(url)?.embedUrl ?? null
}

export function getProviderLabel(provider: VideoProvider): string {
  return provider === 'drive' ? 'Google Drive' : 'YouTube'
}

/**
 * Un momento en palabras para las chapas de programación: «8 oct, 2:30 p. m.».
 * Con a. m./p. m. y no en 24 horas: «02:30» se leía como las dos y media de
 * la tarde y la novedad salía de madrugada.
 */
export function formatWhen(value: string): string {
  const date = new Date(value)
  const day = date.toLocaleDateString('es-ES', { day: 'numeric', month: 'short' }).replace('.', '')
  return `${day}, ${formatClock(date.getHours(), date.getMinutes())}`
}

/** Hora con a. m./p. m.: 14:30 → «2:30 p. m.». */
export function formatClock(hours: number, minutes: number): string {
  const suffix = hours < 12 ? 'a. m.' : 'p. m.'
  const h = hours % 12 === 0 ? 12 : hours % 12
  return `${h}:${String(minutes).padStart(2, '0')} ${suffix}`
}
