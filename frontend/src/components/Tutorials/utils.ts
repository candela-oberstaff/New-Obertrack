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

export function buildEmbedUrl(url: string): string | null {
  return parseVideoUrl(url)?.embedUrl ?? null
}

export function getProviderLabel(provider: VideoProvider): string {
  return provider === 'drive' ? 'Google Drive' : 'YouTube'
}
