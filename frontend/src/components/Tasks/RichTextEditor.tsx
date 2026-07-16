import { useRef, useEffect, useCallback, useState } from 'react'
import { Bold, Italic, Underline, List, ListOrdered, Link as LinkIcon, Heading2, Type } from 'lucide-react'
import { sanitizeRichHtml } from '../../utils/sanitize'
import { uploadService } from '../../services/api'
import styles from './RichTextEditor.module.css'

interface RichTextEditorProps {
  value: string
  onChange: (value: string) => void
  placeholder?: string
}

export function RichTextEditor({ value, onChange, placeholder }: RichTextEditorProps) {
  const editorRef = useRef<HTMLDivElement>(null)
  const isInternalChange = useRef(false)
  const [isUploading, setIsUploading] = useState(false)

  // Sync value from prop to innerHTML
  useEffect(() => {
    if (editorRef.current && !isInternalChange.current) {
      const safe = sanitizeRichHtml(value)
      if (editorRef.current.innerHTML !== safe) {
        editorRef.current.innerHTML = safe
      }
    }
    isInternalChange.current = false
  }, [value])

  const handleInput = useCallback(() => {
    if (editorRef.current) {
      const html = editorRef.current.innerHTML
      isInternalChange.current = true
      onChange(html === '<br>' ? '' : html)
    }
  }, [onChange])

  // Al pegar una imagen (p. ej. una captura), la subimos al servidor e
  // insertamos su URL en vez de dejar el base64 gigante en la descripción.
  const handlePaste = useCallback(async (e: React.ClipboardEvent) => {
    const items = e.clipboardData?.items
    if (!items) return
    const images: File[] = []
    for (let i = 0; i < items.length; i++) {
      const it = items[i]
      if (it.kind === 'file' && it.type.startsWith('image/')) {
        const f = it.getAsFile()
        if (f) images.push(f)
      }
    }
    if (images.length === 0) return // pegado normal (texto/HTML)

    e.preventDefault()
    // Guarda la posición del cursor para insertar la imagen donde estaba.
    const sel = window.getSelection()
    const range = sel && sel.rangeCount ? sel.getRangeAt(0) : null

    setIsUploading(true)
    try {
      for (const file of images) {
        const res = await uploadService.upload(file)
        editorRef.current?.focus()
        if (range) {
          sel!.removeAllRanges()
          sel!.addRange(range)
        }
        document.execCommand('insertImage', false, res.url)
      }
      handleInput()
    } catch (err) {
      console.error('Error subiendo la imagen pegada:', err)
    } finally {
      setIsUploading(false)
    }
  }, [handleInput])

  const execCommand = (command: string, arg?: string) => {
    document.execCommand(command, false, arg)
    handleInput()
    editorRef.current?.focus()
  }

  // Handle keyboard shortcuts
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.ctrlKey || e.metaKey) {
      if (e.key === 'b') { e.preventDefault(); execCommand('bold'); }
      if (e.key === 'i') { e.preventDefault(); execCommand('italic'); }
      if (e.key === 'u') { e.preventDefault(); execCommand('underline'); }
    }
  }

  return (
    <div className={styles.richTextEditor}>
      <div className={styles.richTextToolbar}>
        <button type="button" onMouseDown={(e) => { e.preventDefault(); execCommand('bold'); }} title="Negrita">
          <Bold size={16} />
        </button>
        <button type="button" onMouseDown={(e) => { e.preventDefault(); execCommand('italic'); }} title="Cursiva">
          <Italic size={16} />
        </button>
        <button type="button" onMouseDown={(e) => { e.preventDefault(); execCommand('underline'); }} title="Subrayado">
          <Underline size={16} />
        </button>
        <span className={styles.toolbarSeparator}>|</span>
        <button type="button" onMouseDown={(e) => { e.preventDefault(); execCommand('insertUnorderedList'); }} title="Viñetas">
          <List size={16} />
        </button>
        <button type="button" onMouseDown={(e) => { e.preventDefault(); execCommand('insertOrderedList'); }} title="Numeración">
          <ListOrdered size={16} />
        </button>
        <span className={styles.toolbarSeparator}>|</span>
        <button type="button" onMouseDown={(e) => {
          e.preventDefault()
          const url = prompt('Ingresa la URL:')
          if (url) execCommand('createLink', url)
        }} title="Enlace">
          <LinkIcon size={16} />
        </button>
        <button type="button" onMouseDown={(e) => { e.preventDefault(); execCommand('formatBlock', 'h2'); }} title="Título">
          <Heading2 size={16} />
        </button>
        <button type="button" onMouseDown={(e) => { e.preventDefault(); execCommand('formatBlock', 'p'); }} title="Párrafo">
          <Type size={16} />
        </button>
      </div>
      <div
        ref={editorRef}
        className={styles.richTextContent}
        contentEditable
        onInput={handleInput}
        onKeyDown={handleKeyDown}
        onPaste={handlePaste}
        data-placeholder={placeholder || 'Escribe aquí...'}
      />
      {isUploading && (
        <div className={styles.richTextUploading}>Subiendo imagen…</div>
      )}
    </div>
  )
}
