import { useState, useEffect } from 'react'
import styles from '../../pages/SlackChat.module.css'

const GIPHY_KEY = import.meta.env.VITE_GIPHY_API_KEY

const EMOJI_CATEGORIES: { label: string; icon: string; emojis: string[] }[] = [
  {
    label: 'Caritas',
    icon: '😀',
    emojis: ['😀', '😃', '😄', '😁', '😆', '😅', '😂', '🤣', '😊', '😇', '🙂', '😉', '😍', '🥰', '😘', '😋', '😜', '🤪', '🤗', '🤔', '🤨', '😐', '🙄', '😏', '😴', '🤤', '😷', '🤒', '🥳', '😎', '🤓', '🧐', '😕', '😮', '😲', '😳', '🥺', '😢', '😭', '😱', '😖', '😞', '😓', '😩', '🥱', '😤', '😡', '🤯'],
  },
  {
    label: 'Gestos',
    icon: '👍',
    emojis: ['👍', '👎', '👌', '🤌', '✌️', '🤞', '🤟', '🤘', '🤙', '👈', '👉', '👆', '👇', '☝️', '✋', '🤚', '🖐️', '🖖', '👋', '🤝', '👏', '🙌', '👐', '🤲', '🙏', '💪', '✍️', '🤳'],
  },
  {
    label: 'Corazones',
    icon: '❤️',
    emojis: ['❤️', '🧡', '💛', '💚', '💙', '💜', '🖤', '🤍', '🤎', '💔', '❣️', '💕', '💞', '💓', '💗', '💖', '💘', '💝', '💟'],
  },
  {
    label: 'Animales',
    icon: '🐶',
    emojis: ['🐶', '🐱', '🐭', '🐹', '🐰', '🦊', '🐻', '🐼', '🐨', '🐯', '🦁', '🐮', '🐷', '🐸', '🐵', '🐔', '🐧', '🐦', '🦄', '🐝', '🦋', '🐢', '🐙', '🦀', '🐬', '🐳'],
  },
  {
    label: 'Comida',
    icon: '🍕',
    emojis: ['🍏', '🍎', '🍌', '🍉', '🍇', '🍓', '🍒', '🍑', '🥭', '🍍', '🥑', '🌮', '🍕', '🍔', '🍟', '🌭', '🍿', '🥗', '🍣', '🍜', '🍦', '🍩', '🍪', '🎂', '🍫', '☕', '🍺', '🍷'],
  },
  {
    label: 'Actividades',
    icon: '⚽',
    emojis: ['⚽', '🏀', '🏈', '⚾', '🎾', '🏐', '🎱', '🏓', '🏸', '🥊', '🎮', '🎲', '🎯', '🎳', '🎤', '🎧', '🎸', '🥁', '🎹', '🎨', '🎬', '🏆', '🥇', '🎟️'],
  },
  {
    label: 'Objetos',
    icon: '💡',
    emojis: ['💡', '🔥', '⭐', '🌟', '✨', '⚡', '💥', '💯', '✅', '❌', '⚠️', '❗', '❓', '💬', '📌', '📎', '📁', '📅', '⏰', '💰', '💎', '🎁', '🎉', '🎊', '🚀', '✈️', '🏠', '📱', '💻', '🖥️'],
  },
]

interface GiphyGif {
  id: string
  title: string
  images: {
    fixed_height_small: { url: string }
    original: { url: string }
  }
}

interface EmojiGifPickerProps {
  tab: 'emoji' | 'gif'
  setTab: (tab: 'emoji' | 'gif') => void
  onSelectEmoji: (emoji: string) => void
  onSelectGif: (url: string, title?: string) => void
}

export function EmojiGifPicker({ tab, setTab, onSelectEmoji, onSelectGif }: EmojiGifPickerProps) {
  const [category, setCategory] = useState(0)
  const [gifQuery, setGifQuery] = useState('')
  const [gifs, setGifs] = useState<GiphyGif[]>([])
  const [loadingGifs, setLoadingGifs] = useState(false)

  // Trending by default, search after a short debounce while typing.
  useEffect(() => {
    if (tab !== 'gif' || !GIPHY_KEY) return
    let active = true
    const timeout = setTimeout(async () => {
      setLoadingGifs(true)
      try {
        const endpoint = gifQuery.trim()
          ? `https://api.giphy.com/v1/gifs/search?api_key=${GIPHY_KEY}&q=${encodeURIComponent(gifQuery.trim())}&limit=24&rating=pg-13&lang=es`
          : `https://api.giphy.com/v1/gifs/trending?api_key=${GIPHY_KEY}&limit=24&rating=pg-13`
        const res = await fetch(endpoint)
        const json = await res.json()
        if (active) setGifs(json.data || [])
      } catch (e) {
        console.error('Error fetching GIFs:', e)
        if (active) setGifs([])
      } finally {
        if (active) setLoadingGifs(false)
      }
    }, 400)
    return () => { active = false; clearTimeout(timeout) }
  }, [tab, gifQuery])

  return (
    <div className={styles['composer-picker']}>
      <div className={styles['composer-picker-tabs']}>
        <button
          className={tab === 'emoji' ? styles['active'] : ''}
          onClick={() => setTab('emoji')}
        >
          😊 Emojis
        </button>
        <button
          className={tab === 'gif' ? styles['active'] : ''}
          onClick={() => setTab('gif')}
        >
          GIF
        </button>
      </div>

      {tab === 'emoji' ? (
        <>
          <div className={styles['emoji-category-bar']}>
            {EMOJI_CATEGORIES.map((cat, i) => (
              <button
                key={cat.label}
                className={i === category ? styles['active'] : ''}
                onClick={() => setCategory(i)}
                title={cat.label}
              >
                {cat.icon}
              </button>
            ))}
          </div>
          <div className={styles['emoji-grid']}>
            {EMOJI_CATEGORIES[category].emojis.map(emoji => (
              <button key={emoji} onClick={() => onSelectEmoji(emoji)}>
                {emoji}
              </button>
            ))}
          </div>
        </>
      ) : (
        <>
          {GIPHY_KEY ? (
            <>
              <div className={styles['gif-search']}>
                <input
                  type="text"
                  value={gifQuery}
                  onChange={(e) => setGifQuery(e.target.value)}
                  placeholder="Buscar GIFs..."
                  autoFocus
                />
              </div>
              <div className={styles['gif-grid']}>
                {loadingGifs ? (
                  <p className={styles['gif-hint']}>Buscando...</p>
                ) : gifs.length > 0 ? (
                  gifs.map(gif => (
                    <img
                      key={gif.id}
                      src={gif.images.fixed_height_small.url}
                      alt={gif.title}
                      loading="lazy"
                      onClick={() => onSelectGif(gif.images.original.url, gif.title || 'GIF')}
                    />
                  ))
                ) : (
                  <p className={styles['gif-hint']}>No se encontraron GIFs</p>
                )}
              </div>
              <div className={styles['gif-attribution']}>Powered by GIPHY</div>
            </>
          ) : (
            <p className={styles['gif-hint']}>
              Para habilitar los GIFs, define <code>VITE_GIPHY_API_KEY</code> en el archivo <code>.env</code> del frontend
              (la llave es gratuita en developers.giphy.com).
            </p>
          )}
        </>
      )}
    </div>
  )
}
